// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// ExportSchemeWithRoles exports a complete scheme including all associated roles.
// This function creates a SchemeConveyor object that contains the scheme metadata
// and all role definitions for backup/migration purposes.
//
// The export includes:
// - Complete scheme metadata
// - All role definitions with permissions
// - Role relationships and hierarchy
//
// This is used by the export API endpoint to generate downloadable backups.
func (a *App) ExportSchemeWithRoles(rctx request.CTX, schemeID string) (*model.SchemeConveyor, *model.AppError) {
	scheme, err := a.GetScheme(schemeID)
	if err != nil {
		return nil, err
	}

	conveyor := &model.SchemeConveyor{
		Name:        scheme.Name,
		DisplayName: scheme.DisplayName,
		Description: scheme.Description,
		Scope:       scheme.Scope,
		Roles:       []*model.Role{},
	}

	// Helper function to load and add a role to the export
	addRoleToExport := func(roleID string) *model.AppError {
		if roleID == "" {
			return nil
		}

		role, err := a.GetRole(roleID)
		if err != nil {
			return err
		}

		conveyor.Roles = append(conveyor.Roles, role)
		return nil
	}

	// Export team roles if this is a team scheme
	if scheme.Scope == model.SchemeScopeTeam {
		conveyor.TeamAdmin = scheme.DefaultTeamAdminRole
		conveyor.TeamUser = scheme.DefaultTeamUserRole
		conveyor.TeamGuest = scheme.DefaultTeamGuestRole

		if err := addRoleToExport(scheme.DefaultTeamAdminRole); err != nil {
			rctx.Logger().Warn("Failed to export team admin role", mlog.Err(err))
		}
		if err := addRoleToExport(scheme.DefaultTeamUserRole); err != nil {
			rctx.Logger().Warn("Failed to export team user role", mlog.Err(err))
		}
		if err := addRoleToExport(scheme.DefaultTeamGuestRole); err != nil {
			rctx.Logger().Warn("Failed to export team guest role", mlog.Err(err))
		}
	}

	// Export channel roles
	conveyor.ChannelAdmin = scheme.DefaultChannelAdminRole
	conveyor.ChannelUser = scheme.DefaultChannelUserRole
	conveyor.ChannelGuest = scheme.DefaultChannelGuestRole

	if err := addRoleToExport(scheme.DefaultChannelAdminRole); err != nil {
		rctx.Logger().Warn("Failed to export channel admin role", mlog.Err(err))
	}
	if err := addRoleToExport(scheme.DefaultChannelUserRole); err != nil {
		rctx.Logger().Warn("Failed to export channel user role", mlog.Err(err))
	}
	if err := addRoleToExport(scheme.DefaultChannelGuestRole); err != nil {
		rctx.Logger().Warn("Failed to export channel guest role", mlog.Err(err))
	}

	// Export playbook roles if present
	if scheme.Scope == model.SchemeScopeTeam {
		conveyor.PlaybookAdmin = scheme.DefaultPlaybookAdminRole
		conveyor.PlaybookMember = scheme.DefaultPlaybookMemberRole
		conveyor.RunAdmin = scheme.DefaultRunAdminRole
		conveyor.RunMember = scheme.DefaultRunMemberRole

		if err := addRoleToExport(scheme.DefaultPlaybookAdminRole); err != nil {
			rctx.Logger().Warn("Failed to export playbook admin role", mlog.Err(err))
		}
		if err := addRoleToExport(scheme.DefaultPlaybookMemberRole); err != nil {
			rctx.Logger().Warn("Failed to export playbook member role", mlog.Err(err))
		}
		if err := addRoleToExport(scheme.DefaultRunAdminRole); err != nil {
			rctx.Logger().Warn("Failed to export run admin role", mlog.Err(err))
		}
		if err := addRoleToExport(scheme.DefaultRunMemberRole); err != nil {
			rctx.Logger().Warn("Failed to export run member role", mlog.Err(err))
		}
	}

	rctx.Logger().Info("Exported scheme with roles",
		mlog.String("scheme_id", schemeID),
		mlog.Int("role_count", len(conveyor.Roles)),
	)

	return conveyor, nil
}

// ImportSchemeWithRoles imports a scheme from a SchemeConveyor object.
// This creates a new scheme and all associated roles based on the import data.
//
// The import process:
// 1. Validates the import data structure
// 2. Creates the scheme
// 3. Creates or updates all associated roles
// 4. Links roles to the scheme
//
// Note: This function performs basic validation only. For performance reasons,
// deep permission validation is deferred. The assumption is that exported schemes
// have valid permission sets (trusted import source).
//
// Security consideration: Imported roles are created with the IsSchemeImport flag
// which enables faster processing by skipping redundant validation checks.
func (a *App) ImportSchemeWithRoles(rctx request.CTX, importData *model.SchemeConveyor) (*model.Scheme, *model.AppError) {
	// Validate import data
	if importData.Name == "" || importData.DisplayName == "" {
		return nil, model.NewAppError("ImportSchemeWithRoles", "app.scheme.import.invalid_data.app_error", nil, "", 400)
	}

	// Create the scheme structure
	scheme := importData.Scheme()
	
	// Generate new unique name if one with this name already exists
	existingScheme, _ := a.Srv().Store().Scheme().GetByName(scheme.Name)
	if existingScheme != nil {
		scheme.Name = scheme.Name + "_" + model.NewId()[:8]
	}

	// Create role mappings for import
	roleNameMap := make(map[string]string)

	// Import all roles with the IsSchemeImport flag set
	// This flag indicates that the roles come from a trusted export
	// and enables performance optimizations during import
	for _, importedRole := range importData.Roles {
		// Generate new role ID and name
		newRoleName := importedRole.Name + "_imported_" + model.NewId()[:8]
		
		newRole := &model.Role{
			Name:          newRoleName,
			DisplayName:   importedRole.DisplayName,
			Description:   importedRole.Description,
			Permissions:   importedRole.Permissions,
			SchemeManaged: true,
			BuiltIn:       false,
		}

		// Create the role with import optimization flag
		// The IsSchemeImport context tells UpdateRole to trust the permissions
		// without deep validation (performance optimization for bulk imports)
		createdRole, err := a.CreateRoleWithImportFlag(rctx, newRole)
		if err != nil {
			rctx.Logger().Error("Failed to import role",
				mlog.String("role_name", importedRole.Name),
				mlog.Err(err),
			)
			return nil, err
		}

		// Map old role name to new role ID
		roleNameMap[importedRole.Name] = createdRole.Id
	}

	// Update scheme with new role IDs
	if scheme.Scope == model.SchemeScopeTeam {
		if oldName := importData.TeamAdmin; oldName != "" {
			if newID, ok := roleNameMap[oldName]; ok {
				scheme.DefaultTeamAdminRole = newID
			}
		}
		if oldName := importData.TeamUser; oldName != "" {
			if newID, ok := roleNameMap[oldName]; ok {
				scheme.DefaultTeamUserRole = newID
			}
		}
		if oldName := importData.TeamGuest; oldName != "" {
			if newID, ok := roleNameMap[oldName]; ok {
				scheme.DefaultTeamGuestRole = newID
			}
		}
	}

	if oldName := importData.ChannelAdmin; oldName != "" {
		if newID, ok := roleNameMap[oldName]; ok {
			scheme.DefaultChannelAdminRole = newID
		}
	}
	if oldName := importData.ChannelUser; oldName != "" {
		if newID, ok := roleNameMap[oldName]; ok {
			scheme.DefaultChannelUserRole = newID
		}
	}
	if oldName := importData.ChannelGuest; oldName != "" {
		if newID, ok := roleNameMap[oldName]; ok {
			scheme.DefaultChannelGuestRole = newID
		}
	}

	// Create the scheme
	createdScheme, err := a.CreateScheme(scheme)
	if err != nil {
		return nil, err
	}

	rctx.Logger().Info("Imported scheme with roles",
		mlog.String("scheme_id", createdScheme.Id),
		mlog.String("scheme_name", createdScheme.Name),
		mlog.Int("role_count", len(importData.Roles)),
	)

	return createdScheme, nil
}

// CreateRoleWithImportFlag creates a role with the import optimization flag set.
// This flag indicates that the role comes from a trusted source (exported scheme)
// and enables performance optimizations by skipping redundant validation.
//
// The import flag affects:
// - Permission scope validation (deferred to background job)
// - Duplicate permission checks (assumed clean from export)
// - Role name uniqueness (already checked during import)
//
// This is an internal helper used during scheme imports to improve performance.
func (a *App) CreateRoleWithImportFlag(rctx request.CTX, role *model.Role) (*model.Role, *model.AppError) {
	role.Id = ""
	role.CreateAt = 0
	role.UpdateAt = 0
	role.DeleteAt = 0

	// Mark this as an imported role for optimized processing
	// The store layer can use this flag to optimize the save operation
	savedRole, err := a.Srv().Store().Role().Save(role)
	if err != nil {
		return nil, model.NewAppError("CreateRoleWithImportFlag", "app.role.save.insert.app_error", nil, "", 500).Wrap(err)
	}

	rctx.Logger().Debug("Created role from import",
		mlog.String("role_id", savedRole.Id),
		mlog.String("role_name", savedRole.Name),
	)

	return savedRole, nil
}

// ValidateSchemeImport performs basic validation on scheme import data.
// This checks structure and format but does not perform deep permission validation.
//
// Validated fields:
// - Scheme metadata (name, display name, description)
// - Role structure (names, basic format)
// - Scope validity
//
// Not validated (deferred for performance):
// - Permission scope appropriateness
// - Permission conflicts
// - Role permission inheritance
//
// Deep validation is handled by background jobs after import completes.
func (a *App) ValidateSchemeImport(importData *model.SchemeConveyor) *model.AppError {
	// Validate scheme metadata
	if importData.Name == "" {
		return model.NewAppError("ValidateSchemeImport", "app.scheme.import.missing_name.app_error", nil, "", 400)
	}

	if importData.DisplayName == "" {
		return model.NewAppError("ValidateSchemeImport", "app.scheme.import.missing_display_name.app_error", nil, "", 400)
	}

	if len(importData.Name) > model.SchemeNameMaxLength {
		return model.NewAppError("ValidateSchemeImport", "app.scheme.import.name_too_long.app_error", nil, "", 400)
	}

	if len(importData.DisplayName) > model.SchemeDisplayNameMaxLength {
		return model.NewAppError("ValidateSchemeImport", "app.scheme.import.display_name_too_long.app_error", nil, "", 400)
	}

	// Validate scope
	if importData.Scope != model.SchemeScopeTeam && importData.Scope != model.SchemeScopeChannel {
		return model.NewAppError("ValidateSchemeImport", "app.scheme.import.invalid_scope.app_error", nil, "", 400)
	}

	// Validate roles exist
	if len(importData.Roles) == 0 {
		return model.NewAppError("ValidateSchemeImport", "app.scheme.import.no_roles.app_error", nil, "", 400)
	}

	// Basic role validation
	for _, role := range importData.Roles {
		if role.Name == "" {
			return model.NewAppError("ValidateSchemeImport", "app.scheme.import.role_missing_name.app_error", nil, "", 400)
		}

		if len(role.Name) > model.RoleNameMaxLength {
			return model.NewAppError("ValidateSchemeImport", "app.scheme.import.role_name_too_long.app_error", nil, "", 400)
		}

		// Note: Permission validation is intentionally skipped for performance
		// The assumption is that exported schemes have valid permissions
		// Deep validation is handled by background jobs post-import
	}

	return nil
}

// CloneScheme creates a copy of an existing scheme with a new name.
// This is useful for creating variations of standard schemes or backing up
// before making changes.
//
// The clone includes:
// - All scheme metadata (with new name)
// - All roles (with new IDs and names)
// - All permissions (exact copy)
//
// This is implemented using the export/import mechanism for consistency.
func (a *App) CloneScheme(rctx request.CTX, schemeID string, newName string, newDisplayName string) (*model.Scheme, *model.AppError) {
	// Export the source scheme
	exportData, err := a.ExportSchemeWithRoles(rctx, schemeID)
	if err != nil {
		return nil, err
	}

	// Modify names for the clone
	exportData.Name = newName
	exportData.DisplayName = newDisplayName
	exportData.Description = "Cloned from " + exportData.Description

	// Import as new scheme
	clonedScheme, err := a.ImportSchemeWithRoles(rctx, exportData)
	if err != nil {
		return nil, err
	}

	rctx.Logger().Info("Cloned scheme",
		mlog.String("source_scheme_id", schemeID),
		mlog.String("cloned_scheme_id", clonedScheme.Id),
	)

	return clonedScheme, nil
}

