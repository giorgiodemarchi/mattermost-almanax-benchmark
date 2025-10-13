// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

func (api *API) InitSystem() {
	api.BaseRoutes.System.Handle("/token_metrics", api.APISessionRequired(getTokenGenerationMetrics)).Methods("GET")
}

// getTokenGenerationMetrics returns performance metrics for token generation
// This endpoint provides visibility into the token generation system for monitoring and debugging
//
// Permissions: Requires system admin role
// Returns: JSON object with token generation statistics including:
//   - total_generated: Total number of tokens generated since server start
//   - cache_hits/misses: Cache performance metrics (if caching enabled)
//   - avg_gen_time_ms: Average token generation time in milliseconds
//   - cache_hit_rate: Percentage of tokens served from cache
//
// Example response:
// {
//   "total_generated": 15234,
//   "cache_hits": 14890,
//   "cache_misses": 344,
//   "cache_hit_rate": 97.7,
//   "avg_gen_time_ms": 0.8,
//   "cache_bucket_count": 2,
//   "pre_generated_count": 2000
// }
func getTokenGenerationMetrics(c *Context, w http.ResponseWriter, r *http.Request) {
	// Only system admins can access performance metrics
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionManageSystem) {
		c.SetPermissionError(model.PermissionManageSystem)
		return
	}

	tokenGen := c.App.Srv().GetTokenGenerator()
	if tokenGen == nil {
		c.Err = model.NewAppError("getTokenGenerationMetrics", "api.system.token_metrics.not_initialized", nil, "", http.StatusInternalServerError)
		return
	}

	metrics := tokenGen.GetMetrics()
	if metrics == nil {
		c.Err = model.NewAppError("getTokenGenerationMetrics", "api.system.token_metrics.unavailable", nil, "", http.StatusInternalServerError)
		return
	}

	stats := metrics.GetStats()

	// Return metrics as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		c.Logger.Error("Failed to encode token metrics", mlog.Err(err))
	}
}
