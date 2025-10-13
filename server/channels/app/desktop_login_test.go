// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDesktopLoginSession(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("successful desktop login session creation", func(t *testing.T) {
		request := &DesktopLoginRequest{
			DeviceName:    "Test Desktop App",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.SessionToken)
		assert.NotEmpty(t, response.DeviceCode)
		assert.NotZero(t, response.ExpiresAt)
		assert.NotEmpty(t, response.VerifyURL)
		assert.Len(t, response.DeviceCode, DesktopLoginDeviceCodeLength)
	})

	t.Run("QR code login session creation", func(t *testing.T) {
		request := &DesktopLoginRequest{
			DeviceName:    "Test Mobile App",
			DevicePlatform: "mobile",
			AppVersion:    "5.0.0",
			LoginMethod:   "qr_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)
		require.NotNil(t, response)
		assert.NotEmpty(t, response.SessionToken)
		assert.NotEmpty(t, response.DeviceCode)
		assert.NotEmpty(t, response.QRCode)
	})

	t.Run("missing device name", func(t *testing.T) {
		request := &DesktopLoginRequest{
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		_, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.invalid_device_name.app_error", err.Id)
	})

	t.Run("default platform and method", func(t *testing.T) {
		request := &DesktopLoginRequest{
			DeviceName: "Test Device",
			AppVersion: "5.0.0",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)
		assert.NotNil(t, response)
	})
}

func TestVerifyDesktopLoginCode(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("verify valid device code", func(t *testing.T) {
		// Create a pending session
		request := &DesktopLoginRequest{
			DeviceName:    "Test Device",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)

		// Verify the device code
		session, verifyErr := th.App.VerifyDesktopLoginCode(th.Context, response.DeviceCode)
		require.Nil(t, verifyErr)
		require.NotNil(t, session)
		assert.True(t, session.IsPendingActivation())
		assert.Equal(t, response.DeviceCode, session.GetDeviceCode())
	})

	t.Run("verify invalid device code", func(t *testing.T) {
		_, err := th.App.VerifyDesktopLoginCode(th.Context, "INVALID")
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.code_not_found.app_error", err.Id)
	})

	t.Run("verify empty device code", func(t *testing.T) {
		_, err := th.App.VerifyDesktopLoginCode(th.Context, "")
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.invalid_code.app_error", err.Id)
	})
}

func TestCompleteDesktopLogin(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("complete desktop login successfully", func(t *testing.T) {
		// Create a pending session
		request := &DesktopLoginRequest{
			DeviceName:    "Test Device",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)

		// Complete the login
		session, completeErr := th.App.CompleteDesktopLogin(th.Context, response.DeviceCode, th.BasicUser.Id)
		require.Nil(t, completeErr)
		require.NotNil(t, session)
		assert.Equal(t, th.BasicUser.Id, session.UserId)
		assert.Equal(t, th.BasicUser.GetRawRoles(), session.Roles)
		assert.False(t, session.IsPendingActivation())
		assert.Equal(t, model.SessionStateActive, session.Props[model.SessionPropSessionState])
	})

	t.Run("complete with invalid device code", func(t *testing.T) {
		_, err := th.App.CompleteDesktopLogin(th.Context, "INVALID", th.BasicUser.Id)
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.code_not_found.app_error", err.Id)
	})

	t.Run("complete with empty device code", func(t *testing.T) {
		_, err := th.App.CompleteDesktopLogin(th.Context, "", th.BasicUser.Id)
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.invalid_code.app_error", err.Id)
	})

	t.Run("complete with empty user id", func(t *testing.T) {
		// Create a pending session
		request := &DesktopLoginRequest{
			DeviceName:    "Test Device",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)

		_, completeErr := th.App.CompleteDesktopLogin(th.Context, response.DeviceCode, "")
		require.NotNil(t, completeErr)
		assert.Equal(t, "app.desktop_login.invalid_user.app_error", completeErr.Id)
	})

	t.Run("complete with invalid user id", func(t *testing.T) {
		// Create a pending session
		request := &DesktopLoginRequest{
			DeviceName:    "Test Device",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)

		_, completeErr := th.App.CompleteDesktopLogin(th.Context, response.DeviceCode, "invaliduserid")
		require.NotNil(t, completeErr)
	})
}

func TestAttachDeviceToSession(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("attach device to session successfully", func(t *testing.T) {
		// Create a session
		session := &model.Session{
			UserId: th.BasicUser.Id,
			Roles:  th.BasicUser.GetRawRoles(),
		}

		savedSession, err := th.App.CreateSession(th.Context, session)
		require.Nil(t, err)

		deviceInfo := &model.DeviceInfo{
			DeviceId:   "test-device-123",
			DeviceName: "Test Device",
			Platform:   "desktop",
			Model:      "MacBook Pro",
			OS:         "macOS",
			OSVersion:  "13.0",
		}

		attachErr := th.App.AttachDeviceToSession(th.Context, savedSession.Id, deviceInfo)
		require.Nil(t, attachErr)
	})

	t.Run("attach device with empty session id", func(t *testing.T) {
		deviceInfo := &model.DeviceInfo{
			DeviceId:   "test-device-123",
			DeviceName: "Test Device",
			Platform:   "desktop",
		}

		err := th.App.AttachDeviceToSession(th.Context, "", deviceInfo)
		require.NotNil(t, err)
		assert.Equal(t, "app.session.attach_device.invalid_session.app_error", err.Id)
	})

	t.Run("attach device with nil device info", func(t *testing.T) {
		session := &model.Session{
			UserId: th.BasicUser.Id,
			Roles:  th.BasicUser.GetRawRoles(),
		}

		savedSession, err := th.App.CreateSession(th.Context, session)
		require.Nil(t, err)

		attachErr := th.App.AttachDeviceToSession(th.Context, savedSession.Id, nil)
		require.NotNil(t, attachErr)
		assert.Equal(t, "app.session.attach_device.invalid_device.app_error", attachErr.Id)
	})

	t.Run("attach device with empty device id", func(t *testing.T) {
		session := &model.Session{
			UserId: th.BasicUser.Id,
			Roles:  th.BasicUser.GetRawRoles(),
		}

		savedSession, err := th.App.CreateSession(th.Context, session)
		require.Nil(t, err)

		deviceInfo := &model.DeviceInfo{
			DeviceName: "Test Device",
			Platform:   "desktop",
		}

		attachErr := th.App.AttachDeviceToSession(th.Context, savedSession.Id, deviceInfo)
		require.NotNil(t, attachErr)
		assert.Equal(t, "app.session.attach_device.invalid_device.app_error", attachErr.Id)
	})
}

func TestCancelDesktopLogin(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("cancel pending desktop login", func(t *testing.T) {
		// Create a pending session
		request := &DesktopLoginRequest{
			DeviceName:    "Test Device",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)

		// Cancel the login
		cancelErr := th.App.CancelDesktopLogin(th.Context, response.DeviceCode)
		require.Nil(t, cancelErr)

		// Verify session is cancelled
		_, verifyErr := th.App.VerifyDesktopLoginCode(th.Context, response.DeviceCode)
		require.NotNil(t, verifyErr)
	})

	t.Run("cancel with empty device code", func(t *testing.T) {
		err := th.App.CancelDesktopLogin(th.Context, "")
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.invalid_code.app_error", err.Id)
	})

	t.Run("cancel with invalid device code", func(t *testing.T) {
		err := th.App.CancelDesktopLogin(th.Context, "INVALID")
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.code_not_found.app_error", err.Id)
	})
}

func TestRefreshDesktopSession(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("refresh desktop session successfully", func(t *testing.T) {
		// Create and complete a desktop login
		request := &DesktopLoginRequest{
			DeviceName:    "Test Device",
			DevicePlatform: "desktop",
			AppVersion:    "5.0.0",
			LoginMethod:   "device_code",
		}

		response, err := th.App.CreateDesktopLoginSession(th.Context, request)
		require.Nil(t, err)

		session, completeErr := th.App.CompleteDesktopLogin(th.Context, response.DeviceCode, th.BasicUser.Id)
		require.Nil(t, completeErr)

		originalExpiry := session.ExpiresAt

		// Wait a moment to ensure time changes
		time.Sleep(10 * time.Millisecond)

		// Refresh the session
		refreshedSession, refreshErr := th.App.RefreshDesktopSession(th.Context, session.Token)
		require.Nil(t, refreshErr)
		assert.NotNil(t, refreshedSession)
		assert.Greater(t, refreshedSession.ExpiresAt, originalExpiry)
	})

	t.Run("refresh non-desktop session fails", func(t *testing.T) {
		// Create a regular session
		session := &model.Session{
			UserId: th.BasicUser.Id,
			Roles:  th.BasicUser.GetRawRoles(),
		}

		savedSession, err := th.App.CreateSession(th.Context, session)
		require.Nil(t, err)

		_, refreshErr := th.App.RefreshDesktopSession(th.Context, savedSession.Token)
		require.NotNil(t, refreshErr)
		assert.Equal(t, "app.desktop_login.not_desktop_session.app_error", refreshErr.Id)
	})
}

func TestSyncDesktopSessions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("sync desktop sessions", func(t *testing.T) {
		// Create multiple desktop sessions
		for i := 0; i < 3; i++ {
			request := &DesktopLoginRequest{
				DeviceName:    "Test Device",
				DevicePlatform: "desktop",
				AppVersion:    "5.0.0",
				LoginMethod:   "device_code",
			}

			response, err := th.App.CreateDesktopLoginSession(th.Context, request)
	require.Nil(t, err)

			_, completeErr := th.App.CompleteDesktopLogin(th.Context, response.DeviceCode, th.BasicUser.Id)
			require.Nil(t, completeErr)
		}

		// Sync sessions
		sessions, err := th.App.SyncDesktopSessions(th.Context, th.BasicUser.Id)
	require.Nil(t, err)
		assert.GreaterOrEqual(t, len(sessions), 3)

		// Verify all are desktop sessions
		for _, session := range sessions {
			assert.True(t, session.IsDesktopSession())
		}
	})

	t.Run("sync with empty user id", func(t *testing.T) {
		_, err := th.App.SyncDesktopSessions(th.Context, "")
		require.NotNil(t, err)
		assert.Equal(t, "app.desktop_login.invalid_user.app_error", err.Id)
	})
}

func TestGenerateQRCodeForSession(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("generate QR code for session", func(t *testing.T) {
		// Create a session
		session := &model.Session{
			UserId: th.BasicUser.Id,
			Roles:  th.BasicUser.GetRawRoles(),
		}

		savedSession, err := th.App.CreateSession(th.Context, session)
	require.Nil(t, err)

		qrCode, qrErr := th.App.GenerateQRCodeForSession(th.Context, savedSession.Id)
		require.Nil(t, qrErr)
		assert.NotEmpty(t, qrCode)
		assert.Contains(t, qrCode, "/login/desktop")
	})

	t.Run("generate QR code for non-existent session", func(t *testing.T) {
		_, err := th.App.GenerateQRCodeForSession(th.Context, "invalid-session-id")
		require.NotNil(t, err)
	})
}

func TestCleanupExpiredPendingSessions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("cleanup expired pending sessions", func(t *testing.T) {
		// Create a pending session with short expiry
		session := &model.Session{
			UserId:    "",
			DeviceId:  "test-device",
			Roles:     "",
			IsOAuth:   false,
			ExpiresAt: model.GetMillis() - 1000, // Expired
		}
		session.AddProp(model.SessionPropDeviceCode, "TESTCODE")
		session.AddProp(model.SessionPropSessionState, model.SessionStatePendingActivation)

		_, err := th.App.CreatePendingSession(th.Context, session)
		require.Nil(t, err)

		// Cleanup expired sessions
		cleanupErr := th.App.CleanupExpiredPendingSessions(th.Context)
		require.Nil(t, cleanupErr)
	})
}

func TestSessionHelperMethods(t *testing.T) {
	t.Run("IsPendingActivation", func(t *testing.T) {
		session := &model.Session{
			Props: make(map[string]string),
		}

		// Test false case
		assert.False(t, session.IsPendingActivation())

		// Test true case
		session.AddProp(model.SessionPropSessionState, model.SessionStatePendingActivation)
		assert.True(t, session.IsPendingActivation())
	})

	t.Run("IsDesktopSession", func(t *testing.T) {
		session := &model.Session{
			Props: make(map[string]string),
		}

		// Test false case
		assert.False(t, session.IsDesktopSession())

		// Test device_code login
		session.AddProp(model.SessionPropLoginMethod, "device_code")
		assert.True(t, session.IsDesktopSession())

		// Test qr_code login
		session.Props[model.SessionPropLoginMethod] = "qr_code"
		assert.True(t, session.IsDesktopSession())
	})

	t.Run("GetDeviceCode", func(t *testing.T) {
		session := &model.Session{
			Props: make(map[string]string),
		}

		// Test empty case
		assert.Empty(t, session.GetDeviceCode())

		// Test with code
		session.AddProp(model.SessionPropDeviceCode, "ABC12345")
		assert.Equal(t, "ABC12345", session.GetDeviceCode())
	})
}

// NOTE: These tests verify that the desktop login feature works correctly
// BUT they do NOT test the security vulnerability where:
// 1. Session token is issued BEFORE authentication completes
// 2. An attacker could intercept the token during the pending state
// 3. When victim completes authentication, attacker can use the pre-issued token
// 4. AttachDeviceToSession doesn't verify device ownership

// Missing test cases that would catch the vulnerability:
// - Test that session token cannot be used while in pending state
// - Test that completing login issues a NEW token, not activates existing one
// - Test that device attachment requires cryptographic proof of ownership
// - Test for session fixation attack scenario
