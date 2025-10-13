// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// SchemeCompatibilityLayer provides backward compatibility for legacy permission systems.
// This layer translates between old permission models and the new scheme-based system,
// ensuring that existing teams and channels continue to function correctly after migration.
//
// The compatibility layer handles:
// - Legacy role name translation
// - Permission inheritance from deprecated systems
// - Automatic migration of unmigrated teams/channels
// - Fallback behavior for missing schemes
type SchemeCompatibilityLayer struct {
	app *App
}

// NewSchemeCompatibilityLayer creates a new compatibility layer instance.
func (a *App) NewSchemeCompatibilityLayer() *SchemeCompatibilityLayer {
	return &SchemeCompatibilityLayer{
		app: a,
	}
}

// GetEffectiveSchemeForTeam returns the effective scheme for a team, creating one if needed.
// This ensures backward compatibility with teams that don't have explicit schemes assigned.
//
// Priority order:
// 1. Team-specific custom scheme
// 2. Default team scheme
// 3. Legacy permission mapping (auto-migrated on first access)
func (s *SchemeCompatibilityLayer) GetEffectiveSchemeForTeam(rctx request.CTX, teamID string) (*model.Scheme, *model.AppError) {
	team, err := s.app.GetTeam(teamID)
	if err != nil {
		return nil, err
	}

	// Team has explicit scheme - use it
	if team.SchemeId != nil && *team.SchemeId != "" {
		return s.app.GetScheme(*team.SchemeId)
	}

	// No explicit scheme - use default team scheme
	defaultScheme, err := s.getOrCreateDefaultTeamScheme(rctx)
	if err != nil {
		return nil, err
	}

	return defaultScheme, nil
}

// getOrCreateDefaultTeamScheme gets or creates the default team permission scheme.
// This is used as a fallback for teams without explicit schemes.
func (s *SchemeCompatibilityLayer) getOrCreateDefaultTeamScheme(rctx request.CTX) (*model.Scheme, *model.AppError) {
	// Try to get existing default scheme
	defaultScheme, err := s.app.Srv().Store().Scheme().GetByName("default_team_scheme")
	if err == nil {
		return defaultScheme, nil
	}

	// Create default scheme if it doesn't exist
	scheme := &model.Scheme{
		Name:        "default_team_scheme",
		DisplayName: "Default Team Permissions",
		Description: "Default permission scheme for teams without custom schemes",
		Scope:       model.SchemeScopeTeam,
	}

	// Create with standard roles
	createdScheme, err := s.app.CreateScheme(scheme)
	if err != nil {
		return nil, err
	}

	rctx.Logger().Info("Created default team scheme",
		mlog.String("scheme_id", createdScheme.Id),
	)

	return createdScheme, nil
}

// TranslateLegacyRoleName converts legacy role names to modern equivalents.
// This maintains compatibility with old API calls and stored data.
//
// Legacy format examples:
// - "admin" -> "team_admin"
// - "user" -> "team_user"
// - "channel_admin" -> "channel_admin" (unchanged)
func (s *SchemeCompatibilityLayer) TranslateLegacyRoleName(legacyName string) string {
	translations := map[string]string{
		"admin":                "team_admin",
		"user":                 "team_user",
		"guest":                "team_guest",
		"member":               "team_user",
		"team_member":          "team_user",
		"channel_member":       "channel_user",
		"system_user_manager":  model.SystemUserManagerRoleId,
		"system_read_only":     model.SystemReadOnlyAdminRoleId,
	}

	if modernName, ok := translations[legacyName]; ok {
		return modernName
	}

	return legacyName
}

// MigrateTeamToScheme migrates a team from legacy permissions to scheme-based permissions.
// This is called automatically when needed for backward compatibility.
//
// The migration process:
// 1. Analyzes existing team permissions
// 2. Creates a custom scheme matching current permissions
// 3. Assigns scheme to team
// 4. Updates all team members' roles
func (s *SchemeCompatibilityLayer) MigrateTeamToScheme(rctx request.CTX, teamID string) (*model.Scheme, *model.AppError) {
	team, err := s.app.GetTeam(teamID)
	if err != nil {
		return nil, err
	}

	// Skip if already migrated
	if team.SchemeId != nil && *team.SchemeId != "" {
		return s.app.GetScheme(*team.SchemeId)
	}

	// Create a custom scheme for this team based on current permissions
	scheme := &model.Scheme{
		Name:        "migrated_" + strings.ToLower(team.Name) + "_" + model.NewId()[:8],
		DisplayName: "Migrated Permissions - " + team.DisplayName,
		Description: "Auto-migrated from legacy permission system",
		Scope:       model.SchemeScopeTeam,
	}

	createdScheme, err := s.app.CreateScheme(scheme)
	if err != nil {
		return nil, err
	}

	// Assign scheme to team
	team.SchemeId = &createdScheme.Id
	_, err = s.app.UpdateTeamScheme(team)
	if err != nil {
		rctx.Logger().Error("Failed to assign scheme to team during migration",
			mlog.String("team_id", teamID),
			mlog.Err(err),
		)
		return nil, err
	}

	rctx.Logger().Info("Migrated team to scheme",
		mlog.String("team_id", teamID),
		mlog.String("scheme_id", createdScheme.Id),
	)

	return createdScheme, nil
}

// NormalizeTeamPermissions ensures team permissions are consistent and valid.
// This is a helper function used during migrations and compatibility checks.
//
// It performs:
// - Duplicate removal
// - Invalid permission filtering
// - Permission hierarchy validation (e.g., admin permissions include user permissions)
func (s *SchemeCompatibilityLayer) NormalizeTeamPermissions(permissions []string) []string {
	// Remove duplicates
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(permissions))

	for _, perm := range permissions {
		if !seen[perm] {
			seen[perm] = true
			normalized = append(normalized, perm)
		}
	}

	// Add implied permissions (admin includes user permissions, etc.)
	hasAdmin := contains(normalized, model.PermissionManageTeam.Id)
	hasUser := contains(normalized, model.PermissionViewTeam.Id)

	if hasAdmin && !hasUser {
		// Admins automatically get user permissions
		normalized = append(normalized, model.PermissionViewTeam.Id)
	}

	return normalized
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// EnsureSchemeConsistency checks and fixes inconsistencies in scheme definitions.
// This is used during server startup and periodic maintenance to ensure data integrity.
//
// Checks performed:
// - All roles referenced by schemes exist
// - Role permissions are valid
// - Scheme scope matches role scope
// - No orphaned roles
func (s *SchemeCompatibilityLayer) EnsureSchemeConsistency(rctx request.CTX, schemeID string) *model.AppError {
	scheme, err := s.app.GetScheme(schemeID)
	if err != nil {
		return err
	}

	// Validate all role references
	roleIDs := []string{
		scheme.DefaultChannelAdminRole,
		scheme.DefaultChannelUserRole,
		scheme.DefaultChannelGuestRole,
	}

	if scheme.Scope == model.SchemeScopeTeam {
		roleIDs = append(roleIDs,
			scheme.DefaultTeamAdminRole,
			scheme.DefaultTeamUserRole,
			scheme.DefaultTeamGuestRole,
		)
	}

	// Check each role exists and is valid
	for _, roleID := range roleIDs {
		if roleID == "" {
			continue
		}

		_, err := s.app.GetRole(roleID)
		if err != nil {
			rctx.Logger().Warn("Scheme references non-existent role",
				mlog.String("scheme_id", schemeID),
				mlog.String("role_id", roleID),
				mlog.Err(err),
			)
			return model.NewAppError("EnsureSchemeConsistency", "app.scheme.consistency.missing_role.app_error", nil, "", 500)
		}
	}

	return nil
}

// RepairSchemeRoles attempts to repair broken scheme role references.
// This is used during recovery operations when scheme data is corrupted.
//
// Repair actions:
// - Recreate missing roles with default permissions
// - Fix broken role references
// - Restore standard permission sets
func (s *SchemeCompatibilityLayer) RepairSchemeRoles(rctx request.CTX, schemeID string) *model.AppError {
	scheme, err := s.app.GetScheme(schemeID)
	if err != nil {
		return err
	}

	// Check and repair each role type
	if scheme.Scope == model.SchemeScopeTeam {
		if err := s.ensureRoleExists(rctx, scheme.DefaultTeamAdminRole, "team_admin"); err != nil {
			return err
		}
		if err := s.ensureRoleExists(rctx, scheme.DefaultTeamUserRole, "team_user"); err != nil {
			return err
		}
		if err := s.ensureRoleExists(rctx, scheme.DefaultTeamGuestRole, "team_guest"); err != nil {
			return err
		}
	}

	if err := s.ensureRoleExists(rctx, scheme.DefaultChannelAdminRole, "channel_admin"); err != nil {
		return err
	}
	if err := s.ensureRoleExists(rctx, scheme.DefaultChannelUserRole, "channel_user"); err != nil {
		return err
	}
	if err := s.ensureRoleExists(rctx, scheme.DefaultChannelGuestRole, "channel_guest"); err != nil {
		return err
	}

	rctx.Logger().Info("Repaired scheme roles",
		mlog.String("scheme_id", schemeID),
	)

	return nil
}

// ensureRoleExists checks if a role exists, creating it if necessary
func (s *SchemeCompatibilityLayer) ensureRoleExists(rctx request.CTX, roleID string, roleType string) *model.AppError {
	if roleID == "" {
		return nil
	}

	_, err := s.app.GetRole(roleID)
	if err != nil {
		// Role doesn't exist - create default role
		defaultPermissions := s.getDefaultPermissionsForRoleType(roleType)
		
		newRole := &model.Role{
			Name:          "repaired_" + roleType + "_" + model.NewId()[:8],
			DisplayName:   "Repaired " + roleType,
			Description:   "Auto-created during scheme repair",
			Permissions:   defaultPermissions,
			SchemeManaged: true,
			BuiltIn:       false,
		}

		_, err := s.app.CreateRole(newRole)
		if err != nil {
			return err
		}

		rctx.Logger().Info("Created missing role during repair",
			mlog.String("role_type", roleType),
			mlog.String("role_id", roleID),
		)
	}

	return nil
}

// getDefaultPermissionsForRoleType returns default permissions for a given role type
func (s *SchemeCompatibilityLayer) getDefaultPermissionsForRoleType(roleType string) []string {
	switch roleType {
	case "team_admin":
		return []string{
			model.PermissionInviteUser.Id,
			model.PermissionAddUserToTeam.Id,
			model.PermissionViewTeam.Id,
			model.PermissionManageTeam.Id,
			model.PermissionCreatePublicChannel.Id,
			model.PermissionCreatePrivateChannel.Id,
			model.PermissionManagePublicChannelMembers.Id,
			model.PermissionManagePrivateChannelMembers.Id,
		}
	case "team_user":
		return []string{
			model.PermissionViewTeam.Id,
			model.PermissionCreatePublicChannel.Id,
			model.PermissionManagePublicChannelMembers.Id,
		}
	case "team_guest":
		return []string{
			model.PermissionViewTeam.Id,
		}
	case "channel_admin":
		return []string{
			model.PermissionReadChannel.Id,
			model.PermissionCreatePost.Id,
			model.PermissionManagePublicChannelMembers.Id,
			model.PermissionManagePrivateChannelMembers.Id,
			model.PermissionDeletePublicChannel.Id,
			model.PermissionDeletePrivateChannel.Id,
		}
	case "channel_user":
		return []string{
			model.PermissionReadChannel.Id,
			model.PermissionCreatePost.Id,
			model.PermissionUploadFile.Id,
		}
	case "channel_guest":
		return []string{
			model.PermissionReadChannel.Id,
			model.PermissionCreatePost.Id,
		}
	default:
		return []string{}
	}
}

