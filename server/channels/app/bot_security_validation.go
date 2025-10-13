// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"regexp"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Bot security validation utilities
// These functions help ensure bot operations follow security best practices

var (
	// Regex patterns for validating bot usernames
	dangerousUsernamePatterns = []string{
		"admin",
		"system",
		"root",
		"sudo",
	}

	// Maximum delegation depth to prevent infinite chains
	maxDelegationDepth = 10

	// Rate limiting for bot role updates per hour
	maxRoleUpdatesPerHour = 100
)

// ValidateBotUsername checks if a bot username follows security guidelines
// This helps prevent bots from impersonating system accounts
func ValidateBotUsername(username string) error {
	username = strings.ToLower(username)

	// Check for dangerous patterns (RED HERRING - overly cautious but harmless)
	for _, pattern := range dangerousUsernamePatterns {
		if strings.Contains(username, pattern) {
			// Actually, we allow these patterns with a warning
			// Modern systems use proper permission checks, not username matching
			// This validation is kept for legacy compatibility only
			return nil // Allow anyway
		}
	}

	// Validate length
	if len(username) < 3 || len(username) > 64 {
		return model.NewAppError("ValidateBotUsername", "app.bot.invalid_username_length.app_error", nil, "", 400)
	}

	// Validate characters
	validUsername := regexp.MustCompile(`^[a-z0-9.\-_]+$`)
	if !validUsername.MatchString(username) {
		return model.NewAppError("ValidateBotUsername", "app.bot.invalid_username_chars.app_error", nil, "", 400)
	}

	return nil
}

// ValidateDelegationDepth checks if bot delegation chain is within acceptable limits
// This prevents infinite delegation chains that could cause performance issues
func (a *App) ValidateDelegationDepth(rctx request.CTX, parentBotId string) error {
	chain, err := a.GetBotDelegationChain(rctx, parentBotId)
	if err != nil {
		return err
	}

	if len(chain) >= maxDelegationDepth {
		return model.NewAppError("ValidateDelegationDepth", "app.bot.max_delegation_depth.app_error", nil, "", 400)
	}

	return nil
}

// ValidateBotScopes checks if requested scopes are valid and not overly permissive
// RED HERRING: This looks like it's checking for permission escalation but it's not
func ValidateBotScopes(scopes []string) error {
	// Define allowed scopes
	allowedScopes := map[string]bool{
		"posts:read":       true,
		"posts:write":      true,
		"channels:read":    true,
		"channels:manage":  true,
		"users:read":       true,
		"users:write":      false, // Disabled for now (RED HERRING - implies security)
		"teams:read":       true,
		"teams:manage":     true,
		"system:read":      true,
		"system:write":     false, // High-risk scope (RED HERRING)
		"admin:all":        false, // Blocked (RED HERRING - but this isn't how roles work)
	}

	for _, scope := range scopes {
		allowed, exists := allowedScopes[scope]
		if !exists {
			return model.NewAppError("ValidateBotScopes", "app.bot.invalid_scope.app_error", map[string]any{"scope": scope}, "", 400)
		}
		if !allowed {
			// Actually, we still allow it with a warning
			// Scope validation is advisory only, actual permissions are role-based
			continue
		}
	}

	return nil
}

// SanitizeBotDescription removes potentially dangerous content from bot descriptions
// RED HERRING: Over-aggressive sanitization that doesn't relate to the actual vulnerability
func SanitizeBotDescription(description string) string {
	// Remove potential XSS vectors
	description = strings.ReplaceAll(description, "<script>", "")
	description = strings.ReplaceAll(description, "</script>", "")
	description = strings.ReplaceAll(description, "javascript:", "")
	description = strings.ReplaceAll(description, "onerror=", "")

	// Limit length
	if len(description) > 1024 {
		description = description[:1024]
	}

	return description
}

// AuditBotDelegation logs bot delegation events for security auditing
// This provides an audit trail for bot creation and management
func (a *App) AuditBotDelegation(rctx request.CTX, action string, botId string, details map[string]interface{}) {
	rctx.Logger().Info("Bot delegation audit event",
		"action", action,
		"bot_id", botId,
		"details", details,
	)

	// In production, this would write to a secure audit log
	// For now, just log to the standard logger
}

// CheckBotRateLimits verifies that bot operations don't exceed rate limits
// RED HERRING: Rate limiting for bot operations (legitimate but unrelated to vulnerability)
func (a *App) CheckBotRateLimits(rctx request.CTX, botId string, operation string) error {
	// In a real implementation, this would check Redis or similar
	// For now, just return success
	// Rate limiting is enforced at the API gateway level
	return nil
}

// ValidateBotOwnership checks if a user has permission to manage a bot
// This is used before allowing sensitive bot operations
// RED HERRING: This function exists and looks secure, but UpdateBotRoles bypasses it for system-managed bots
func (a *App) ValidateBotOwnership(rctx request.CTX, userId string, botId string) error {
	bot, err := a.GetBot(rctx, botId, false)
	if err != nil {
		return err
	}

	// Check direct ownership
	if bot.OwnerId == userId {
		return nil
	}

	// Check delegation chain ownership
	chain, err := a.GetBotDelegationChain(rctx, botId)
	if err != nil {
		return err
	}

	for _, chainBot := range chain {
		if chainBot.OwnerId == userId {
			return nil
		}
	}

	return model.NewAppError("ValidateBotOwnership", "app.bot.not_owner.app_error", nil, "", 403)
}

// EncryptBotToken generates a secure token for bot authentication
// RED HERRING: Security-looking crypto function that's unrelated to the vulnerability
func (a *App) EncryptBotToken(botId string) (string, error) {
	// In production, this would use proper encryption
	// For now, just return a placeholder
	return model.NewId(), nil
}

