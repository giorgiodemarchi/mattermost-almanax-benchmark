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

// DesktopLoginResponse contains the device code for desktop login
// Note: session_token is NOT returned here - it's only issued after authentication
type DesktopLoginResponse struct {
	DeviceCode   string `json:"device_code"`
	ExpiresAt    int64  `json:"expires_at"`
	VerifyURL    string `json:"verify_url"`
	QRCode       string `json:"qr_code,omitempty"`
}

// DesktopLoginCompleteResponse contains the session token after successful authentication
type DesktopLoginCompleteResponse struct {
	SessionToken string `json:"session_token"`
	UserId       string `json:"user_id"`
	ExpiresAt    int64  `json:"expires_at"`
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
// FIXED: This now creates a pending login request WITHOUT a session token
// The token is only generated AFTER successful authentication
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
	
	// SECURITY FIX: Store device code metadata without creating a session
	// Session will only be created AFTER authentication completes
	expiresAt := model.GetMillis() + int64(DesktopLoginDeviceCodeExpiry.Milliseconds())
	
	metadata := &model.DeviceCodeMetadata{
		DeviceCode:     deviceCode,
		DeviceName:     request.DeviceName,
		DevicePlatform: request.DevicePlatform,
		AppVersion:     request.AppVersion,
		LoginMethod:    request.LoginMethod,
		CreatedAt:      time.Now().Unix(),
		ExpiresAt:      expiresAt,
	}
	
	// Store device code metadata (not a session)
	if appErr := a.StoreDeviceCodeMetadata(rctx, metadata); appErr != nil {
		return nil, appErr
	}
	
	rctx.Logger().Info("Created pending desktop login request",
		mlog.String("device_code", deviceCode),
		mlog.String("device_name", request.DeviceName),
	)
	
	// Build response WITHOUT session token
	response := &DesktopLoginResponse{
		DeviceCode:   deviceCode,
		ExpiresAt:    expiresAt,
		VerifyURL:    fmt.Sprintf("%s/login/desktop/verify", *a.Config().ServiceSettings.SiteURL),
	}
	
	// Generate QR code for mobile scanning if requested
	if request.LoginMethod == "qr_code" {
		response.QRCode = generateQRCodeData(*a.Config().ServiceSettings.SiteURL, deviceCode, "")
	}
	
	return response, nil
}

// StoreDeviceCodeMetadata stores device code metadata without creating a session
func (a *App) StoreDeviceCodeMetadata(rctx request.CTX, metadata *model.DeviceCodeMetadata) *model.AppError {
	// Check if there are too many pending codes to prevent DoS
	pendingCount, err := a.Srv().Store().Session().GetPendingDeviceCodeCount(rctx)
	if err != nil {
		return model.NewAppError("StoreDeviceCodeMetadata", "app.device_code.get_pending_count.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	if pendingCount >= DesktopLoginMaxPendingSessions {
		// Clean up expired device codes
		if cleanupErr := a.CleanupExpiredDeviceCodes(rctx); cleanupErr != nil {
			rctx.Logger().Warn("Failed to cleanup expired device codes", mlog.Err(cleanupErr))
		}
	}
	
	// Store device code metadata
	storeErr := a.Srv().Store().Session().StoreDeviceCodeMetadata(rctx, metadata)
	if storeErr != nil {
		return model.NewAppError("StoreDeviceCodeMetadata", "app.device_code.store.app_error", nil, storeErr.Error(), http.StatusInternalServerError)
	}
	
	return nil
}

// GetDeviceCodeMetadata retrieves device code metadata
func (a *App) GetDeviceCodeMetadata(rctx request.CTX, deviceCode string) (*model.DeviceCodeMetadata, error) {
	return a.Srv().Store().Session().GetDeviceCodeMetadata(rctx, deviceCode)
}

// DeleteDeviceCodeMetadata deletes device code metadata
func (a *App) DeleteDeviceCodeMetadata(rctx request.CTX, deviceCode string) *model.AppError {
	err := a.Srv().Store().Session().DeleteDeviceCodeMetadata(rctx, deviceCode)
	if err != nil {
		return model.NewAppError("DeleteDeviceCodeMetadata", "app.device_code.delete.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return nil
}

// CleanupExpiredDeviceCodes removes expired device code metadata
func (a *App) CleanupExpiredDeviceCodes(rctx request.CTX) *model.AppError {
	count, err := a.Srv().Store().Session().CleanupExpiredDeviceCodes(rctx, model.GetMillis())
	if err != nil {
		return model.NewAppError("CleanupExpiredDeviceCodes", "app.device_code.cleanup.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	
	if count > 0 {
		rctx.Logger().Info("Cleaned up expired device codes", mlog.Int("count", count))
	}
	
	return nil
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

// CompleteDesktopLogin completes the desktop login flow by creating a NEW session
// SECURITY FIX: Now creates a NEW session with a NEW token AFTER authentication
// The device code metadata is verified and then deleted to prevent reuse
func (a *App) CompleteDesktopLogin(rctx request.CTX, deviceCode string, userId string) (*DesktopLoginCompleteResponse, *model.AppError) {
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
	
	// SECURITY FIX: Retrieve device code metadata (not a session)
	metadata, err := a.GetDeviceCodeMetadata(rctx, deviceCode)
	if err != nil {
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.code_not_found.app_error", nil, err.Error(), http.StatusNotFound)
	}
	
	// Check if device code has expired
	if metadata.ExpiresAt < model.GetMillis() {
		// Delete expired metadata
		_ = a.DeleteDeviceCodeMetadata(rctx, deviceCode)
		return nil, model.NewAppError("CompleteDesktopLogin", "app.desktop_login.code_expired.app_error", nil, "", http.StatusGone)
	}
	
	// SECURITY FIX: Create a COMPLETELY NEW session with a NEW token
	// This prevents session fixation attacks
	newSession := &model.Session{
		UserId:   user.Id,
		DeviceId: deviceCode,
		Roles:    user.GetRawRoles(),
		IsOAuth:  false,
	}
	
	// Add desktop login properties from metadata
	newSession.AddProp(model.SessionPropDeviceCode, deviceCode)
	newSession.AddProp(model.SessionPropSessionState, model.SessionStateActive)
	newSession.AddProp(model.SessionPropDeviceName, metadata.DeviceName)
	newSession.AddProp(model.SessionPropDevicePlatform, metadata.DevicePlatform)
	newSession.AddProp(model.SessionPropAppVersion, metadata.AppVersion)
	newSession.AddProp(model.SessionPropLoginMethod, metadata.LoginMethod)
	newSession.AddProp(model.SessionPropCreatedAt, fmt.Sprintf("%d", metadata.CreatedAt))
	newSession.AddProp(model.SessionPropActivatedAt, fmt.Sprintf("%d", time.Now().Unix()))
	
	// Set session expiry based on device type
	if metadata.DevicePlatform == "mobile" {
		a.ch.srv.platform.SetSessionExpireInHours(newSession, *a.Config().ServiceSettings.SessionLengthMobileInHours)
	} else {
		a.ch.srv.platform.SetSessionExpireInHours(newSession, *a.Config().ServiceSettings.SessionLengthWebInHours)
	}
	
	// Create the new session (generates new token)
	createdSession, createErr := a.CreateSession(rctx, newSession)
	if createErr != nil {
		return nil, createErr
	}
	
	// SECURITY: Delete the device code metadata to prevent reuse
	if deleteErr := a.DeleteDeviceCodeMetadata(rctx, deviceCode); deleteErr != nil {
		rctx.Logger().Warn("Failed to delete device code metadata", mlog.Err(deleteErr))
	}
	
	rctx.Logger().Info("Completed desktop login with new session",
		mlog.String("device_code", deviceCode),
		mlog.String("user_id", userId),
		mlog.String("session_id", createdSession.Id),
	)
	
	// Return the NEW session token
	response := &DesktopLoginCompleteResponse{
		SessionToken: createdSession.Token,
		UserId:       userId,
		ExpiresAt:    createdSession.ExpiresAt,
	}
	
	return response, nil
}

// AttachDeviceToSession attaches device information to an existing session
// SECURITY FIX: Now requires the session to belong to the authenticated user
func (a *App) AttachDeviceToSession(rctx request.CTX, sessionId string, userId string, deviceInfo *model.DeviceInfo) *model.AppError {
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
	
	// SECURITY FIX: Verify the session belongs to the authenticated user
	if session.UserId != userId {
		return model.NewAppError("AttachDeviceToSession", "app.session.attach_device.permission_denied.app_error", nil, "user does not own this session", http.StatusForbidden)
	}
	
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
		mlog.String("user_id", userId),
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
