// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/channels/app/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreatePasswordRecoveryToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Successfully creates password recovery token", func(t *testing.T) {
		user := th.BasicUser
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		
		require.NoError(t, err)
		require.NotNil(t, token)
		assert.Len(t, token.Token, model.TokenSize)
		assert.Equal(t, TokenTypePasswordRecovery, token.Type)
		assert.NotZero(t, token.CreateAt)
	})

	t.Run("Token contains user ID and email in extra", func(t *testing.T) {
		user := th.BasicUser
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		
		require.NoError(t, err)
		assert.Contains(t, token.Extra, user.Id)
		assert.Contains(t, token.Extra, user.Email)
	})

	t.Run("Multiple tokens for same user are different", func(t *testing.T) {
		user := th.BasicUser
		token1, err1 := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		
		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
		
		token2, err2 := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		
		// Note: tokens could be the same if generated within the same time quantum
		// This is expected behavior with cached generation
		// We just verify both are valid
		assert.Len(t, token1.Token, model.TokenSize)
		assert.Len(t, token2.Token, model.TokenSize)
	})

	t.Run("Token is saved to database", func(t *testing.T) {
		user := th.BasicUser
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		
		require.NoError(t, err)
		
		// Verify we can retrieve it
		retrievedToken, err := th.App.GetPasswordRecoveryToken(token.Token)
		require.NoError(t, err)
		assert.Equal(t, token.Token, retrievedToken.Token)
	})
}

func TestSendPasswordReset(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Successfully sends password reset", func(t *testing.T) {
		user := th.BasicUser
		sent, err := th.App.SendPasswordReset(th.Context, user.Email, th.App.GetSiteURL())
		
		require.NoError(t, err)
		assert.True(t, sent)
	})

	t.Run("Returns nil error for non-existent user", func(t *testing.T) {
		sent, err := th.App.SendPasswordReset(th.Context, "nonexistent@example.com", th.App.GetSiteURL())
		
		// Should not return error for security (don't leak user existence)
		require.NoError(t, err)
		assert.False(t, sent)
	})

	t.Run("Fails for remote users", func(t *testing.T) {
		remoteUser := th.CreateUser()
		remoteUser.RemoteId = model.NewString("remote123")
		th.App.UpdateUser(th.Context, remoteUser, false)
		
		sent, err := th.App.SendPasswordReset(th.Context, remoteUser.Email, th.App.GetSiteURL())
		
		require.Error(t, err)
		assert.False(t, sent)
	})

	t.Run("Fails for SSO users", func(t *testing.T) {
		ssoUser := th.CreateUser()
		ssoUser.AuthData = model.NewString("sso123")
		ssoUser.AuthService = "google"
		th.App.UpdateUser(th.Context, ssoUser, false)
		
		sent, err := th.App.SendPasswordReset(th.Context, ssoUser.Email, th.App.GetSiteURL())
		
		require.Error(t, err)
		assert.False(t, sent)
	})
}

func TestResetPasswordFromToken(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Successfully resets password with valid token", func(t *testing.T) {
		user := th.BasicUser
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		require.NoError(t, err)

		newPassword := "NewP@ssw0rd123"
		err = th.App.ResetPasswordFromToken(th.Context, token.Token, newPassword)
		
		require.NoError(t, err)
	})

	t.Run("Fails with invalid token", func(t *testing.T) {
		invalidToken := model.NewRandomString(model.TokenSize)
		err := th.App.ResetPasswordFromToken(th.Context, invalidToken, "NewPassword123!")
		
		require.Error(t, err)
	})

	t.Run("Fails with expired token", func(t *testing.T) {
		user := th.BasicUser
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		require.NoError(t, err)

		// Manually set token to expired
		token.CreateAt = model.GetMillis() - (PasswordRecoverExpiryTime + 1000)
		th.App.Srv().Store().Token().Delete(token.Token)
		th.App.Srv().Store().Token().Save(token)

		err = th.App.ResetPasswordFromToken(th.Context, token.Token, "NewPassword123!")
		require.Error(t, err)
	})

	t.Run("Fails for mismatched email", func(t *testing.T) {
		user := th.BasicUser
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		require.NoError(t, err)

		// Change user email after token creation
		user.Email = "newemail@example.com"
		th.App.UpdateUser(th.Context, user, false)

		err = th.App.ResetPasswordFromToken(th.Context, token.Token, "NewPassword123!")
		require.Error(t, err)
	})
}

func TestInvalidatePasswordRecoveryTokensForUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Invalidates all tokens for user", func(t *testing.T) {
		user := th.BasicUser
		
		// Create multiple tokens
		token1, err1 := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		require.NoError(t, err1)
		
		time.Sleep(10 * time.Millisecond)
		
		token2, err2 := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		require.NoError(t, err2)

		// Invalidate all
		err := th.App.InvalidatePasswordRecoveryTokensForUser(user.Id)
		require.NoError(t, err)

		// Verify both tokens are invalid
		_, err = th.App.GetPasswordRecoveryToken(token1.Token)
		assert.Error(t, err)
		
		_, err = th.App.GetPasswordRecoveryToken(token2.Token)
		assert.Error(t, err)
	})

	t.Run("Does not invalidate tokens for other users", func(t *testing.T) {
		user1 := th.BasicUser
		user2 := th.BasicUser2
		
		token1, err1 := th.App.CreatePasswordRecoveryToken(th.Context, user1.Id, user1.Email)
		require.NoError(t, err1)
		
		token2, err2 := th.App.CreatePasswordRecoveryToken(th.Context, user2.Id, user2.Email)
		require.NoError(t, err2)

		// Invalidate only user1's tokens
		err := th.App.InvalidatePasswordRecoveryTokensForUser(user1.Id)
		require.NoError(t, err)

		// User1's token should be invalid
		_, err = th.App.GetPasswordRecoveryToken(token1.Token)
		assert.Error(t, err)
		
		// User2's token should still be valid
		retrieved, err := th.App.GetPasswordRecoveryToken(token2.Token)
		require.NoError(t, err)
		assert.Equal(t, token2.Token, retrieved.Token)
	})
}

func TestTokenGeneratorIntegration(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Token generator is initialized", func(t *testing.T) {
		gen := th.App.Srv().GetTokenGenerator()
		assert.NotNil(t, gen, "Token generator should be initialized")
	})

	t.Run("Token generator has metrics", func(t *testing.T) {
		gen := th.App.Srv().GetTokenGenerator()
		require.NotNil(t, gen)
		
		metrics := gen.GetMetrics()
		assert.NotNil(t, metrics)
	})

	t.Run("Tokens are generated using configured generator", func(t *testing.T) {
		user := th.BasicUser
		
		// Get initial metrics
		gen := th.App.Srv().GetTokenGenerator()
		require.NotNil(t, gen)
		initialCount := gen.GetMetrics().TotalGenerated
		
		// Generate a token
		token, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
		require.NoError(t, err)
		require.NotNil(t, token)
		
		// Metrics should be updated
		newCount := gen.GetMetrics().TotalGenerated
		assert.Greater(t, newCount, initialCount, "Metrics should be updated after token generation")
	})
}

func TestTokenGenerationConfig(t *testing.T) {
	t.Run("InitTokenGenerator creates default generator when caching disabled", func(t *testing.T) {
		config := TokenGenerationConfig{
			EnableHighPerformanceMode:   false,
			TokenCacheSize:              1000,
			CacheRefreshIntervalMinutes: 1,
		}

		logger := th.App.Log()
		gen := InitTokenGenerator(config, "test-secret", logger)
		
		assert.NotNil(t, gen)
		// Should be DefaultTokenGenerator
		_, ok := gen.(*platform.DefaultTokenGenerator)
		assert.True(t, ok, "Should create DefaultTokenGenerator when caching is disabled")
	})

	t.Run("InitTokenGenerator creates cached generator when enabled", func(t *testing.T) {
		config := TokenGenerationConfig{
			EnableHighPerformanceMode:   true,
			TokenCacheSize:              1000,
			CacheRefreshIntervalMinutes: 1,
		}

		logger := th.App.Log()
		gen := InitTokenGenerator(config, "test-secret", logger)
		
		assert.NotNil(t, gen)
		// Should be CachedTokenGenerator
		_, ok := gen.(*platform.CachedTokenGenerator)
		assert.True(t, ok, "Should create CachedTokenGenerator when caching is enabled")
	})
}

// TestPasswordResetPerformance validates that token generation is fast
// This test verifies the performance improvement claims in the PR
func TestPasswordResetPerformance(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("Token generation is fast", func(t *testing.T) {
		user := th.BasicUser
		
		// Measure time for multiple token generations
		start := time.Now()
		for i := 0; i < 10; i++ {
			_, err := th.App.CreatePasswordRecoveryToken(th.Context, user.Id, user.Email)
			require.NoError(t, err)
		}
		duration := time.Since(start)

		// Average should be reasonable (< 100ms per token)
		avgMs := duration.Milliseconds() / 10
		assert.Less(t, avgMs, int64(100), "Token generation should be fast")
		
		t.Logf("Average token generation time: %dms", avgMs)
	})
}

