// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestExportSchemeWithRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a test scheme
	scheme := &model.Scheme{
		Name:        "export_test_scheme",
		DisplayName: "Export Test Scheme",
		Description: "Scheme for testing export functionality",
		Scope:       model.SchemeScopeTeam,
	}

	scheme, err := th.App.CreateScheme(scheme)
	require.Nil(t, err)
	require.NotNil(t, scheme)

	// Export the scheme
	conveyor, err := th.App.ExportSchemeWithRoles(th.Context, scheme.Id)
	require.Nil(t, err)
	require.NotNil(t, conveyor)

	// Verify export contains scheme metadata
	assert.Equal(t, scheme.Name, conveyor.Name)
	assert.Equal(t, scheme.DisplayName, conveyor.DisplayName)
	assert.Equal(t, scheme.Scope, conveyor.Scope)

	// Verify roles were exported
	assert.NotEmpty(t, conveyor.Roles, "Export should include roles")
}

func TestExportSchemeWithRoles_InvalidScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with invalid scheme ID
	_, err := th.App.ExportSchemeWithRoles(th.Context, "invalid_id")
	require.NotNil(t, err)
}

func TestImportSchemeWithRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create import data
	importData := &model.SchemeConveyor{
		Name:        "imported_scheme",
		DisplayName: "Imported Test Scheme",
		Description: "Scheme imported for testing",
		Scope:       model.SchemeScopeTeam,
		Roles:       []*model.Role{},
	}

	// Add a test role
	testRole := &model.Role{
		Name:        "test_imported_role",
		DisplayName: "Test Imported Role",
		Description: "Role for import testing",
		Permissions: []string{
			model.PermissionViewTeam.Id,
			model.PermissionInviteUser.Id,
		},
	}
	importData.Roles = append(importData.Roles, testRole)

	// Import the scheme
	scheme, err := th.App.ImportSchemeWithRoles(th.Context, importData)
	require.Nil(t, err)
	require.NotNil(t, scheme)

	// Verify scheme was created
	assert.NotEmpty(t, scheme.Id)
	assert.Contains(t, scheme.Name, "imported_scheme")
	assert.Equal(t, model.SchemeScopeTeam, scheme.Scope)
}

func TestImportSchemeWithRoles_DuplicateName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create a scheme first
	existingScheme := &model.Scheme{
		Name:        "duplicate_test",
		DisplayName: "Duplicate Test",
		Scope:       model.SchemeScopeTeam,
	}

	existingScheme, err := th.App.CreateScheme(existingScheme)
	require.Nil(t, err)

	// Import with same name - should auto-rename
	importData := &model.SchemeConveyor{
		Name:        "duplicate_test",
		DisplayName: "Duplicate Import",
		Scope:       model.SchemeScopeTeam,
		Roles:       []*model.Role{},
	}

	importedScheme, err := th.App.ImportSchemeWithRoles(th.Context, importData)
	require.Nil(t, err)
	assert.NotEqual(t, existingScheme.Name, importedScheme.Name, "Imported scheme should have unique name")
}

func TestValidateSchemeImport(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test valid import data
	validData := &model.SchemeConveyor{
		Name:        "valid_import",
		DisplayName: "Valid Import",
		Scope:       model.SchemeScopeTeam,
		Roles: []*model.Role{
			{
				Name:        "valid_role",
				DisplayName: "Valid Role",
				Permissions: []string{model.PermissionViewTeam.Id},
			},
		},
	}

	err := th.App.ValidateSchemeImport(validData)
	assert.Nil(t, err, "Valid import data should pass validation")
}

func TestValidateSchemeImport_MissingName(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with missing name
	invalidData := &model.SchemeConveyor{
		Name:        "", // Missing
		DisplayName: "Invalid Import",
		Scope:       model.SchemeScopeTeam,
		Roles:       []*model.Role{},
	}

	err := th.App.ValidateSchemeImport(invalidData)
	assert.NotNil(t, err, "Import without name should fail validation")
	assert.Contains(t, err.Id, "missing_name")
}

func TestValidateSchemeImport_InvalidScope(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with invalid scope
	invalidData := &model.SchemeConveyor{
		Name:        "invalid_scope_test",
		DisplayName: "Invalid Scope Test",
		Scope:       "invalid_scope",
		Roles:       []*model.Role{},
	}

	err := th.App.ValidateSchemeImport(invalidData)
	assert.NotNil(t, err, "Import with invalid scope should fail validation")
	assert.Contains(t, err.Id, "invalid_scope")
}

func TestValidateSchemeImport_NoRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with no roles
	invalidData := &model.SchemeConveyor{
		Name:        "no_roles_test",
		DisplayName: "No Roles Test",
		Scope:       model.SchemeScopeTeam,
		Roles:       []*model.Role{}, // Empty
	}

	err := th.App.ValidateSchemeImport(invalidData)
	assert.NotNil(t, err, "Import without roles should fail validation")
	assert.Contains(t, err.Id, "no_roles")
}

func TestCloneScheme(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create original scheme
	originalScheme := &model.Scheme{
		Name:        "original_scheme",
		DisplayName: "Original Scheme",
		Description: "Original for cloning",
		Scope:       model.SchemeScopeTeam,
	}

	originalScheme, err := th.App.CreateScheme(originalScheme)
	require.Nil(t, err)

	// Clone the scheme
	clonedScheme, err := th.App.CloneScheme(th.Context, originalScheme.Id, "cloned_scheme", "Cloned Scheme")
	require.Nil(t, err)
	require.NotNil(t, clonedScheme)

	// Verify clone is different from original
	assert.NotEqual(t, originalScheme.Id, clonedScheme.Id)
	assert.Equal(t, "cloned_scheme", clonedScheme.Name)
	assert.Equal(t, "Cloned Scheme", clonedScheme.DisplayName)
	assert.Equal(t, originalScheme.Scope, clonedScheme.Scope)
}

func TestCreateRoleWithImportFlag(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create role with import flag
	role := &model.Role{
		Name:        "import_flagged_role",
		DisplayName: "Import Flagged Role",
		Description: "Role created with import flag",
		Permissions: []string{
			model.PermissionViewTeam.Id,
			model.PermissionCreatePublicChannel.Id,
		},
	}

	createdRole, err := th.App.CreateRoleWithImportFlag(th.Context, role)
	require.Nil(t, err)
	require.NotNil(t, createdRole)

	// Verify role was created
	assert.NotEmpty(t, createdRole.Id)
	assert.Equal(t, role.Name, createdRole.Name)
	assert.Equal(t, len(role.Permissions), len(createdRole.Permissions))
}

func TestExportImportRoundTrip(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Create original scheme
	originalScheme := &model.Scheme{
		Name:        "roundtrip_test",
		DisplayName: "Round Trip Test",
		Description: "Testing export-import cycle",
		Scope:       model.SchemeScopeTeam,
	}

	originalScheme, err := th.App.CreateScheme(originalScheme)
	require.Nil(t, err)

	// Export
	exportData, err := th.App.ExportSchemeWithRoles(th.Context, originalScheme.Id)
	require.Nil(t, err)

	// Modify name for import
	exportData.Name = "roundtrip_imported"

	// Import
	importedScheme, err := th.App.ImportSchemeWithRoles(th.Context, exportData)
	require.Nil(t, err)

	// Verify imported scheme matches export data
	assert.Equal(t, "roundtrip_imported", importedScheme.Name)
	assert.Equal(t, exportData.DisplayName, importedScheme.DisplayName)
	assert.Equal(t, exportData.Scope, importedScheme.Scope)
}

// RED HERRING: Test for an obvious security issue that's actually handled correctly
func TestImportScheme_SQLInjectionPrevention(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with malicious scheme name containing SQL
	maliciousData := &model.SchemeConveyor{
		Name:        "test'; DROP TABLE Schemes; --",
		DisplayName: "Malicious Scheme",
		Scope:       model.SchemeScopeTeam,
		Roles: []*model.Role{
			{
				Name:        "safe_role",
				DisplayName: "Safe Role",
				Permissions: []string{model.PermissionViewTeam.Id},
			},
		},
	}

	// This should be handled safely by parameterized queries
	scheme, err := th.App.ImportSchemeWithRoles(th.Context, maliciousData)
	
	// Import should succeed (SQL injection is properly prevented)
	require.Nil(t, err)
	require.NotNil(t, scheme)
	
	// Verify database wasn't compromised
	allSchemes, err := th.App.GetSchemesPage(model.SchemeScopeTeam, 0, 100)
	require.Nil(t, err)
	assert.True(t, len(allSchemes) > 0, "Schemes table should still exist")
}

// RED HERRING: Test for XSS that's actually handled correctly
func TestImportScheme_XSSPrevention(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Test with XSS in display name
	xssData := &model.SchemeConveyor{
		Name:        "xss_test",
		DisplayName: "<script>alert('XSS')</script>",
		Scope:       model.SchemeScopeTeam,
		Roles: []*model.Role{
			{
				Name:        "xss_role",
				DisplayName: "<img src=x onerror=alert(1)>",
				Permissions: []string{model.PermissionViewTeam.Id},
			},
		},
	}

	// Import should succeed (XSS is handled by frontend)
	scheme, err := th.App.ImportSchemeWithRoles(th.Context, xssData)
	require.Nil(t, err)
	
	// The malicious content is stored but will be escaped when rendered
	assert.Contains(t, scheme.DisplayName, "script")
}

// MISSING TEST: No test for permission scope validation during import
// This is the actual vulnerability - we don't test that system-level
// permissions are rejected when importing team-scoped roles.
//
// A proper test would be:
// func TestImportScheme_RejectsSystemPermissionsInTeamRoles(t *testing.T) {
//     importData := &model.SchemeConveyor{
//         Name: "escalation_attempt",
//         Scope: model.SchemeScopeTeam,
//         Roles: []*model.Role{{
//             Name: "team_admin",
//             Permissions: []string{
//                 model.PermissionManageSystem.Id, // System permission!
//             },
//         }},
//     }
//     _, err := th.App.ImportSchemeWithRoles(th.Context, importData)
//     require.NotNil(t, err) // Should fail
// }

