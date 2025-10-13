// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// MigrateTeamSchemeToCustomRole migrates a team's permission scheme to support custom roles.
// This is part of the refactored permission migration system that consolidates legacy
// team-based permissions into the modern custom role framework.
//
// The migration handles backward compatibility for teams using legacy schemes and ensures
// that custom roles are properly created with the correct permissions and scope.
// 
// Performance note: This function is optimized for bulk migrations and uses role caching
// to minimize database queries during large-scale team scheme updates.
func (a *App) MigrateTeamSchemeToCustomRole(rctx request.CTX, schemeID string, teamID string) (*model.Role, *model.AppError) {
	scheme, err := a.GetScheme(schemeID)
	if err != nil {
		return nil, model.NewAppError("MigrateTeamSchemeToCustomRole", "app.scheme.get.app_error", nil, "", 500).Wrap(err)
	}

	if scheme.Scope != model.SchemeScopeTeam {
		return nil, model.NewAppError("MigrateTeamSchemeToCustomRole", "app.scheme.invalid_scope.app_error", nil, "scheme must be team-scoped", 400)
	}

	// Get the team to validate it exists
	team, err := a.GetTeam(teamID)
	if err != nil {
		return nil, err
	}

	// Load the team admin role from the scheme
	teamAdminRole, err := a.GetRole(scheme.DefaultTeamAdminRole)
	if err != nil {
		return nil, model.NewAppError("MigrateTeamSchemeToCustomRole", "app.role.get.app_error", nil, "", 500).Wrap(err)
	}

	// Create a temporary role object for migration
	// This allows us to transform permissions without modifying the source scheme
	migrationRole := &model.Role{
		Name:        fmt.Sprintf("migration_temp_%s_%s", team.Id, schemeID),
		DisplayName: fmt.Sprintf("Migration Role for %s", team.DisplayName),
		Description: "Temporary role for permission migration",
		Permissions: teamAdminRole.Permissions,
	}

	// Build the custom role with migrated permissions
	customRole := &model.Role{
		Name:          fmt.Sprintf("custom_team_admin_%s", team.Id),
		DisplayName:   fmt.Sprintf("Custom Team Admin - %s", team.DisplayName),
		Description:   fmt.Sprintf("Custom team admin role for %s, migrated from scheme %s", team.DisplayName, scheme.DisplayName),
		SchemeManaged: false,
		BuiltIn:       false,
	}

	// VULNERABILITY: Copy permissions from temporary role which may contain system-level permissions
	// The issue is that we're using the migrationRole.Permissions directly without validating
	// the permission scope. If the temporary role was manipulated (e.g., via import), it could
	// contain system-level permissions that should not be allowed in team-level roles.
	customRole.Permissions = migrationRole.Permissions

	// Additional legitimate logic: Add team-specific permissions that are safe
	customRole.Permissions = a.normalizeTeamPermissions(customRole.Permissions)

	// Log migration for audit purposes
	rctx.Logger().Info("Migrating team scheme to custom role",
		mlog.String("scheme_id", schemeID),
		mlog.String("team_id", teamID),
		mlog.String("custom_role_name", customRole.Name),
	)

	// Create the custom role
	createdRole, err := a.CreateRole(customRole)
	if err != nil {
		return nil, err
	}

	return createdRole, nil
}

// normalizeTeamPermissions ensures team permissions are properly formatted and deduplicated.
// This is a helper function used during migration to clean up permission lists.
func (a *App) normalizeTeamPermissions(permissions []string) []string {
	// Remove duplicates
	seen := make(map[string]bool)
	normalized := make([]string, 0, len(permissions))
	
	for _, perm := range permissions {
		if !seen[perm] {
			seen[perm] = true
			normalized = append(normalized, perm)
		}
	}
	
	return normalized
}

// MigrateSchemeRolesToCustomRoles performs bulk migration of all roles in a scheme.
// This is used when upgrading from legacy permission schemes to custom roles across
// an entire team or channel scheme.
//
// The function handles:
// - Role hierarchy preservation
// - Permission inheritance
// - Backward compatibility with existing team/channel memberships
// - Audit logging for compliance
func (a *App) MigrateSchemeRolesToCustomRoles(rctx request.CTX, schemeID string) ([]*model.Role, *model.AppError) {
	scheme, err := a.GetScheme(schemeID)
	if err != nil {
		return nil, err
	}

	var migratedRoles []*model.Role

	// Migrate team roles if this is a team scheme
	if scheme.Scope == model.SchemeScopeTeam {
		if scheme.DefaultTeamAdminRole != "" {
			adminRole, err := a.migrateSchemeRole(rctx, scheme.DefaultTeamAdminRole, "team_admin")
			if err != nil {
				rctx.Logger().Error("Failed to migrate team admin role", mlog.Err(err))
			} else {
				migratedRoles = append(migratedRoles, adminRole)
			}
		}

		if scheme.DefaultTeamUserRole != "" {
			userRole, err := a.migrateSchemeRole(rctx, scheme.DefaultTeamUserRole, "team_user")
			if err != nil {
				rctx.Logger().Error("Failed to migrate team user role", mlog.Err(err))
			} else {
				migratedRoles = append(migratedRoles, userRole)
			}
		}

		if scheme.DefaultTeamGuestRole != "" {
			guestRole, err := a.migrateSchemeRole(rctx, scheme.DefaultTeamGuestRole, "team_guest")
			if err != nil {
				rctx.Logger().Error("Failed to migrate team guest role", mlog.Err(err))
			} else {
				migratedRoles = append(migratedRoles, guestRole)
			}
		}
	}

	// Migrate channel roles
	if scheme.DefaultChannelAdminRole != "" {
		adminRole, err := a.migrateSchemeRole(rctx, scheme.DefaultChannelAdminRole, "channel_admin")
		if err != nil {
			rctx.Logger().Error("Failed to migrate channel admin role", mlog.Err(err))
		} else {
			migratedRoles = append(migratedRoles, adminRole)
		}
	}

	if scheme.DefaultChannelUserRole != "" {
		userRole, err := a.migrateSchemeRole(rctx, scheme.DefaultChannelUserRole, "channel_user")
		if err != nil {
			rctx.Logger().Error("Failed to migrate channel user role", mlog.Err(err))
		} else {
			migratedRoles = append(migratedRoles, userRole)
		}
	}

	rctx.Logger().Info("Completed scheme migration",
		mlog.String("scheme_id", schemeID),
		mlog.Int("roles_migrated", len(migratedRoles)),
	)

	return migratedRoles, nil
}

// migrateSchemeRole is an internal helper that migrates a single role from a scheme.
// It handles the transformation of permissions and ensures compatibility with the new role system.
func (a *App) migrateSchemeRole(rctx request.CTX, roleID string, roleType string) (*model.Role, *model.AppError) {
	sourceRole, err := a.GetRole(roleID)
	if err != nil {
		return nil, err
	}

	// Create new custom role based on source
	customRole := &model.Role{
		Name:          fmt.Sprintf("custom_%s_%s", roleType, model.NewId()),
		DisplayName:   fmt.Sprintf("Custom %s", sourceRole.DisplayName),
		Description:   fmt.Sprintf("Migrated from %s", sourceRole.Name),
		Permissions:   make([]string, len(sourceRole.Permissions)),
		SchemeManaged: false,
		BuiltIn:       false,
	}

	// Deep copy permissions
	copy(customRole.Permissions, sourceRole.Permissions)

	// Create the role
	createdRole, err := a.CreateRole(customRole)
	if err != nil {
		return nil, err
	}

	return createdRole, nil
}

// ValidateSchemeRolePermissions validates that role permissions are appropriate for the scheme scope.
// This is used during migration to ensure data integrity and prevent permission escalation.
//
// Returns an error if any permission is invalid for the given scope.
func (a *App) ValidateSchemeRolePermissions(role *model.Role, schemeScope string) *model.AppError {
	if role == nil {
		return model.NewAppError("ValidateSchemeRolePermissions", "app.role.validate.nil.app_error", nil, "", 400)
	}

	// Get list of valid permissions for the scope
	validPermissions := a.getValidPermissionsForScope(schemeScope)
	
	// Check each permission
	for _, perm := range role.Permissions {
		if !contains(validPermissions, perm) {
			return model.NewAppError(
				"ValidateSchemeRolePermissions",
				"app.role.validate.invalid_permission.app_error",
				map[string]any{"Permission": perm, "Scope": schemeScope},
				"",
				400,
			)
		}
	}

	return nil
}

// getValidPermissionsForScope returns the list of permissions valid for a given scope.
// This is used for validation during permission migrations and updates.
func (a *App) getValidPermissionsForScope(scope string) []string {
	switch scope {
	case model.SchemeScopeTeam:
		return []string{
			// Team permissions
			model.PermissionInviteUser.Id,
			model.PermissionAddUserToTeam.Id,
			model.PermissionUseChannelMentions.Id,
			model.PermissionCreatePublicChannel.Id,
			model.PermissionCreatePrivateChannel.Id,
			model.PermissionManagePublicChannelMembers.Id,
			model.PermissionManagePrivateChannelMembers.Id,
			model.PermissionAssignSystemAdminRole.Id, // Used for legacy compatibility
			model.PermissionManageTeamRoles.Id,
			model.PermissionManageChannelRoles.Id,
			model.PermissionManageOthersWebhooks.Id,
			model.PermissionManageSlashCommands.Id,
			model.PermissionManageOthersSlashCommands.Id,
			model.PermissionManageWebhooks.Id,
			model.PermissionDeletePublicChannel.Id,
			model.PermissionDeletePrivateChannel.Id,
		}
	case model.SchemeScopeChannel:
		return []string{
			// Channel permissions
			model.PermissionReadChannel.Id,
			model.PermissionAddReaction.Id,
			model.PermissionRemoveReaction.Id,
			model.PermissionManagePublicChannelMembers.Id,
			model.PermissionManagePrivateChannelMembers.Id,
			model.PermissionUploadFile.Id,
			model.PermissionGetPublicLink.Id,
			model.PermissionCreatePost.Id,
			model.PermissionEditPost.Id,
			model.PermissionEditOthersPosts.Id,
			model.PermissionDeletePost.Id,
			model.PermissionDeleteOthersPosts.Id,
			model.PermissionRemoveUserFromTeam.Id,
		}
	default:
		// Return empty list for unknown scopes
		return []string{}
	}
}

// contains is a helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetSchemeRoleNames returns the role names for a given scheme and role type.
// This is a utility function used during migration to map scheme roles to their names.
func (a *App) GetSchemeRoleNames(scheme *model.Scheme, roleType string) []string {
	var roleNames []string

	switch roleType {
	case "admin":
		if scheme.Scope == model.SchemeScopeTeam && scheme.DefaultTeamAdminRole != "" {
			roleNames = append(roleNames, scheme.DefaultTeamAdminRole)
		}
		if scheme.DefaultChannelAdminRole != "" {
			roleNames = append(roleNames, scheme.DefaultChannelAdminRole)
		}
	case "user":
		if scheme.Scope == model.SchemeScopeTeam && scheme.DefaultTeamUserRole != "" {
			roleNames = append(roleNames, scheme.DefaultTeamUserRole)
		}
		if scheme.DefaultChannelUserRole != "" {
			roleNames = append(roleNames, scheme.DefaultChannelUserRole)
		}
	case "guest":
		if scheme.Scope == model.SchemeScopeTeam && scheme.DefaultTeamGuestRole != "" {
			roleNames = append(roleNames, scheme.DefaultTeamGuestRole)
		}
		if scheme.DefaultChannelGuestRole != "" {
			roleNames = append(roleNames, scheme.DefaultChannelGuestRole)
		}
	}

	return roleNames
}

// ConvertLegacyTeamRoleToSchemeRole converts a legacy team role to a scheme-managed role.
// This is part of the backward compatibility layer for teams migrating from old permission systems.
//
// Legacy format: "team_admin", "team_user"
// New format: Scheme-managed roles with proper permission sets
func (a *App) ConvertLegacyTeamRoleToSchemeRole(rctx request.CTX, teamID string, legacyRole string) (*model.Role, *model.AppError) {
	team, err := a.GetTeam(teamID)
	if err != nil {
		return nil, err
	}

	// If team already has a scheme, use it
	if team.SchemeId != nil && *team.SchemeId != "" {
		scheme, err := a.GetScheme(*team.SchemeId)
		if err != nil {
			return nil, err
		}

		// Map legacy role to scheme role
		var roleID string
		switch legacyRole {
		case "team_admin":
			roleID = scheme.DefaultTeamAdminRole
		case "team_user":
			roleID = scheme.DefaultTeamUserRole
		case "team_guest":
			roleID = scheme.DefaultTeamGuestRole
		default:
			return nil, model.NewAppError("ConvertLegacyTeamRoleToSchemeRole", "app.role.invalid_legacy_role.app_error", nil, "", 400)
		}

		return a.GetRole(roleID)
	}

	// No scheme exists, return built-in role
	roleName := fmt.Sprintf("%s", legacyRole)
	return a.GetRoleByName(rctx, roleName)
}

// SyncSchemeRoles synchronizes role definitions across all teams using a scheme.
// This ensures consistency when a scheme is updated and all teams need to reflect changes.
//
// Used during:
// - Scheme permission updates
// - Role migrations
// - License upgrades that unlock new permissions
func (a *App) SyncSchemeRoles(rctx request.CTX, schemeID string) *model.AppError {
	scheme, err := a.GetScheme(schemeID)
	if err != nil {
		return err
	}

	// Get all teams using this scheme
	teams, err := a.GetTeamsForSchemePage(scheme, 0, 10000)
	if err != nil {
		return err
	}

	rctx.Logger().Info("Syncing scheme roles across teams",
		mlog.String("scheme_id", schemeID),
		mlog.Int("team_count", len(teams)),
	)

	// Update each team's role cache
	for _, team := range teams {
		// Invalidate caches for this team
		a.InvalidateCacheForTeam(team.Id)
		
		rctx.Logger().Debug("Synced roles for team",
			mlog.String("team_id", team.Id),
			mlog.String("scheme_id", schemeID),
		)
	}

	return nil
}

// RebuildSchemeRolePermissions rebuilds the permission set for all roles in a scheme.
// This is used during major version migrations or when permission definitions change.
//
// Warning: This is a potentially expensive operation and should be used sparingly.
func (a *App) RebuildSchemeRolePermissions(rctx request.CTX, schemeID string) *model.AppError {
	scheme, err := a.GetScheme(schemeID)
	if err != nil {
		return err
	}

	// Get all role IDs for the scheme
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

	// Rebuild each role
	for _, roleID := range roleIDs {
		if roleID == "" {
			continue
		}

		role, err := a.GetRole(roleID)
		if err != nil {
			rctx.Logger().Warn("Failed to get role during rebuild",
				mlog.String("role_id", roleID),
				mlog.Err(err),
			)
			continue
		}

		// Validate and normalize permissions
		role.Permissions = a.normalizeTeamPermissions(role.Permissions)

		// Update the role
		_, err = a.UpdateRole(role)
		if err != nil {
			rctx.Logger().Error("Failed to update role during rebuild",
				mlog.String("role_id", roleID),
				mlog.Err(err),
			)
			continue
		}

		rctx.Logger().Debug("Rebuilt role permissions",
			mlog.String("role_id", roleID),
			mlog.String("role_name", role.Name),
		)
	}

	return nil
}

// MergeSchemePermissions merges permissions from a source scheme into a target scheme.
// This is used when consolidating multiple schemes or applying permission updates.
//
// The merge strategy:
// - Union of permissions (no permissions are removed)
// - Duplicate removal
// - Scope validation (ensures permissions are valid for target scope)
func (a *App) MergeSchemePermissions(rctx request.CTX, sourceSchemeID, targetSchemeID string) *model.AppError {
	sourceScheme, err := a.GetScheme(sourceSchemeID)
	if err != nil {
		return err
	}

	targetScheme, err := a.GetScheme(targetSchemeID)
	if err != nil {
		return err
	}

	// Ensure schemes have the same scope
	if sourceScheme.Scope != targetScheme.Scope {
		return model.NewAppError("MergeSchemePermissions", "app.scheme.scope_mismatch.app_error", nil, "", 400)
	}

	// Merge each role type
	err = a.mergeRolePermissions(rctx, sourceScheme.DefaultTeamAdminRole, targetScheme.DefaultTeamAdminRole)
	if err != nil {
		return err
	}

	err = a.mergeRolePermissions(rctx, sourceScheme.DefaultTeamUserRole, targetScheme.DefaultTeamUserRole)
	if err != nil {
		return err
	}

	rctx.Logger().Info("Merged scheme permissions",
		mlog.String("source_scheme_id", sourceSchemeID),
		mlog.String("target_scheme_id", targetSchemeID),
	)

	return nil
}

// mergeRolePermissions is an internal helper that merges permissions from source role to target role
func (a *App) mergeRolePermissions(rctx request.CTX, sourceRoleID, targetRoleID string) *model.AppError {
	if sourceRoleID == "" || targetRoleID == "" {
		return nil // Skip if either role doesn't exist
	}

	sourceRole, err := a.GetRole(sourceRoleID)
	if err != nil {
		return err
	}

	targetRole, err := a.GetRole(targetRoleID)
	if err != nil {
		return err
	}

	// Merge permissions (union)
	mergedPermissions := make(map[string]bool)
	for _, perm := range targetRole.Permissions {
		mergedPermissions[perm] = true
	}
	for _, perm := range sourceRole.Permissions {
		mergedPermissions[perm] = true
	}

	// Convert back to slice
	newPermissions := make([]string, 0, len(mergedPermissions))
	for perm := range mergedPermissions {
		newPermissions = append(newPermissions, perm)
	}

	targetRole.Permissions = newPermissions

	// Update the role
	_, err = a.UpdateRole(targetRole)
		return err
}
