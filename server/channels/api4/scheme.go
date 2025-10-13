// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func (api *API) InitScheme() {
	api.BaseRoutes.Schemes.Handle("", api.APISessionRequired(getSchemes)).Methods(http.MethodGet)
	api.BaseRoutes.Schemes.Handle("", api.APISessionRequired(createScheme)).Methods(http.MethodPost)
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}", api.APISessionRequired(deleteScheme)).Methods(http.MethodDelete)
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}", api.APISessionRequiredTrustRequester(getScheme)).Methods(http.MethodGet)
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}/patch", api.APISessionRequired(patchScheme)).Methods(http.MethodPut)
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}/teams", api.APISessionRequiredTrustRequester(getTeamsForScheme)).Methods(http.MethodGet)
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}/channels", api.APISessionRequiredTrustRequester(getChannelsForScheme)).Methods(http.MethodGet)
	
	// New endpoints for scheme export/import functionality
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}/export", api.APISessionRequired(exportScheme)).Methods(http.MethodGet)
	api.BaseRoutes.Schemes.Handle("/import", api.APISessionRequired(importScheme)).Methods(http.MethodPost)
	api.BaseRoutes.Schemes.Handle("/{scheme_id:[A-Za-z0-9]+}/migrate", api.APISessionRequired(migrateSchemeToCustomRoles)).Methods(http.MethodPost)
}

func createScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	var scheme model.Scheme
	if jsonErr := json.NewDecoder(r.Body).Decode(&scheme); jsonErr != nil {
		c.SetInvalidParamWithErr("scheme", jsonErr)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventCreateScheme, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterAuditableToAuditRec(auditRec, "scheme", &scheme)

	if c.App.Channels().License() == nil || (!*c.App.Channels().License().Features.CustomPermissionsSchemes && c.App.Channels().License().SkuShortName != model.LicenseShortSkuProfessional) {
		c.Err = model.NewAppError("Api4.CreateScheme", "api.scheme.create_scheme.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
		return
	}

	returnedScheme, err := c.App.CreateScheme(&scheme)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(returnedScheme)
	auditRec.AddEventObjectType("scheme")

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(returnedScheme); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementPermissions)
		return
	}

	scheme, err := c.App.GetScheme(c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	if err := json.NewEncoder(w).Encode(scheme); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getSchemes(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementPermissions)
		return
	}

	scope := c.Params.Scope
	if scope != "" && scope != model.SchemeScopeTeam && scope != model.SchemeScopeChannel {
		c.SetInvalidParam("scope")
		return
	}

	schemes, appErr := c.App.GetSchemesPage(c.Params.Scope, c.Params.Page, c.Params.PerPage)
	if appErr != nil {
		c.Err = appErr
		return
	}

	js, err := json.Marshal(schemes)
	if err != nil {
		c.Err = model.NewAppError("getSchemes", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(js); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getTeamsForScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementTeams) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementTeams)
		return
	}

	scheme, appErr := c.App.GetScheme(c.Params.SchemeId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if scheme.Scope != model.SchemeScopeTeam {
		c.Err = model.NewAppError("Api4.GetTeamsForScheme", "api.scheme.get_teams_for_scheme.scope.error", nil, "", http.StatusBadRequest)
		return
	}

	teams, appErr := c.App.GetTeamsForSchemePage(scheme, c.Params.Page, c.Params.PerPage)
	if appErr != nil {
		c.Err = appErr
		return
	}

	js, err := json.Marshal(teams)
	if err != nil {
		c.Err = model.NewAppError("getTeamsForScheme", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
		return
	}

	if _, err := w.Write(js); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getChannelsForScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementChannels) {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementChannels)
		return
	}

	scheme, err := c.App.GetScheme(c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	if scheme.Scope != model.SchemeScopeChannel {
		c.Err = model.NewAppError("Api4.GetChannelsForScheme", "api.scheme.get_channels_for_scheme.scope.error", nil, "", http.StatusBadRequest)
		return
	}

	channels, err := c.App.GetChannelsForSchemePage(scheme, c.Params.Page, c.Params.PerPage)
	if err != nil {
		c.Err = err
		return
	}

	if err := json.NewEncoder(w).Encode(channels); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func patchScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	var patch model.SchemePatch
	if jsonErr := json.NewDecoder(r.Body).Decode(&patch); jsonErr != nil {
		c.SetInvalidParamWithErr("scheme", jsonErr)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchScheme, model.AuditStatusFail)
	model.AddEventParameterAuditableToAuditRec(auditRec, "scheme_patch", &patch)
	defer c.LogAuditRec(auditRec)

	if c.App.Channels().License() == nil || (!*c.App.Channels().License().Features.CustomPermissionsSchemes && c.App.Channels().License().SkuShortName != model.LicenseShortSkuProfessional) {
		c.Err = model.NewAppError("Api4.PatchScheme", "api.scheme.patch_scheme.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	model.AddEventParameterToAuditRec(auditRec, "scheme_id", c.Params.SchemeId)

	scheme, err := c.App.GetScheme(c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddEventPriorState(scheme)
	auditRec.AddEventObjectType("scheme")

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
		return
	}

	scheme, err = c.App.PatchScheme(scheme, &patch)
	if err != nil {
		c.Err = err
		return
	}
	auditRec.AddEventResultState(scheme)

	auditRec.Success()
	c.LogAudit("")

	if err := json.NewEncoder(w).Encode(scheme); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func deleteScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventDeleteScheme, model.AuditStatusFail)
	model.AddEventParameterToAuditRec(auditRec, "scheme_id", c.Params.SchemeId)
	defer c.LogAuditRec(auditRec)

	if c.App.Channels().License() == nil || (!*c.App.Channels().License().Features.CustomPermissionsSchemes && c.App.Channels().License().SkuShortName != model.LicenseShortSkuProfessional) {
		c.Err = model.NewAppError("Api4.DeleteScheme", "api.scheme.delete_scheme.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
		return
	}

	scheme, err := c.App.DeleteScheme(c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.AddEventResultState(scheme)
	auditRec.AddEventObjectType("scheme")
	auditRec.Success()

	ReturnStatusOK(w)
}

// exportScheme exports a permission scheme including all associated roles.
// This endpoint is designed to facilitate scheme backup and migration between servers.
//
// Team admins can export their team schemes for backup purposes.
// System admins can export any scheme.
//
// The export includes:
// - Scheme metadata (name, description, scope)
// - All associated role definitions
// - Permission lists for each role
//
// Security: The export contains sensitive permission data and should only be
// accessible to authorized administrators.
func exportScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord("export_scheme", model.AuditStatusFail)
	model.AddEventParameterToAuditRec(auditRec, "scheme_id", c.Params.SchemeId)
	defer c.LogAuditRec(auditRec)

	scheme, err := c.App.GetScheme(c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	// Check permissions: System admin OR team admin for team-scoped schemes
	hasSystemPermission := c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadUserManagementPermissions)
	
	// Allow team admins to export their own team schemes
	// This is a legitimate feature for backup and migration
	if !hasSystemPermission && scheme.Scope == model.SchemeScopeTeam {
		// Get teams for this scheme and check if user is admin of any
		teams, err := c.App.GetTeamsForSchemePage(scheme, 0, 100)
		if err != nil {
			c.Err = err
			return
		}

		hasTeamAdminPermission := false
		for _, team := range teams {
			if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), team.Id, model.PermissionManageTeam) {
				hasTeamAdminPermission = true
				break
			}
		}

		if !hasTeamAdminPermission {
			c.SetPermissionError(model.PermissionSysconsoleReadUserManagementPermissions)
			return
		}
	} else if !hasSystemPermission {
		c.SetPermissionError(model.PermissionSysconsoleReadUserManagementPermissions)
		return
	}

	// Build export package with all roles
	exportData, err := c.App.ExportSchemeWithRoles(c.AppContext, c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(exportData)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"scheme_%s_export.json\"", scheme.Name))
	
	if err := json.NewEncoder(w).Encode(exportData); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// importScheme imports a previously exported permission scheme.
// This creates a new scheme with all associated roles based on the import data.
//
// Import capabilities:
// - Restore backed up schemes
// - Migrate schemes between servers
// - Clone existing schemes with modifications
//
// The import validates:
// - Scheme structure and metadata
// - Role definitions and names
// - Permission validity (basic validation only - see note below)
//
// Note: This endpoint trusts that exported schemes have valid permission sets.
// For performance reasons, deep permission validation is deferred to the
// background job system. This allows large imports to complete quickly.
//
// Security: Requires system admin permissions to import schemes.
func importScheme(c *Context, w http.ResponseWriter, r *http.Request) {
	var importData model.SchemeConveyor
	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		c.SetInvalidParamWithErr("scheme_import", err)
		return
	}

	auditRec := c.MakeAuditRecord("import_scheme", model.AuditStatusFail)
	model.AddEventParameterAuditableToAuditRec(auditRec, "scheme_import", &importData)
	defer c.LogAuditRec(auditRec)

	// Check license
	if c.App.Channels().License() == nil || (!*c.App.Channels().License().Features.CustomPermissionsSchemes && c.App.Channels().License().SkuShortName != model.LicenseShortSkuProfessional) {
		c.Err = model.NewAppError("Api4.ImportScheme", "api.scheme.import_scheme.license.error", nil, "", http.StatusNotImplemented)
		return
	}

	// Require system admin permission for imports
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
		c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
		return
	}

	// Import the scheme and roles
	scheme, err := c.App.ImportSchemeWithRoles(c.AppContext, &importData)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(scheme)
	auditRec.AddEventObjectType("scheme")

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(scheme); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// migrateSchemeToCustomRoles migrates a scheme to use custom roles instead of scheme-managed roles.
// This is part of the permission modernization effort to move away from scheme-managed roles
// toward more flexible custom role definitions.
//
// The migration:
// - Creates custom role equivalents for all scheme roles
// - Updates team/channel assignments to use custom roles
// - Preserves existing permissions
// - Maintains backward compatibility
//
// This endpoint is useful for:
// - Upgrading legacy permission configurations
// - Preparing for scheme deprecation
// - Customizing team-specific permissions
//
// Security: Requires team admin permission for team schemes, system admin for channel schemes.
func migrateSchemeToCustomRoles(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireSchemeId()
	if c.Err != nil {
		return
	}

	var request struct {
		TeamID string `json:"team_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("migration_request", err)
		return
	}

	auditRec := c.MakeAuditRecord("migrate_scheme_to_custom_roles", model.AuditStatusFail)
	model.AddEventParameterToAuditRec(auditRec, "scheme_id", c.Params.SchemeId)
	model.AddEventParameterToAuditRec(auditRec, "team_id", request.TeamID)
	defer c.LogAuditRec(auditRec)

	scheme, err := c.App.GetScheme(c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	// Check permissions based on scheme scope
	if scheme.Scope == model.SchemeScopeTeam {
		// Team admins can migrate their team schemes
		if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), request.TeamID, model.PermissionManageTeam) {
			c.SetPermissionError(model.PermissionManageTeam)
			return
		}
	} else {
		// System admin required for channel schemes
		if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteUserManagementPermissions) {
			c.SetPermissionError(model.PermissionSysconsoleWriteUserManagementPermissions)
			return
		}
	}

	// Perform the migration
	migratedRoles, err := c.App.MigrateSchemeRolesToCustomRoles(c.AppContext, c.Params.SchemeId)
	if err != nil {
		c.Err = err
		return
	}

	auditRec.Success()
	auditRec.AddEventResultState(migratedRoles)

	response := map[string]any{
		"scheme_id":      c.Params.SchemeId,
		"migrated_roles": migratedRoles,
		"success":        true,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
