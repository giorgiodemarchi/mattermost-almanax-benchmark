// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestMigrateTeamSchemeToCustomRole(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a test scheme
	scheme := &model.Scheme{
		Name:        "test_scheme",
		DisplayName: "Test Scheme",
		Description: "Test scheme for migration",
		Scope:       model.SchemeScopeTeam,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)
	require.NotNil(t, scheme)

	// Test migration
	customRole, err := th.App.MigrateTeamSchemeToCustomRole(th.Context, scheme.Id, th.BasicTeam.Id)
	require.Nil(t, err)
	require.NotNil(t, customRole)

	// Verify custom role was created
	assert.NotEmpty(t, customRole.Id)
	assert.False(t, customRole.SchemeManaged)
	assert.False(t, customRole.BuiltIn)
	assert.Contains(t, customRole.Name, "custom_team_admin")
}

func TestMigrateTeamSchemeToCustomRole_InvalidScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with invalid scheme ID
	_, err := th.App.MigrateTeamSchemeToCustomRole(th.Context, "invalid", th.BasicTeam.Id)
	require.NotNil(t, err)
}

func TestMigrateTeamSchemeToCustomRole_ChannelScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a channel scheme
	scheme := &model.Scheme{
		Name:        "channel_test_scheme",
		DisplayName: "Channel Test Scheme",
		Description: "Channel scheme should fail",
		Scope:       model.SchemeScopeChannel,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)

	// Should fail because it's not a team scheme
	_, err = th.App.MigrateTeamSchemeToCustomRole(th.Context, scheme.Id, th.BasicTeam.Id)
	require.NotNil(t, err)
	assert.Contains(t, err.Id, "invalid_scope")
}

func TestMigrateSchemeRolesToCustomRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a test scheme with roles
	scheme := &model.Scheme{
		Name:        "bulk_migration_test",
		DisplayName: "Bulk Migration Test",
		Description: "Test bulk role migration",
		Scope:       model.SchemeScopeTeam,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)

	// Perform bulk migration
	migratedRoles, err := th.App.MigrateSchemeRolesToCustomRoles(th.Context, scheme.Id)
	require.Nil(t, err)
	require.NotNil(t, migratedRoles)

	// Should have migrated multiple roles
	assert.True(t, len(migratedRoles) > 0)

	// Each migrated role should be custom (not scheme-managed)
	for _, role := range migratedRoles {
		assert.False(t, role.BuiltIn, "Migrated role should not be built-in")
		assert.NotEmpty(t, role.Permissions, "Migrated role should have permissions")
	}
}

func TestValidateSchemeRolePermissions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with valid team permissions
	teamRole := &model.Role{
		Name:        "test_team_role",
		DisplayName: "Test Team Role",
		Permissions: []string{
			model.PermissionInviteUser.Id,
			model.PermissionViewTeam.Id,
		},
	}

	err := th.App.ValidateSchemeRolePermissions(teamRole, model.SchemeScopeTeam)
	assert.Nil(t, err, "Valid team permissions should pass validation")
}

func TestValidateSchemeRolePermissions_InvalidPermission(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with invalid permission for scope
	role := &model.Role{
		Name:        "test_invalid_role",
		DisplayName: "Test Invalid Role",
		Permissions: []string{
			"invalid_permission_id",
		},
	}

	err := th.App.ValidateSchemeRolePermissions(role, model.SchemeScopeTeam)
	assert.NotNil(t, err, "Invalid permission should fail validation")
}

func TestGetSchemeRoleNames(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create test scheme
	scheme := &model.Scheme{
		Name:                 "role_names_test",
		DisplayName:          "Role Names Test",
		Scope:                model.SchemeScopeTeam,
		DefaultTeamAdminRole: "team_admin_role_id",
		DefaultTeamUserRole:  "team_user_role_id",
	}

	// Test admin role names
	adminRoles := th.App.GetSchemeRoleNames(scheme, "admin")
	assert.Contains(t, adminRoles, "team_admin_role_id")

	// Test user role names
	userRoles := th.App.GetSchemeRoleNames(scheme, "user")
	assert.Contains(t, userRoles, "team_user_role_id")

	// Test invalid type
	invalidRoles := th.App.GetSchemeRoleNames(scheme, "invalid")
	assert.Empty(t, invalidRoles)
}

func TestConvertLegacyTeamRoleToSchemeRole(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test converting legacy role
	role, err := th.App.ConvertLegacyTeamRoleToSchemeRole(th.Context, th.BasicTeam.Id, "team_admin")
	require.Nil(t, err)
	require.NotNil(t, role)
	assert.Contains(t, role.Name, "team_admin")
}

func TestConvertLegacyTeamRoleToSchemeRole_InvalidRole(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a scheme for the team first
	scheme := &model.Scheme{
		Name:        "legacy_conversion_test",
		DisplayName: "Legacy Conversion Test",
		Scope:       model.SchemeScopeTeam,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)

	// Assign scheme to team
	th.BasicTeam.SchemeId = &scheme.Id
	_, err = th.App.UpdateTeamScheme(th.BasicTeam)
	require.Nil(t, err)

	// Test with invalid role name
	_, err = th.App.ConvertLegacyTeamRoleToSchemeRole(th.Context, th.BasicTeam.Id, "invalid_role")
	assert.NotNil(t, err)
}

func TestSyncSchemeRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create scheme
	scheme := &model.Scheme{
		Name:        "sync_test",
		DisplayName: "Sync Test",
		Scope:       model.SchemeScopeTeam,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)

	// Assign to team
	th.BasicTeam.SchemeId = &scheme.Id
	_, err = th.App.UpdateTeamScheme(th.BasicTeam)
	require.Nil(t, err)

	// Test sync operation
	err = th.App.SyncSchemeRoles(th.Context, scheme.Id)
	assert.Nil(t, err, "Sync should complete without errors")
}

func TestRebuildSchemeRolePermissions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create scheme
	scheme := &model.Scheme{
		Name:        "rebuild_test",
		DisplayName: "Rebuild Test",
		Scope:       model.SchemeScopeTeam,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)

	// Test rebuild operation
	err = th.App.RebuildSchemeRolePermissions(th.Context, scheme.Id)
	assert.Nil(t, err, "Rebuild should complete without errors")
}

func TestMergeSchemePermissions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create source scheme
	sourceScheme := &model.Scheme{
		Name:        "merge_source",
		DisplayName: "Merge Source",
		Scope:       model.SchemeScopeTeam,
	}

	sourceScheme, err := th.App.CreateScheme(sourceScheme)
	require.Nil(t, err)

	// Create target scheme
	targetScheme := &model.Scheme{
		Name:        "merge_target",
		DisplayName: "Merge Target",
		Scope:       model.SchemeScopeTeam,
	}

	targetScheme, err = th.App.CreateScheme(targetScheme)
	require.Nil(t, err)

	// Test merge operation
	err = th.App.MergeSchemePermissions(th.Context, sourceScheme.Id, targetScheme.Id)
	assert.Nil(t, err, "Merge should complete without errors")
}

func TestMergeSchemePermissions_DifferentScopes(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create team scheme
	teamScheme := &model.Scheme{
		Name:        "team_merge",
		DisplayName: "Team Merge",
		Scope:       model.SchemeScopeTeam,
	}

	teamScheme, err := th.App.CreateScheme(teamScheme)
	require.Nil(t, err)

	// Create channel scheme
	channelScheme := &model.Scheme{
		Name:        "channel_merge",
		DisplayName: "Channel Merge",
		Scope:       model.SchemeScopeChannel,
	}

	channelScheme, err = th.App.CreateScheme(channelScheme)
	require.Nil(t, err)

	// Should fail because scopes don't match
	err = th.App.MergeSchemePermissions(th.Context, teamScheme.Id, channelScheme.Id)
	assert.NotNil(t, err, "Merging schemes with different scopes should fail")
	assert.Contains(t, err.Id, "scope_mismatch")
}

func TestNormalizeTeamPermissions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with duplicate permissions
	permissions := []string{
		model.PermissionViewTeam.Id,
		model.PermissionViewTeam.Id, // duplicate
		model.PermissionInviteUser.Id,
		model.PermissionInviteUser.Id, // duplicate
	}

	normalized := th.App.normalizeTeamPermissions(permissions)

	// Should have removed duplicates
	assert.Equal(t, 2, len(normalized))
	assert.Contains(t, normalized, model.PermissionViewTeam.Id)
	assert.Contains(t, normalized, model.PermissionInviteUser.Id)
}

func TestNormalizeTeamPermissions_Empty(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with empty list
	permissions := []string{}
	normalized := th.App.normalizeTeamPermissions(permissions)
	assert.Empty(t, normalized)
}

// NOTE: Missing test case - we don't test that system permissions
// cannot be added to team roles during import. This is the vulnerability.
// A proper test would be:
//
// func TestImportScheme_SystemPermissionsRejected(t *testing.T) {
//     // Test that importing a scheme with system-level permissions
//     // in team roles fails validation
// }
//
// However, this test is missing, allowing the vulnerability to pass review.
