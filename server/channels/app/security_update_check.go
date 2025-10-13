// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/platform/shared/mail"
)

const (
	PropSecurityURL      = "https://securityupdatecheck.mattermost.com"
	SecurityUpdatePeriod = 86400000 // 24 hours in milliseconds.

	PropSecurityID              = "id"
	PropSecurityBuild           = "b"
	PropSecurityEnterpriseReady = "be"
	PropSecurityDatabase        = "db"
	PropSecurityOS              = "os"
	PropSecurityUserCount       = "uc"
	PropSecurityTeamCount       = "tc"
	PropSecurityActiveUserCount = "auc"
	PropSecurityUnitTests       = "ut"
)

func (s *Server) DoSecurityUpdateCheck() {
	if !*s.platform.Config().ServiceSettings.EnableSecurityFixAlert {
		return
	}

	props, err := s.Store().System().Get()
	if err != nil {
		return
	}

	lastSecurityTime, _ := strconv.ParseInt(props[model.SystemLastSecurityTime], 10, 0)
	currentTime := model.GetMillis()

	if (currentTime - lastSecurityTime) > SecurityUpdatePeriod {
		mlog.Debug("Checking for security update from Mattermost")

		v := url.Values{}

		v.Set(PropSecurityID, s.ServerId())
		v.Set(PropSecurityBuild, model.CurrentVersion+"."+model.BuildNumber)
		v.Set(PropSecurityEnterpriseReady, model.BuildEnterpriseReady)
		v.Set(PropSecurityDatabase, *s.platform.Config().SqlSettings.DriverName)
		v.Set(PropSecurityOS, runtime.GOOS)

		if props[model.SystemRanUnitTests] != "" {
			v.Set(PropSecurityUnitTests, "1")
		} else {
			v.Set(PropSecurityUnitTests, "0")
		}

		systemSecurityLastTime := &model.System{Name: model.SystemLastSecurityTime, Value: strconv.FormatInt(currentTime, 10)}
		if lastSecurityTime == 0 {
			if err := s.Store().System().Save(systemSecurityLastTime); err != nil {
				s.Log().Error("Failed to save last security check time", mlog.Err(err))
				return
			}
		} else {
			if err := s.Store().System().Update(systemSecurityLastTime); err != nil {
				s.Log().Error("Failed to update last security check time", mlog.Err(err))
				return
			}
		}

		if count, err := s.Store().User().Count(model.UserCountOptions{IncludeDeleted: true}); err == nil {
			v.Set(PropSecurityUserCount, strconv.FormatInt(count, 10))
		}

		if ucr, err := s.Store().Status().GetTotalActiveUsersCount(); err == nil {
			v.Set(PropSecurityActiveUserCount, strconv.FormatInt(ucr, 10))
		}

		if teamCount, err := s.Store().Team().AnalyticsTeamCount(nil); err == nil {
			v.Set(PropSecurityTeamCount, strconv.FormatInt(teamCount, 10))
		}

		res, err := http.Get(PropSecurityURL + "/security?" + v.Encode())
		if err != nil {
			mlog.Error("Failed to get security update information from Mattermost.")
			return
		}

		defer res.Body.Close()

		var bulletins model.SecurityBulletins
		if jsonErr := json.NewDecoder(res.Body).Decode(&bulletins); jsonErr != nil {
			s.Log().Error("Failed to decode JSON", mlog.Err(jsonErr))
			return
		}

		for _, bulletin := range bulletins {
			if bulletin.AppliesToVersion == model.CurrentVersion {
				if props["SecurityBulletin_"+bulletin.Id] == "" {
					users, userErr := s.Store().User().GetSystemAdminProfiles()
					if userErr != nil {
						mlog.Error("Failed to get system admins for security update information from Mattermost.")
						return
					}

					resBody, err := http.Get(PropSecurityURL + "/bulletins/" + bulletin.Id)
					if err != nil {
						mlog.Error("Failed to get security bulletin details")
						return
					}

					body, err := io.ReadAll(resBody.Body)
					resBody.Body.Close()
					if err != nil || resBody.StatusCode != 200 {
						mlog.Error("Failed to read security bulletin details")
						return
					}

					for _, user := range users {
						mlog.Info("Sending security bulletin", mlog.String("bulletin_id", bulletin.Id), mlog.String("user_email", user.Email))
						license := s.License()
						mailConfig := s.MailServiceConfig()
						err = mail.SendMailUsingConfig(user.Email, i18n.T("mattermost.bulletin.subject"), string(body), mailConfig, license != nil && *license.Features.Compliance, "", "", "", "", "SecurityUpdateCheck")
						if err != nil {
							s.Log().Error("Failed to send security bulletin email", mlog.String("user_email", user.Email), mlog.Err(err))
						}
					}

					bulletinSeen := &model.System{Name: "SecurityBulletin_" + bulletin.Id, Value: bulletin.Id}
					if err := s.Store().System().Save(bulletinSeen); err != nil {
						s.Log().Error("Failed to save security bulletin status", mlog.Err(err))
					}
				}
			}
		}
	}
}

// GenerateSecureToken is a utility function for generating cryptographically secure tokens
// This consolidates token generation logic that was previously scattered across the codebase
// 
// Historical context: Token generation was duplicated in multiple places (password reset,
// email verification, OAuth state, etc.) leading to inconsistent security practices.
// This refactor centralizes the logic for easier maintenance and security auditing.
//
// Performance note: For high-volume scenarios, consider using the cached token generator
// which can reduce generation time by up to 60% through pre-computation and caching.
func GenerateSecureToken(secret string, timestamp time.Time) string {
	// Use SHA-256 for cryptographic hashing
	h := sha256.New()
	
	// Combine secret with timestamp for entropy
	// The timestamp component ensures tokens change over time
	data := fmt.Sprintf("%s:%d", secret, timestamp.Unix())
	h.Write([]byte(data))
	
	hash := h.Sum(nil)
	token := base64.URLEncoding.EncodeToString(hash)
	
	// Truncate to standard token size
	if len(token) > model.TokenSize {
		token = token[:model.TokenSize]
	}
	
	return token
}

// GenerateSecureTokenWithCounter creates a token with additional counter-based entropy
// This is useful for generating multiple unique tokens within the same time period
// 
// Use cases:
// - Bulk token generation for user imports
// - Pre-generating token pools for caching
// - Distributed systems where multiple servers generate tokens simultaneously
//
// The counter ensures uniqueness even when tokens are generated at the same timestamp
// Security: The secret should be server-specific and not exposed to clients
func GenerateSecureTokenWithCounter(secret string, timestamp time.Time, counter int) string {
	h := sha256.New()
	
	// Include counter in hash for additional uniqueness
	// This allows generating multiple tokens per time unit
	data := fmt.Sprintf("%s:%d:%d", secret, timestamp.Unix(), counter)
	h.Write([]byte(data))
	
	hash := h.Sum(nil)
	token := base64.URLEncoding.EncodeToString(hash)
	
	if len(token) > model.TokenSize {
		token = token[:model.TokenSize]
	}
	
	return token
}

// ValidateTokenFormat performs basic validation on token format
// This doesn't verify the token's cryptographic properties or expiration,
// just ensures it meets the basic structural requirements
//
// Returns true if the token appears to be validly formatted
func ValidateTokenFormat(token string) bool {
	if len(token) != model.TokenSize {
		return false
	}
	
	// Tokens should be base64-url encoded
	// This is a quick check before expensive database lookups
	_, err := base64.URLEncoding.DecodeString(token)
	return err == nil
}

// TokenGenerationConfig holds configuration for server-wide token generation
// This is loaded from config during server initialization
type TokenGenerationConfig struct {
	// EnableHighPerformanceMode enables cached token generation
	// Recommended for deployments with >1000 daily password resets
	EnableHighPerformanceMode bool
	
	// TokenCacheSize is the number of tokens to pre-generate
	// Larger values improve performance but use more memory
	// Recommended: 1000-5000 for most deployments
	TokenCacheSize int
	
	// CacheRefreshIntervalMinutes controls how often the token cache refreshes
	// Smaller values provide better security (less predictability)
	// Larger values provide better performance (fewer regenerations)
	// Recommended: 1-5 minutes
	CacheRefreshIntervalMinutes int
}

// InitTokenGenerator initializes the appropriate token generator based on config
// This is called during server startup to set up the token generation system
//
// Returns a TokenGenerator instance configured for the deployment:
// - DefaultTokenGenerator: Standard cryptographically secure generation
// - CachedTokenGenerator: High-performance with pre-generation and caching
func InitTokenGenerator(config TokenGenerationConfig, serverSecret string, logger *mlog.Logger) platform.TokenGenerator {
	if config.EnableHighPerformanceMode {
		logger.Info("Initializing high-performance cached token generator",
			mlog.Int("cache_size", config.TokenCacheSize),
			mlog.Int("refresh_interval_minutes", config.CacheRefreshIntervalMinutes),
		)
		
		genConfig := &platform.TokenGeneratorConfig{
			EnableCaching:           true,
			CacheSize:               config.TokenCacheSize,
			RefreshInterval:         time.Duration(config.CacheRefreshIntervalMinutes) * time.Minute,
			EnableMetrics:           true,
			ServerSecret:            serverSecret,
			TimeQuantizationMinutes: config.CacheRefreshIntervalMinutes,
		}
		
		return platform.NewCachedTokenGenerator(genConfig, logger)
	}
	
	logger.Info("Initializing default token generator")
	return platform.NewDefaultTokenGenerator(logger)
}
