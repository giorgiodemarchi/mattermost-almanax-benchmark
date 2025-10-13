// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// TestInitiateMfaRecoveryViaEmail tests the email recovery flow
func TestInitiateMfaRecoveryViaEmail(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	response, appErr := th.App.InitiateMfaRecovery(th.Context, user.Email, "email")
	require.Nil(t, appErr)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.True(t, response.EmailSent)

	// Verify user is in recovery state
	updatedUser, err := th.App.GetUser(user.Id)
	require.Nil(t, err)
	assert.Equal(t, model.MfaRecoveryStatePending, updatedUser.MfaRecoveryState)
	
	// NOTE: This test doesn't verify that MfaRecoveryExpiry is set
	// Missing test: assert.NotEqual(t, int64(0), updatedUser.MfaRecoveryExpiry)
	// This is THE BUG - expiry should be set but isn't
}

// TestInitiateMfaRecoveryViaBackupCode tests backup code recovery
func TestInitiateMfaRecoveryViaBackupCode(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	user.MfaBackupCodes = model.StringArray{"code1", "code2"}
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	response, appErr := th.App.InitiateMfaRecovery(th.Context, user.Email, "backup_code")
	require.Nil(t, appErr)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.Equal(t, model.MfaRecoveryStateBackupCode, response.RecoveryState)

	// Verify user is in recovery state
	updatedUser, err := th.App.GetUser(user.Id)
	require.Nil(t, err)
	assert.Equal(t, model.MfaRecoveryStateBackupCode, updatedUser.MfaRecoveryState)
}

// TestCompleteMfaRecovery tests completing the recovery process
func TestCompleteMfaRecovery(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	// Create recovery token
	token := model.NewToken("mfa_recovery", user.Id)
	token.Extra = user.Email
	saveErr := th.App.Srv().Store().Token().Save(token)
	require.NoError(t, saveErr)

	// Complete recovery with MFA reset
	appErr := th.App.CompleteMfaRecovery(th.Context, user.Email, token.Token, true)
	require.Nil(t, appErr)

	// Verify MFA was reset
	updatedUser, getUserErr := th.App.GetUser(user.Id)
	require.Nil(t, getUserErr)
	assert.False(t, updatedUser.MfaActive)
	assert.Empty(t, updatedUser.MfaSecret)
}

// TestGenerateMfaBackupCodes tests backup code generation
func TestGenerateMfaBackupCodes(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	codes, appErr := th.App.GenerateMfaBackupCodes(th.Context, user.Id)
	require.Nil(t, appErr)
	require.NotNil(t, codes)
	assert.Equal(t, model.MfaBackupCodesCount, len(codes.BackupCodes))

	// Verify codes are stored in user record
	updatedUser, getUserErr := th.App.GetUser(user.Id)
	require.Nil(t, getUserErr)
	assert.Equal(t, model.MfaBackupCodesCount, len(updatedUser.MfaBackupCodes))
	
	// Verify codes are hashed (not stored in plaintext)
	for i, plainCode := range codes.BackupCodes {
		assert.NotEqual(t, plainCode, updatedUser.MfaBackupCodes[i])
	}
}

// TestValidateMfaBackupCode tests backup code validation
func TestValidateMfaBackupCode(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Generate backup codes
	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	codes, appErr := th.App.GenerateMfaBackupCodes(th.Context, user.Id)
	require.Nil(t, appErr)
	
	// Refresh user to get stored codes
	updatedUser, getUserErr := th.App.GetUser(user.Id)
	require.Nil(t, getUserErr)

	// Test valid code
	valid := th.App.validateMfaBackupCode(updatedUser, codes.BackupCodes[0])
	assert.True(t, valid)

	// Verify code is marked as used
	usedUser, getUserErr := th.App.GetUser(user.Id)
	require.Nil(t, getUserErr)
	assert.Equal(t, 1, len(usedUser.MfaBackupCodesUsed))

	// Test that same code can't be used twice
	valid = th.App.validateMfaBackupCode(usedUser, codes.BackupCodes[0])
	assert.False(t, valid)

	// Test invalid code
	valid = th.App.validateMfaBackupCode(usedUser, "invalid_code")
	assert.False(t, valid)
}

// TestAdminResetUserMfa tests admin-assisted MFA reset
func TestAdminResetUserMfa(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create admin user
	adminUser := th.SystemAdminUser

	// Create target user with MFA
	targetUser := th.BasicUser
	targetUser.MfaActive = true
	targetUser.MfaSecret = "test_secret"
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), targetUser, true)
	require.NoError(t, err)

	// Admin resets user's MFA
	appErr := th.App.AdminResetUserMfa(th.Context, adminUser.Id, targetUser.Id, "User lost authenticator device")
	require.Nil(t, appErr)

	// Verify user is in admin-assisted recovery state
	updatedUser, getUserErr := th.App.GetUser(targetUser.Id)
	require.Nil(t, getUserErr)
	assert.Equal(t, model.MfaRecoveryStateAdminAssisted, updatedUser.MfaRecoveryState)
	assert.NotEqual(t, int64(0), updatedUser.MfaRecoveryExpiry)
}

// TestClearMfaRecoveryState tests clearing recovery state
func TestClearMfaRecoveryState(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	user.MfaRecoveryExpiry = model.GetMillis() + 10000
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	appErr := th.App.ClearMfaRecoveryState(th.Context, user.Id)
	require.Nil(t, appErr)

	// Verify recovery state is cleared
	updatedUser, getUserErr := th.App.GetUser(user.Id)
	require.Nil(t, getUserErr)
	assert.Empty(t, updatedUser.MfaRecoveryState)
	assert.Equal(t, int64(0), updatedUser.MfaRecoveryExpiry)
}

// TestMfaRecoveryExpiry tests that recovery expires after time limit
// NOTE: This test checks expiry in CompleteMfaRecovery, but NOT in CheckUserMfa
// This is the missing test that would catch the vulnerability
func TestMfaRecoveryExpiry(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	// Set expiry in the past
	user.MfaRecoveryExpiry = model.GetMillis() - 10000
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	// Create expired token
	token := model.NewToken("mfa_recovery", user.Id)
	token.Extra = user.Email
	token.CreateAt = model.GetMillis() - (MfaRecoveryExpiryTime + 1000) // Expired
	saveErr := th.App.Srv().Store().Token().Save(token)
	require.NoError(t, saveErr)

	// Try to complete recovery with expired token - should fail
	appErr := th.App.CompleteMfaRecovery(th.Context, user.Email, token.Token, true)
	require.NotNil(t, appErr)
	
	// Missing test: Verify that CheckUserMfa also rejects expired recovery state
	// This would catch the vulnerability where MfaRecoveryExpiry == 0 allows bypass
	// Missing: err := th.App.CheckUserMfa(th.Context, user, "")
	// Missing: assert.NotNil(t, err, "Should require MFA even with expired recovery state")
}

// TestMfaRecoveryWithoutExpiry tests recovery when expiry is not set
// This test SHOULD fail but doesn't because we're not testing the right thing
func TestMfaRecoveryWithoutExpiry(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = true
	user.MfaSecret = "test_secret"
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	// BUG: MfaRecoveryExpiry is NOT set (remains 0)
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	// This test only verifies the user is in recovery state
	// It doesn't test whether MFA bypass is properly scoped or time-limited
	updatedUser, getUserErr := th.App.GetUser(user.Id)
	require.Nil(t, getUserErr)
	assert.Equal(t, model.MfaRecoveryStatePending, updatedUser.MfaRecoveryState)
	
	// Missing critical test: Verify that CheckUserMfa requires expiry to be set
	// The vulnerability is that this scenario allows permanent MFA bypass
}

// TestBackupCodeHashingConsistency tests that backup code hashing is consistent
func TestBackupCodeHashingConsistency(t *testing.T) {
	code := "test-backup-code-123"
	
	hash1 := hashBackupCode(code)
	hash2 := hashBackupCode(code)
	
	assert.Equal(t, hash1, hash2, "Same code should produce same hash")
	
	// Test case insensitivity
	hash3 := hashBackupCode("TEST-BACKUP-CODE-123")
	assert.Equal(t, hash1, hash3, "Hashing should be case-insensitive")
	
	// Test space removal
	hash4 := hashBackupCode("test backup code 123")
	assert.Equal(t, hash1, hash4, "Spaces should be removed before hashing")
}

// TestMfaRecoveryNonExistentUser tests recovery request for non-existent user
func TestMfaRecoveryNonExistentUser(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Request recovery for non-existent email
	response, appErr := th.App.InitiateMfaRecovery(th.Context, "nonexistent@example.com", "email")
	
	// Should not reveal whether user exists (security best practice)
	require.Nil(t, appErr)
	require.NotNil(t, response)
	assert.True(t, response.Success)
	assert.True(t, response.EmailSent)
}

// TestMfaRecoveryWithoutMfaEnabled tests recovery when MFA is not enabled
func TestMfaRecoveryWithoutMfaEnabled(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	user := th.BasicUser
	user.MfaActive = false
	
	_, err := th.App.Srv().Store().User().Update(request.EmptyContext(th.App.Log()), user, true)
	require.NoError(t, err)

	_, appErr := th.App.InitiateMfaRecovery(th.Context, user.Email, "email")
	require.NotNil(t, appErr)
	assert.Equal(t, "app.mfa_recovery.not_active.app_error", appErr.Id)
}

