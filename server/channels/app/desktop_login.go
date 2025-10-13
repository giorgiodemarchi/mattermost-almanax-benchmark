// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const (
	DesktopLoginDeviceCodeLength  = 8
	DesktopLoginDeviceCodeExpiry  = 10 * time.Minute
	DesktopLoginMaxPendingSessions = 10
	QRCodeLoginExpiry             = 5 * time.Minute
)

// DesktopLoginRequest represents a request to initiate desktop login
type DesktopLoginRequest struct {
	DeviceName    string `json:"device_name"`
	DevicePlatform string `json:"device_platform"`
	AppVersion    string `json:"app_version"`
	LoginMethod   string `json:"login_method"` // "device_code" or "qr_code"
}

// DesktopLoginResponse contains the session token and device code for desktop login
type DesktopLoginResponse struct {
	SessionToken string `json:"session_token"`
	DeviceCode   string `json:"device_code"`
	ExpiresAt    int64  `json:"expires_at"`
	VerifyURL    string `json:"verify_url"`
	QRCode       string `json:"qr_code,omitempty"`
}

// generateDeviceCode creates a random device code for user-friendly desktop login
func generateDeviceCode() (string, error) {
	// Generate 6 bytes (8 characters when base32 encoded)
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	
	// Use custom character set for readability (no 0/O, 1/I confusion)
	const charset = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	code := make([]byte, DesktopLoginDeviceCodeLength)
	for i := 0; i < DesktopLoginDeviceCodeLength; i++ {
		code[i] = charset[b[i%6]%byte(len(charset))]
	}
	
	return string(code), nil
}

// generateQRCodeData creates QR code data for mobile scanning
func generateQRCodeData(baseURL, deviceCode, sessionToken string) string {
	// QR code contains both device code and session token for seamless login
	return fmt.Sprintf("%s/login/desktop?code=%s&token=%s", baseURL, deviceCode, sessionToken)
}

// CreateDesktopLoginSession initiates a desktop login session
// This creates a session in PendingActivation state and returns a device code
// VULNERABILITY: Session token is issued BEFORE authentication!
func (a *App) CreateDesktopLoginSession(rctx request.CTX, request *DesktopLoginRequest) (*DesktopLoginResponse, *model.AppError) {
	// Validate request
	if request.DeviceName == "" {
		return nil, model.NewAppError("CreateDesktopLoginSession", "app.desktop_login.invalid_device_name.app_error", nil, "", http.StatusBadRequest)
	}
	
	if request.DevicePlatform == "" {
		request.DevicePlatform = "desktop"
	}
	
	if request.LoginMethod == "" {
		request.LoginMethod = "device_code"
	}
	
	// Generate device code
	deviceCode, err := generateDeviceCode()
	if err != nil {
		return nil, model.NewAppError("CreateDesktopLoginSession", "app.desktop_login.generate_code.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	// Create session in PendingActivation state
	// CRITICAL VULNERABILITY: We create the session and token BEFORE authentication
	// This means the token exists and could be intercepted before the user authenticates
	session := &model.Session{
		UserId:    "", // Will be set when device code is verified
		DeviceId:  deviceCode,
		Roles:     "",
		IsOAuth:   false,
		ExpiresAt: model.GetMillis() + int64(DesktopLoginDeviceCodeExpiry.Milliseconds()),
	}
	
	// Add desktop login specific properties
	session.AddProp(model.SessionPropDeviceCode, deviceCode)
	session.AddProp(model.SessionPropSessionState, model.SessionStatePendingActivation)
	session.AddProp(model.SessionPropDeviceName, request.DeviceName)
	session.AddProp(model.SessionPropDevicePlatform, request.DevicePlatform)
	session.AddProp(model.SessionPropAppVersion, request.AppVersion)
	session.AddProp(model.SessionPropLoginMethod, request.LoginMethod)
	session.AddProp(model.SessionPropCreatedAt, fmt.Sprintf("%d", time.Now().Unix()))
	
	// Save the session - this assigns a token
	// VULNERABILITY POINT: Token is generated here, before any user authentication
	savedSession, appErr := a.CreatePendingSession(rctx, session)
	if appErr != nil {
		return nil, appErr
	}
	
	rctx.Logger().Info("Created pending desktop login session",
		mlog.String("device_code", deviceCode),
		mlog.String("device_name", request.DeviceName),
		mlog.String("session_id", savedSession.Id),
	)
	
	// Build response
	response := &DesktopLoginResponse{
		SessionToken: savedSession.Token, // VULNERABILITY: Returning token before authentication
		DeviceCode:   deviceCode,
		ExpiresAt:    savedSession.ExpiresAt,
		VerifyURL:    fmt.Sprintf("%s/login/desktop/verify", *a.Config().ServiceSettings.SiteURL),
	}
	
	// Generate QR code for mobile scanning if requested
	if request.LoginMethod == "qr_code" {
		response.QRCode = generateQRCodeData(*a.Config().ServiceSettings.SiteURL, deviceCode, savedSession.Token)
	}
	
	return response, nil
}

// CreatePendingSession creates a session that hasn't been authenticated yet
func (a *App) CreatePendingSession(rctx request.CTX, session *model.Session) (*model.Session, *model.AppError) {
	// Check if there are too many pending sessions to prevent DoS
	pendingCount, err := a.Srv().Store().Session().GetPendingSessionCount(rctx)
	if err != nil {
		return nil, model.NewAppError("CreatePendingSession", "app.session.get_pending_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	if pendingCount >= DesktopLoginMaxPendingSessions {
		// Clean up expired pending sessions
		if cleanupErr := a.CleanupExpiredPendingSessions(rctx); cleanupErr != nil {
			rctx.Logger().Warn("Failed to cleanup expired pending sessions", mlog.Err(cleanupErr))
		}
	}
	
	// Create the session - this will generate a token
	createdSession, createErr := a.ch.srv.platform.CreateSession(rctx, session)
	if createErr != nil {
		return nil, model.NewAppError("CreatePendingSession", "app.session.save.app_error", nil, createErr.Error(), http.StatusInternalServerError)
	}
	
	return createdSession, nil
}

// VerifyDesktopLoginCode verifies a device code and returns session info
func (a *App) VerifyDesktopLoginCode(rctx request.CTX, deviceCode string) (*model.Session, *model.AppError) {
	if deviceCode == "" {
		return nil, model.NewAppError("VerifyDesktopLoginCode", "app.desktop_login.invalid_code.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Find pending session by device code
	session, err := a.Srv().Store().Session().GetSessionByDeviceCode(rctx, deviceCode)
	if err != nil {
		return nil, model.NewAppError("VerifyDesktopLoginCode", "app.desktop_login.code_not_found.app_error", nil, err.Error(), http.StatusNotFound)
	}
	
	// Verify session is in pending state
	if state, ok := session.Props[model.SessionPropSessionState]; !ok || state != model.SessionStatePendingActivation {
		return nil, model.NewAppError("VerifyDesktopLoginCode", "app.desktop_login.invalid_session_state.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Check if session has expired
	if session.IsExpired() {
		return nil, model.NewAppError("VerifyDesktopLoginCode", "app.desktop_login.code_expired.app_error", nil, "", http.StatusGone)
	}
	
	// Sanitize before returning to user
	session.Sanitize()
	
	return session, nil
}

// CompleteDesktopLogin completes the desktop login flow by activating the session
// VULNERABILITY: This method doesn't verify that the device making the completion
// request is the same device that initiated the login
func (a *App) CompleteDesktopLogin(rctx request.CTX, deviceCode string, userId string) (*model.Session, *model.AppError) {
	if deviceCode == "" {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.invalid_code.app_error", nil, "", http.StatusBadRequest)
	}
	
	if userId == "" {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.invalid_user.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Get the user to validate and get roles
	user, appErr := a.GetUser(userId)
	if appErr != nil {
		return nil, appErr
	}
	
	// Find the pending session
	session, err := a.Srv().Store().Session().GetSessionByDeviceCode(rctx, deviceCode)
	if err != nil {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.code_not_found.app_error", nil, err.Error(), http.StatusNotFound)
	}
	
	// Verify session is in pending state
	if state, ok := session.Props[model.SessionPropSessionState]; !ok || state != model.SessionStatePendingActivation {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.invalid_session_state.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Check if session has expired
	if session.IsExpired() {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.code_expired.app_error", nil, "", http.StatusGone)
	}
	
	// VULNERABILITY EXPLOITATION POINT: 
	// We activate the EXISTING session (with existing token) rather than creating a new one
	// An attacker who intercepted the token can now use it to impersonate the user
	session.UserId = user.Id
	session.Roles = user.GetRawRoles()
	session.Props[model.SessionPropSessionState] = model.SessionStateActive
	session.AddProp(model.SessionPropActivatedAt, fmt.Sprintf("%d", time.Now().Unix()))
	
	// Update session expiry based on device type
	devicePlatform := session.Props[model.SessionPropDevicePlatform]
	if devicePlatform == "mobile" {
		a.ch.srv.platform.SetSessionExpireInHours(session, *a.Config().ServiceSettings.SessionLengthMobileInHours)
	} else {
		a.ch.srv.platform.SetSessionExpireInHours(session, *a.Config().ServiceSettings.SessionLengthWebInHours)
	}
	
	// Update the session in the database
	updatedSession, updateErr := a.Srv().Store().Session().UpdateDeviceCodeSession(rctx, session)
	if updateErr != nil {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.update_session.app_error", nil, updateErr.Error(), http.StatusInternalServerError)
	}
	
	// Add session to cache for fast lookup
	if cacheErr := a.ch.srv.platform.AddSessionToCache(updatedSession); cacheErr != nil {
		rctx.Logger().Warn("Failed to add session to cache", mlog.Err(cacheErr))
	}
	
	rctx.Logger().Info("Completed desktop login",
		mlog.String("device_code", deviceCode),
		mlog.String("user_id", userId),
		mlog.String("session_id", updatedSession.Id),
	)
	
	return updatedSession, nil
}

// AttachDeviceToSession attaches device information to an existing session
// Used for session synchronization across devices
func (a *App) AttachDeviceToSession(rctx request.CTX, sessionId string, deviceInfo *model.DeviceInfo) *model.AppError {
	if sessionId == "" {
		return model.NewAppError("AttachDeviceToSession", "app.session.attach_device.invalid_session.app_error", nil, "", http.StatusBadRequest)
	}
	
	if deviceInfo == nil || deviceInfo.DeviceId == "" {
		return model.NewAppError("AttachDeviceToSession", "app.session.attach_device.invalid_device.app_error", nil, "", http.StatusBadRequest)
	}
	
	session, err := a.Srv().Store().Session().Get(rctx, sessionId)
	if err != nil {
		return model.NewAppError("AttachDeviceToSession", "app.session.attach_device.get_session.app_error", nil, err.Error(), http.StatusNotFound)
	}
	
	// VULNERABILITY: No verification that the device making this request
	// is authorized to attach to this session
	// Should verify device ownership through some secure mechanism
	
	// Update session with device info
	session.DeviceId = deviceInfo.DeviceId
	session.AddProp(model.SessionPropDeviceName, deviceInfo.DeviceName)
	session.AddProp(model.SessionPropDevicePlatform, deviceInfo.Platform)
	session.AddProp(model.SessionPropDeviceModel, deviceInfo.Model)
	session.AddProp(model.SessionPropDeviceOS, deviceInfo.OS)
	session.AddProp(model.SessionPropDeviceOSVersion, deviceInfo.OSVersion)
	session.AddProp(model.SessionPropDeviceAttachedAt, fmt.Sprintf("%d", time.Now().Unix()))
	
	// Save updated session
	if updateErr := a.Srv().Store().Session().UpdateDeviceId(rctx, session.Id, session.DeviceId, session.ExpiresAt); updateErr != nil {
		return model.NewAppError("AttachDeviceToSession", "app.session.attach_device.update.app_error", nil, updateErr.Error(), http.StatusInternalServerError)
	}
	
	// Update cache
	if cacheErr := a.ch.srv.platform.AddSessionToCache(session); cacheErr != nil {
		rctx.Logger().Warn("Failed to update session in cache", mlog.Err(cacheErr))
	}
	
	rctx.Logger().Info("Attached device to session",
		mlog.String("session_id", sessionId),
		mlog.String("device_id", deviceInfo.DeviceId),
	)
	
	return nil
}

// GetPendingDesktopSessions returns all pending desktop login sessions
func (a *App) GetPendingDesktopSessions(rctx request.CTX, limit int) ([]*model.Session, *model.AppError) {
	sessions, err := a.Srv().Store().Session().GetPendingSessions(rctx, limit)
	if err != nil {
		return nil, model.NewAppError("GetPendingDesktopSessions", "app.session.get_pending.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	// Sanitize sessions before returning
	for _, session := range sessions {
		session.Sanitize()
	}
	
	return sessions, nil
}

// CancelDesktopLogin cancels a pending desktop login session
func (a *App) CancelDesktopLogin(rctx request.CTX, deviceCode string) *model.AppError {
	if deviceCode == "" {
		return model.NewAppError("CancelDesktopLogin", "app.desktop_login.invalid_code.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Find the session
	session, err := a.Srv().Store().Session().GetSessionByDeviceCode(rctx, deviceCode)
	if err != nil {
		return model.NewAppError("CancelDesktopLogin", "app.desktop_login.code_not_found.app_error", nil, err.Error(), http.StatusNotFound)
	}
	
	// Revoke the session
	if revokeErr := a.RevokeSessionById(rctx, session.Id); revokeErr != nil {
		return revokeErr
	}
	
	rctx.Logger().Info("Cancelled desktop login",
		mlog.String("device_code", deviceCode),
		mlog.String("session_id", session.Id),
	)
	
	return nil
}

// CleanupExpiredPendingSessions removes expired pending sessions
func (a *App) CleanupExpiredPendingSessions(rctx request.CTX) *model.AppError {
	count, err := a.Srv().Store().Session().CleanupExpiredPendingSessions(rctx, model.GetMillis())
	if err != nil {
		return model.NewAppError("CleanupExpiredPendingSessions", "app.session.cleanup_pending.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	if count > 0 {
		rctx.Logger().Info("Cleaned up expired pending sessions", mlog.Int("count", count))
	}
	
	return nil
}

// RefreshDesktopSession extends a desktop session's expiry
func (a *App) RefreshDesktopSession(rctx request.CTX, sessionToken string) (*model.Session, *model.AppError) {
	session, appErr := a.GetSession(sessionToken)
	if appErr != nil {
		return nil, appErr
	}
	
	// Verify this is a desktop session
	if loginMethod, ok := session.Props[model.SessionPropLoginMethod]; !ok || (loginMethod != "device_code" && loginMethod != "qr_code") {
		return nil, model.NewAppError("RefreshDesktopSession", "app.desktop_login.not_desktop_session.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Extend expiry based on device platform
	devicePlatform := session.Props[model.SessionPropDevicePlatform]
	if devicePlatform == "mobile" {
		a.ch.srv.platform.SetSessionExpireInHours(session, *a.Config().ServiceSettings.SessionLengthMobileInHours)
	} else {
		a.ch.srv.platform.SetSessionExpireInHours(session, *a.Config().ServiceSettings.SessionLengthWebInHours)
	}
	
	// Update session
	if err := a.Srv().Store().Session().UpdateExpiresAt(rctx, session.Id, session.ExpiresAt); err != nil {
		return nil, model.NewAppError("RefreshDesktopSession", "app.session.update_expires.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	// Update cache
	if cacheErr := a.ch.srv.platform.AddSessionToCache(session); cacheErr != nil {
		rctx.Logger().Warn("Failed to update session in cache", mlog.Err(cacheErr))
	}
	
	return session, nil
}

// GenerateQRCodeForSession generates a QR code for an existing session
func (a *App) GenerateQRCodeForSession(rctx request.CTX, sessionId string) (string, *model.AppError) {
	session, err := a.Srv().Store().Session().Get(rctx, sessionId)
	if err != nil {
		return "", model.NewAppError("GenerateQRCodeForSession", "app.session.get.app_error", nil, err.Error(), http.StatusNotFound)
	}
	
	// Generate a temporary device code for QR scanning
	deviceCode, genErr := generateDeviceCode()
	if genErr != nil {
		return "", model.NewAppError("GenerateQRCodeForSession", "app.desktop_login.generate_code.app_error", nil, genErr.Error(), http.StatusInternalServerError)
	}
	
	// Store device code in session props with expiry
	session.AddProp(model.SessionPropQRDeviceCode, deviceCode)
	session.AddProp(model.SessionPropQRGeneratedAt, fmt.Sprintf("%d", time.Now().Unix()))
	
	// Generate QR code data
	qrCode := generateQRCodeData(*a.Config().ServiceSettings.SiteURL, deviceCode, session.Token)
	
	return qrCode, nil
}

// SyncDesktopSessions synchronizes sessions across multiple desktop devices
// This allows users to maintain consistent session state
func (a *App) SyncDesktopSessions(rctx request.CTX, userId string) ([]*model.Session, *model.AppError) {
	if userId == "" {
		return nil, model.NewAppError("SyncDesktopSessions", "app.desktop_login.invalid_user.app_error", nil, "", http.StatusBadRequest)
	}
	
	// Get all active desktop sessions for user
	allSessions, err := a.GetSessions(rctx, userId)
	if err != nil {
		return nil, err
	}
	
	// Filter for desktop sessions
	desktopSessions := make([]*model.Session, 0)
	for _, session := range allSessions {
		if loginMethod, ok := session.Props[model.SessionPropLoginMethod]; ok && (loginMethod == "device_code" || loginMethod == "qr_code") {
			desktopSessions = append(desktopSessions, session)
		}
	}
	
	// Sanitize before returning
	for _, session := range desktopSessions {
		session.Sanitize()
	}
	
	return desktopSessions, nil
}
