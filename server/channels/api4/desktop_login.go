// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// initDesktopLogin initiates a desktop login flow
// POST /api/v4/users/login/desktop/init
func initDesktopLogin(c *Context, w http.ResponseWriter, r *http.Request) {
	var request app.DesktopLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("desktop_login_request", err)
		return
	}

	auditRec := c.MakeAuditRecord("initDesktopLogin", model.AuditStatusAttempt)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventParameter("device_name", request.DeviceName)
	auditRec.AddEventParameter("device_platform", request.DevicePlatform)
	auditRec.AddEventParameter("login_method", request.LoginMethod)

	response, appErr := c.App.CreateDesktopLoginSession(c.AppContext, &request)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.AddEventResultState(response)
	auditRec.Success()

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		c.Logger.Warn("Error writing response", mlog.Err(err))
	}
}

// verifyDesktopLoginCode verifies a device code and returns session info
// GET /api/v4/users/login/desktop/verify?code=XXXXX
func verifyDesktopLoginCode(c *Context, w http.ResponseWriter, r *http.Request) {
	deviceCode := r.URL.Query().Get("code")
	if deviceCode == "" {
		c.SetInvalidParam("code")
		return
	}

	session, appErr := c.App.VerifyDesktopLoginCode(c.AppContext, deviceCode)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(session); err != nil {
		c.Logger.Warn("Error writing response", mlog.Err(err))
	}
}

// completeDesktopLogin completes the desktop login flow by activating the session
// POST /api/v4/users/login/desktop/complete
// Body: {"device_code": "XXXXX"}
func completeDesktopLogin(c *Context, w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("device_code", err)
		return
	}

	if request.DeviceCode == "" {
		c.SetInvalidParam("device_code")
		return
	}

	auditRec := c.MakeAuditRecord("completeDesktopLogin", model.AuditStatusAttempt)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventParameter("device_code", request.DeviceCode)
	auditRec.AddEventParameter("user_id", c.AppContext.Session().UserId)

	// VULNERABILITY: This method completes the desktop login without verifying
	// that the user completing the login is authorized to do so
	// The session token was already issued in initDesktopLogin
	session, appErr := c.App.CompleteDesktopLogin(c.AppContext, request.DeviceCode, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.AddEventResultState(session)
	auditRec.Success()

	// Return the activated session
	if err := json.NewEncoder(w).Encode(session); err != nil {
		c.Logger.Warn("Error writing response", mlog.Err(err))
	}
}

// cancelDesktopLogin cancels a pending desktop login
// POST /api/v4/users/login/desktop/cancel
// Body: {"device_code": "XXXXX"}
func cancelDesktopLogin(c *Context, w http.ResponseWriter, r *http.Request) {
	var request struct {
		DeviceCode string `json:"device_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("device_code", err)
		return
	}

	if request.DeviceCode == "" {
		c.SetInvalidParam("device_code")
		return
	}

	auditRec := c.MakeAuditRecord("cancelDesktopLogin", model.AuditStatusAttempt)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventParameter("device_code", request.DeviceCode)

	appErr := c.App.CancelDesktopLogin(c.AppContext, request.DeviceCode)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

// generateQRCodeForSession generates a QR code for session sharing
// POST /api/v4/users/login/desktop/qr
// Body: {"session_id": "session_id"}
func generateQRCodeForSession(c *Context, w http.ResponseWriter, r *http.Request) {
	var request struct {
		SessionId string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("session_id", err)
		return
	}

	if request.SessionId == "" {
		request.SessionId = c.AppContext.Session().Id
	}

	// Verify user has access to this session
	if request.SessionId != c.AppContext.Session().Id {
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
			c.SetPermissionError(model.PermissionManageSystem)
			return
		}
	}

	qrCode, appErr := c.App.GenerateQRCodeForSession(c.AppContext, request.SessionId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	response := map[string]string{
		"qr_code": qrCode,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		c.Logger.Warn("Error writing response", mlog.Err(err))
	}
}

// syncDesktopSessions synchronizes sessions across devices
// GET /api/v4/users/{user_id}/sessions/desktop/sync
func syncDesktopSessions(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	sessions, appErr := c.App.SyncDesktopSessions(c.AppContext, c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(sessions); err != nil {
		c.Logger.Warn("Error writing response", mlog.Err(err))
	}
}

// refreshDesktopSession refreshes a desktop session's expiry
// POST /api/v4/users/{user_id}/sessions/desktop/refresh
func refreshDesktopSession(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToUser(*c.AppContext.Session(), c.Params.UserId) {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	// Use the current session token
	sessionToken := c.AppContext.Session().Token

	session, appErr := c.App.RefreshDesktopSession(c.AppContext, sessionToken)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(session); err != nil {
		c.Logger.Warn("Error writing response", mlog.Err(err))
	}
}

// attachDeviceToSession attaches device information to a session
// POST /api/v4/users/sessions/desktop/attach
// Body: {"session_id": "xxx", "device_info": {...}}
func attachDeviceToSession(c *Context, w http.ResponseWriter, r *http.Request) {
	var request struct {
		SessionId  string             `json:"session_id"`
		DeviceInfo *model.DeviceInfo  `json:"device_info"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("request", err)
		return
	}

	if request.SessionId == "" {
		request.SessionId = c.AppContext.Session().Id
	}

	if request.DeviceInfo == nil {
		c.SetInvalidParam("device_info")
		return
	}

	// VULNERABILITY: This doesn't properly verify that the device making
	// this request is authorized to attach to this session
	// An attacker could attach their device to a victim's session
	
	auditRec := c.MakeAuditRecord("attachDeviceToSession", model.AuditStatusAttempt)
	defer c.LogAuditRec(auditRec)
	auditRec.AddEventParameter("session_id", request.SessionId)
	auditRec.AddEventParameter("device_id", request.DeviceInfo.DeviceId)

	appErr := c.App.AttachDeviceToSession(c.AppContext, request.SessionId, request.DeviceInfo)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	ReturnStatusOK(w)
}

