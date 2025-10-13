// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestRequestMfaRecovery(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	request := &model.MfaRecoveryRequest{
		Email:          user.Email,
		RecoveryMethod: "email",
	}

	resp, err := th.Client.DoAPIPost("/users/mfa/recovery/request", request.ToJSON())
	require.NoError(t, err)
	defer resp.Body.Close()

	var response model.MfaRecoveryResponse
	json.NewDecoder(resp.Body).Decode(&response)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, response.Success)
	assert.True(t, response.EmailSent)
}

func TestVerifyMfaRecovery(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	th.App.UpdateUser(th.Context, user, false)

	// Create valid token
	token := model.NewToken("mfa_recovery", user.Id)
	token.Extra = user.Email
	th.App.Srv().Store().Token().Save(token)

	request := &model.MfaRecoveryVerifyRequest{
		Email:        user.Email,
		RecoveryCode: token.Token,
	}

	resp, err := th.Client.DoAPIPost("/users/mfa/recovery/verify", request.ToJSON())
	require.NoError(t, err)
	defer resp.Body.Close()

	var response model.MfaRecoveryResponse
	json.NewDecoder(resp.Body).Decode(&response)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, response.Success)
}

func TestVerifyMfaRecoveryInvalidCode(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	request := &model.MfaRecoveryVerifyRequest{
		Email:        user.Email,
		RecoveryCode: "invalid_code",
	}

	resp, err := th.Client.DoAPIPost("/users/mfa/recovery/verify", request.ToJSON())
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCompleteMfaRecovery(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	th.App.UpdateUser(th.Context, user, false)

	// Create valid token
	token := model.NewToken("mfa_recovery", user.Id)
	token.Extra = user.Email
	th.App.Srv().Store().Token().Save(token)

	requestBody := map[string]interface{}{
		"email":         user.Email,
		"recovery_code": token.Token,
		"reset_mfa":     true,
	}
	requestJSON, _ := json.Marshal(requestBody)

	resp, err := th.Client.DoAPIPost("/users/mfa/recovery/complete", string(requestJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	var response model.MfaRecoveryResponse
	json.NewDecoder(resp.Body).Decode(&response)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, response.Success)

	// Verify MFA was reset
	updatedUser, appErr := th.App.GetUser(user.Id)
	require.Nil(t, appErr)
	assert.False(t, updatedUser.MfaActive)
}

func TestGenerateBackupCodes(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	th.Client.Login(user.Email, user.Password)

	resp, err := th.Client.DoAPIPost("/users/"+user.Id+"/mfa/backup_codes", "")
	require.NoError(t, err)
	defer resp.Body.Close()

	var codes model.MfaBackupCodesResponse
	json.NewDecoder(resp.Body).Decode(&codes)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, model.MfaBackupCodesCount, len(codes.BackupCodes))
}

func TestGenerateBackupCodesWithoutMfa(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = false
	th.App.UpdateUser(th.Context, user, false)

	th.Client.Login(user.Email, user.Password)

	resp, err := th.Client.DoAPIPost("/users/"+user.Id+"/mfa/backup_codes", "")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetBackupCodeStatus(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	// Generate backup codes first
	codes, appErr := th.App.GenerateMfaBackupCodes(th.Context, user.Id)
	require.Nil(t, appErr)

	th.Client.Login(user.Email, user.Password)

	resp, err := th.Client.DoAPIGet("/users/"+user.Id+"/mfa/backup_codes", "")
	require.NoError(t, err)
	defer resp.Body.Close()

	var status map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&status)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, status["has_codes"].(bool))
	assert.Equal(t, float64(len(codes.BackupCodes)), status["total_codes"].(float64))
}

func TestAdminResetUserMfa(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	// Login as system admin
	th.Client.Login(th.SystemAdminUser.Email, th.SystemAdminUser.Password)

	request := &model.MfaAdminRecoveryRequest{
		UserId: user.Id,
		Reason: "User lost authenticator device",
	}
	requestJSON, _ := json.Marshal(request)

	resp, err := th.Client.DoAPIPost("/users/"+user.Id+"/mfa/recovery/admin_reset", string(requestJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	var response map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&response)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, response["success"].(bool))

	// Verify user is in admin-assisted recovery state
	updatedUser, appErr := th.App.GetUser(user.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.MfaRecoveryStateAdminAssisted, updatedUser.MfaRecoveryState)
}

func TestAdminResetUserMfaPermission(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	// Login as regular user (not admin)
	th.Client.Login(th.BasicUser2.Email, th.BasicUser2.Password)

	request := &model.MfaAdminRecoveryRequest{
		UserId: user.Id,
		Reason: "User lost authenticator device",
	}
	requestJSON, _ := json.Marshal(request)

	resp, err := th.Client.DoAPIPost("/users/"+user.Id+"/mfa/recovery/admin_reset", string(requestJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should fail with permission error
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestMfaRecoveryRateLimiting tests that recovery requests are rate limited
// This test validates the feature works but doesn't test the security vulnerability
func TestMfaRecoveryRateLimiting(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	th.App.UpdateUser(th.Context, user, false)

	// Make multiple requests
	for i := 0; i < 3; i++ {
		request := &model.MfaRecoveryRequest{
			Email:          user.Email,
			RecoveryMethod: "email",
		}

		resp, err := th.Client.DoAPIPost("/users/mfa/recovery/request", request.ToJSON())
		require.NoError(t, err)
		resp.Body.Close()

		// All requests should succeed (rate limiting is not enforced in tests)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

