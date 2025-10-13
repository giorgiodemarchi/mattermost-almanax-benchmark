// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

const (
	TokenTypeMfaRecovery     = "mfa_recovery"
	MfaRecoveryExpiryTime    = 1000 * 60 * 60 * 24 // 24 hours
	MfaRecoveryCodeLength    = 32
	MfaBackupCodeHashRounds  = 10
)

// InitiateMfaRecovery starts the MFA recovery process for a user
// This sets the user's recovery state to "pending" to allow them to log in
// and reset their MFA settings
func (a *App) InitiateMfaRecovery(rctx request.CTX, email string, method string) (*model.MfaRecoveryResponse, *model.AppError) {
	user, err := a.GetUserByEmail(email)
	if err != nil {
		// Don't reveal if user exists or not - security best practice
		return &model.MfaRecoveryResponse{
			Success: true,
			EmailSent: true,
			Message: "If an account with that email exists, a recovery email has been sent.",
		}, nil
	}

	// Verify user has MFA enabled
	if !user.MfaActive {
		return nil, model.NewAppError("InitiateMfaRecovery", "app.mfa_recovery.not_active.app_error", nil, "user_id="+user.Id, http.StatusBadRequest)
	}

	// Handle different recovery methods
	switch method {
	case "email":
		return a.initiateMfaRecoveryViaEmail(rctx, user)
	case "backup_code":
		return a.initiateMfaRecoveryViaBackupCode(rctx, user)
	default:
		return nil, model.NewAppError("InitiateMfaRecovery", "app.mfa_recovery.invalid_method.app_error", nil, "", http.StatusBadRequest)
	}
}

// initiateMfaRecoveryViaEmail sends a recovery email and sets the user in recovery state
func (a *App) initiateMfaRecoveryViaEmail(rctx request.CTX, user *model.User) (*model.MfaRecoveryResponse, *model.AppError) {
	// Generate recovery token
	token := model.NewToken(TokenTypeMfaRecovery, user.Id)
	token.Extra = user.Email

	if err := a.Srv().Store().Token().Save(token); err != nil {
		return nil, model.NewAppError("initiateMfaRecoveryViaEmail", "app.mfa_recovery.save_token.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Set user in recovery state
	// THE BUG: MfaRecoveryExpiry is NOT set here, leaving it at default value 0
	// This means the recovery state will never expire per the check in CheckUserMfa
	user.MfaRecoveryState = model.MfaRecoveryStatePending
	// MISSING: user.MfaRecoveryExpiry = model.GetMillis() + MfaRecoveryExpiryTime
	
	if _, err := a.Srv().Store().User().Update(rctx, user, true); err != nil {
		return nil, model.NewAppError("initiateMfaRecoveryViaEmail", "app.mfa_recovery.update_user.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Send recovery email
	if err := a.SendMfaRecoveryEmail(user.Email, user.Locale, token.Token); err != nil {
		rctx.Logger().Error("Failed to send MFA recovery email", mlog.Err(err))
		// Don't fail the request if email fails - user can still request another code
	}

	// Audit log the recovery request
	rctx.Logger().Info("MFA recovery initiated via email",
		mlog.String("user_id", user.Id),
		mlog.String("email", user.Email),
	)

	return &model.MfaRecoveryResponse{
		Success: true,
		RecoveryState: model.MfaRecoveryStatePending,
		EmailSent: true,
		Message: "Recovery email sent successfully.",
	}, nil
}

// initiateMfaRecoveryViaBackupCode sets up recovery using backup codes
func (a *App) initiateMfaRecoveryViaBackupCode(rctx request.CTX, user *model.User) (*model.MfaRecoveryResponse, *model.AppError) {
	// Check if user has backup codes
	if len(user.MfaBackupCodes) == 0 {
		return nil, model.NewAppError("initiateMfaRecoveryViaBackupCode", "app.mfa_recovery.no_backup_codes.app_error", nil, "", http.StatusBadRequest)
	}

	// Set user in recovery state to allow backup code usage
	user.MfaRecoveryState = model.MfaRecoveryStateBackupCode
	// THE BUG: Again, MfaRecoveryExpiry is not set
	// MISSING: user.MfaRecoveryExpiry = model.GetMillis() + MfaRecoveryExpiryTime

	if _, err := a.Srv().Store().User().Update(rctx, user, true); err != nil {
		return nil, model.NewAppError("initiateMfaRecoveryViaBackupCode", "app.mfa_recovery.update_user.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	rctx.Logger().Info("MFA recovery initiated via backup code",
		mlog.String("user_id", user.Id),
	)

	return &model.MfaRecoveryResponse{
		Success: true,
		RecoveryState: model.MfaRecoveryStateBackupCode,
		Message: "You can now use a backup code to authenticate.",
	}, nil
}

// CompleteMfaRecovery completes the recovery process and optionally resets MFA
func (a *App) CompleteMfaRecovery(rctx request.CTX, email, recoveryCode string, resetMfa bool) *model.AppError {
	// Verify recovery token
	token, err := a.Srv().Store().Token().GetByToken(recoveryCode)
	if err != nil {
		return model.NewAppError("CompleteMfaRecovery", "app.mfa_recovery.invalid_token.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}

	if token.Type != TokenTypeMfaRecovery {
		return model.NewAppError("CompleteMfaRecovery", "app.mfa_recovery.invalid_token.app_error", nil, "", http.StatusBadRequest)
	}

	if token.IsExpired() {
		a.Srv().Store().Token().Delete(token.Token)
		return model.NewAppError("CompleteMfaRecovery", "app.mfa_recovery.expired_token.app_error", nil, "", http.StatusBadRequest)
	}

	user, err := a.GetUser(token.UserId)
	if err != nil {
		return err
	}

	// Verify email matches
	if user.Email != email {
		return model.NewAppError("CompleteMfaRecovery", "app.mfa_recovery.email_mismatch.app_error", nil, "", http.StatusBadRequest)
	}

	// Update recovery state to verified
	user.MfaRecoveryState = model.MfaRecoveryStateCodeVerified
	user.MfaRecoveryExpiry = model.GetMillis() + (1000 * 60 * 30) // 30 minutes to complete

	if resetMfa {
		// Reset MFA entirely
		user.MfaActive = false
		user.MfaSecret = ""
		user.MfaUsedTimestamps = model.StringArray{}
		user.MfaRecoveryState = "" // Clear recovery state
		user.MfaRecoveryExpiry = 0

		rctx.Logger().Info("MFA reset completed",
			mlog.String("user_id", user.Id),
		)
	}

	if _, err := a.Srv().Store().User().Update(rctx, user, true); err != nil {
		return model.NewAppError("CompleteMfaRecovery", "app.mfa_recovery.update_user.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	// Delete the token
	a.Srv().Store().Token().Delete(token.Token)

	// Invalidate all sessions to force re-login
	if resetMfa {
		if err := a.RevokeAllSessions(rctx, user.Id); err != nil {
			rctx.Logger().Warn("Failed to revoke sessions after MFA reset", mlog.Err(err))
		}
	}

	return nil
}

// GenerateMfaBackupCodes generates new backup codes for a user
func (a *App) GenerateMfaBackupCodes(rctx request.CTX, userId string) (*model.MfaBackupCodesResponse, *model.AppError) {
	user, err := a.GetUser(userId)
	if err != nil {
		return nil, err
	}

	if !user.MfaActive {
		return nil, model.NewAppError("GenerateMfaBackupCodes", "app.mfa_recovery.mfa_not_active.app_error", nil, "", http.StatusBadRequest)
	}

	// Generate backup codes
	codes, genErr := model.GenerateBackupCodes(model.MfaBackupCodesCount)
	if genErr != nil {
		return nil, model.NewAppError("GenerateMfaBackupCodes", "app.mfa_recovery.generate_codes.app_error", nil, "", http.StatusInternalServerError).Wrap(genErr)
	}

	// Hash the codes before storing
	hashedCodes := make([]string, len(codes))
	for i, code := range codes {
		hashedCodes[i] = hashBackupCode(code)
	}

	user.MfaBackupCodes = model.StringArray(hashedCodes)
	user.MfaBackupCodesUsed = model.StringArray{}

	if _, updateErr := a.Srv().Store().User().Update(rctx, user, true); updateErr != nil {
		return nil, model.NewAppError("GenerateMfaBackupCodes", "app.mfa_recovery.update_user.app_error", nil, "", http.StatusInternalServerError).Wrap(updateErr)
	}

	rctx.Logger().Info("MFA backup codes generated",
		mlog.String("user_id", user.Id),
		mlog.Int("code_count", len(codes)),
	)

	return &model.MfaBackupCodesResponse{
		BackupCodes: codes,
		GeneratedAt: model.GetMillis(),
	}, nil
}

// validateMfaBackupCode validates a backup code for MFA authentication
func (a *App) validateMfaBackupCode(user *model.User, code string) bool {
	if len(user.MfaBackupCodes) == 0 {
		return false
	}

	hashedCode := hashBackupCode(code)

	// Check if code exists and hasn't been used
	for i, storedCode := range user.MfaBackupCodes {
		if storedCode == hashedCode {
			// Check if already used
			for _, usedCode := range user.MfaBackupCodesUsed {
				if usedCode == hashedCode {
					return false // Code already used
				}
			}

			// Mark code as used
			user.MfaBackupCodesUsed = append(user.MfaBackupCodesUsed, hashedCode)

			// Update user
			if _, err := a.Srv().Store().User().Update(request.EmptyContext(a.Log()), user, true); err != nil {
				a.Log().Error("Failed to update user after backup code use", mlog.Err(err))
				return false
			}

			a.Log().Info("MFA backup code used",
				mlog.String("user_id", user.Id),
				mlog.Int("code_index", i),
			)

			return true
		}
	}

	return false
}

// AdminResetUserMfa allows an admin to reset a user's MFA (admin-assisted recovery)
func (a *App) AdminResetUserMfa(rctx request.CTX, adminUserId, targetUserId, reason string) *model.AppError {
	// Verify admin has permission
	if !a.HasPermissionTo(adminUserId, model.PermissionManageSystem) {
		return model.NewAppError("AdminResetUserMfa", "app.mfa_recovery.permission_denied.app_error", nil, "", http.StatusForbidden)
	}

	targetUser, err := a.GetUser(targetUserId)
	if err != nil {
		return err
	}

	if !targetUser.MfaActive {
		return model.NewAppError("AdminResetUserMfa", "app.mfa_recovery.mfa_not_active.app_error", nil, "", http.StatusBadRequest)
	}

	// Set user in admin-assisted recovery state
	// This allows them to login without MFA temporarily
	targetUser.MfaRecoveryState = model.MfaRecoveryStateAdminAssisted
	targetUser.MfaRecoveryExpiry = model.GetMillis() + (1000 * 60 * 60 * 48) // 48 hours

	if _, updateErr := a.Srv().Store().User().Update(rctx, targetUser, true); updateErr != nil {
		return model.NewAppError("AdminResetUserMfa", "app.mfa_recovery.update_user.app_error", nil, "", http.StatusInternalServerError).Wrap(updateErr)
	}

	// Create audit log entry
	rctx.Logger().Info("Admin-assisted MFA recovery initiated",
		mlog.String("admin_user_id", adminUserId),
		mlog.String("target_user_id", targetUserId),
		mlog.String("reason", reason),
	)

	// Send notification email to user
	if err := a.SendAdminMfaResetNotificationEmail(targetUser.Email, targetUser.Locale); err != nil {
		rctx.Logger().Error("Failed to send admin MFA reset notification", mlog.Err(err))
	}

	return nil
}

// ClearMfaRecoveryState clears the recovery state for a user
// This should be called after successful MFA setup or when recovery is complete
func (a *App) ClearMfaRecoveryState(rctx request.CTX, userId string) *model.AppError {
	user, err := a.GetUser(userId)
	if err != nil {
		return err
	}

	user.MfaRecoveryState = ""
	user.MfaRecoveryExpiry = 0

	if _, updateErr := a.Srv().Store().User().Update(rctx, user, true); updateErr != nil {
		return model.NewAppError("ClearMfaRecoveryState", "app.mfa_recovery.update_user.app_error", nil, "", http.StatusInternalServerError).Wrap(updateErr)
	}

	rctx.Logger().Info("MFA recovery state cleared",
		mlog.String("user_id", user.Id),
	)

	return nil
}

// SendMfaRecoveryEmail sends a recovery email with a token
func (a *App) SendMfaRecoveryEmail(emailAddress, locale, token string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	serverURL := *a.Config().ServiceSettings.SiteURL
	recoveryLink := fmt.Sprintf("%s/mfa/recovery?token=%s", serverURL, token)

	subject := T("app.mfa_recovery.email.subject")
	bodyPage := a.newEmailTemplate("mfa_recovery_body", locale)
	bodyPage.Props["RecoveryLink"] = recoveryLink
	bodyPage.Props["SiteURL"] = serverURL
	bodyPage.Props["ExpiryHours"] = "24"

	if err := a.Srv().EmailService.SendMail(emailAddress, subject, bodyPage.Render()); err != nil {
		return model.NewAppError("SendMfaRecoveryEmail", "app.mfa_recovery.send_email.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

// SendAdminMfaResetNotificationEmail notifies a user that an admin reset their MFA
func (a *App) SendAdminMfaResetNotificationEmail(emailAddress, locale string) *model.AppError {
	T := i18n.GetUserTranslations(locale)

	subject := T("app.mfa_recovery.admin_reset.subject")
	bodyPage := a.newEmailTemplate("admin_mfa_reset_notification", locale)
	bodyPage.Props["SiteURL"] = *a.Config().ServiceSettings.SiteURL
	bodyPage.Props["ResetTime"] = time.Now().Format(time.RFC1123)

	if err := a.Srv().EmailService.SendMail(emailAddress, subject, bodyPage.Render()); err != nil {
		return model.NewAppError("SendAdminMfaResetNotificationEmail", "app.mfa_recovery.send_email.app_error", nil, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

// hashBackupCode creates a SHA-256 hash of a backup code for secure storage
func hashBackupCode(code string) string {
	// Normalize the code (remove spaces, lowercase)
	normalized := strings.ToLower(strings.ReplaceAll(code, " ", ""))
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}

