// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
)

// MfaRecoveryRequest represents a request to initiate MFA recovery
type MfaRecoveryRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username"`
	RecoveryMethod string `json:"recovery_method"` // "email", "backup_code", "admin"
}

// MfaRecoveryVerifyRequest represents a request to verify recovery code
type MfaRecoveryVerifyRequest struct {
	Email        string `json:"email"`
	RecoveryCode string `json:"recovery_code"`
}

// MfaRecoveryCompleteRequest represents a request to complete MFA recovery
type MfaRecoveryCompleteRequest struct {
	Email           string `json:"email"`
	RecoveryCode    string `json:"recovery_code"`
	NewMfaSecret    string `json:"new_mfa_secret,omitempty"`
	ResetMfa        bool   `json:"reset_mfa"` // If true, disable MFA entirely
}

// MfaBackupCodeRequest represents a request to use a backup code
type MfaBackupCodeRequest struct {
	UserId     string `json:"user_id"`
	BackupCode string `json:"backup_code"`
}

// MfaBackupCodesResponse contains generated backup codes
type MfaBackupCodesResponse struct {
	BackupCodes []string `json:"backup_codes"`
	GeneratedAt int64    `json:"generated_at"`
}

// MfaAdminRecoveryRequest represents an admin-initiated MFA reset
type MfaAdminRecoveryRequest struct {
	UserId string `json:"user_id"`
	Reason string `json:"reason"`
}

// MfaRecoveryResponse represents the response to a recovery request
type MfaRecoveryResponse struct {
	Success         bool   `json:"success"`
	RecoveryState   string `json:"recovery_state"`
	RecoveryExpiry  int64  `json:"recovery_expiry,omitempty"`
	EmailSent       bool   `json:"email_sent"`
	Message         string `json:"message"`
}

func (r *MfaRecoveryRequest) IsValid() *AppError {
	if r.Email == "" && r.Username == "" {
		return NewAppError("MfaRecoveryRequest.IsValid", "model.mfa_recovery.is_valid.email_or_username.app_error", nil, "", http.StatusBadRequest)
	}

	if r.RecoveryMethod == "" {
		r.RecoveryMethod = "email" // Default to email recovery
	}

	validMethods := []string{"email", "backup_code", "admin"}
	isValid := false
	for _, method := range validMethods {
		if r.RecoveryMethod == method {
			isValid = true
			break
		}
	}
	if !isValid {
		return NewAppError("MfaRecoveryRequest.IsValid", "model.mfa_recovery.is_valid.method.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}

func (r *MfaRecoveryVerifyRequest) IsValid() *AppError {
	if r.Email == "" {
		return NewAppError("MfaRecoveryVerifyRequest.IsValid", "model.mfa_recovery.is_valid.email.app_error", nil, "", http.StatusBadRequest)
	}
	if r.RecoveryCode == "" {
		return NewAppError("MfaRecoveryVerifyRequest.IsValid", "model.mfa_recovery.is_valid.code.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

func (r *MfaBackupCodeRequest) IsValid() *AppError {
	if r.UserId == "" {
		return NewAppError("MfaBackupCodeRequest.IsValid", "model.mfa_recovery.is_valid.user_id.app_error", nil, "", http.StatusBadRequest)
	}
	if r.BackupCode == "" {
		return NewAppError("MfaBackupCodeRequest.IsValid", "model.mfa_recovery.is_valid.backup_code.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

func (r *MfaRecoveryRequest) ToJSON() []byte {
	b, _ := json.Marshal(r)
	return b
}

func (r *MfaRecoveryVerifyRequest) ToJSON() []byte {
	b, _ := json.Marshal(r)
	return b
}

func (r *MfaBackupCodesResponse) ToJSON() []byte {
	b, _ := json.Marshal(r)
	return b
}

func (r *MfaRecoveryResponse) ToJSON() []byte {
	b, _ := json.Marshal(r)
	return b
}

func MfaRecoveryRequestFromJSON(data io.Reader) *MfaRecoveryRequest {
	var r MfaRecoveryRequest
	if err := json.NewDecoder(data).Decode(&r); err != nil {
		return nil
	}
	return &r
}

func MfaRecoveryVerifyRequestFromJSON(data io.Reader) *MfaRecoveryVerifyRequest {
	var r MfaRecoveryVerifyRequest
	if err := json.NewDecoder(data).Decode(&r); err != nil {
		return nil
	}
	return &r
}

func MfaBackupCodeRequestFromJSON(data io.Reader) *MfaBackupCodeRequest {
	var r MfaBackupCodeRequest
	if err := json.NewDecoder(data).Decode(&r); err != nil {
		return nil
	}
	return &r
}

func MfaAdminRecoveryRequestFromJSON(data io.Reader) *MfaAdminRecoveryRequest {
	var r MfaAdminRecoveryRequest
	if err := json.NewDecoder(data).Decode(&r); err != nil {
		return nil
	}
	return &r
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateBackupCodes generates a set of backup codes for MFA recovery
func GenerateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	for i := 0; i < count; i++ {
		// Generate 8-byte random values for each backup code
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		// Format as hex string with dashes for readability
		codes[i] = base64.RawStdEncoding.EncodeToString(b)
	}
	return codes, nil
}

