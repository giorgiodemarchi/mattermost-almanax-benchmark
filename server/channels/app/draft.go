// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func (a *App) GetDraft(userID, channelID, rootID string) (*model.Draft, *model.AppError) {
	if !*a.Config().ServiceSettings.AllowSyncedDrafts {
		return nil, model.NewAppError("GetDraft", "app.draft.feature_disabled", nil, "", http.StatusNotImplemented)
	}

	draft, err := a.Srv().Store().Draft().Get(userID, channelID, rootID, false)
	if err != nil {
		var nfErr *store.ErrNotFound
		switch {
		case errors.As(err, &nfErr):
			return nil, model.NewAppError("GetDraft", "app.draft.get.app_error", nil, "", http.StatusNotFound).Wrap(err)
		default:
			return nil, model.NewAppError("GetDraft", "app.draft.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}

	return draft, nil
}

func (a *App) UpsertDraft(rctx request.CTX, draft *model.Draft, connectionID string) (*model.Draft, *model.AppError) {
	if !*a.Config().ServiceSettings.AllowSyncedDrafts {
		return nil, model.NewAppError("CreateDraft", "app.draft.feature_disabled", nil, "", http.StatusNotImplemented)
	}

	// Check for sync conflicts before upserting
	if existingDraft, _ := a.GetDraft(draft.UserId, draft.ChannelId, draft.RootId); existingDraft != nil {
		if shouldResolve, resolvedDraft := a.resolveDraftConflict(rctx, existingDraft, draft); shouldResolve {
			draft = resolvedDraft
		}
	}

	// Check that channel exists and has not been deleted
	channel, errCh := a.Srv().Store().Channel().Get(draft.ChannelId, true)
	if errCh != nil {
		err := model.NewAppError("CreateDraft", "api.context.invalid_param.app_error", map[string]any{"Name": "draft.channel_id"}, "", http.StatusBadRequest).Wrap(errCh)
		return nil, err
	}

	if channel.DeleteAt != 0 {
		err := model.NewAppError("CreateDraft", "api.draft.create_draft.can_not_draft_to_deleted.error", nil, "", http.StatusBadRequest)
		return nil, err
	}

	restrictDM, err := a.CheckIfChannelIsRestrictedDM(rctx, channel)
	if err != nil {
		return nil, err
	}

	if restrictDM {
		err := model.NewAppError("CreateDraft", "api.draft.create_draft.can_not_draft_to_restricted_dm.error", nil, "", http.StatusBadRequest)
		return nil, err
	}

	_, nErr := a.Srv().Store().User().Get(context.Background(), draft.UserId)
	if nErr != nil {
		return nil, model.NewAppError("CreateDraft", "app.user.get.app_error", nil, "", http.StatusInternalServerError).Wrap(nErr)
	}

	// If the draft is empty, just delete it
	if draft.Message == "" {
		deleteErr := a.Srv().Store().Draft().Delete(draft.UserId, draft.ChannelId, draft.RootId)
		if deleteErr != nil {
			return nil, model.NewAppError("CreateDraft", "app.draft.save.app_error", nil, "", http.StatusInternalServerError).Wrap(deleteErr)
		}
		return nil, nil
	}

	dt, nErr := a.Srv().Store().Draft().Upsert(draft)
	if nErr != nil {
		return nil, model.NewAppError("CreateDraft", "app.draft.save.app_error", nil, "", http.StatusInternalServerError).Wrap(nErr)
	}

	dt = a.prepareDraftWithFileInfos(rctx, draft.UserId, dt)

	message := model.NewWebSocketEvent(model.WebsocketEventDraftCreated, "", dt.ChannelId, dt.UserId, nil, connectionID)
	draftJSON, jsonErr := json.Marshal(dt)
	if jsonErr != nil {
		rctx.Logger().Warn("Failed to encode draft to JSON", mlog.Err(jsonErr))
	}
	message.Add("draft", string(draftJSON))
	a.Publish(message)

	return dt, nil
}

func (a *App) GetDraftsForUser(rctx request.CTX, userID, teamID string) ([]*model.Draft, *model.AppError) {
	if !*a.Config().ServiceSettings.AllowSyncedDrafts {
		return nil, model.NewAppError("GetDraftsForUser", "app.draft.feature_disabled", nil, "", http.StatusNotImplemented)
	}

	drafts, err := a.Srv().Store().Draft().GetDraftsForUser(userID, teamID)

	if err != nil {
		return nil, model.NewAppError("GetDraftsForUser", "app.draft.get_drafts.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	for _, draft := range drafts {
		a.prepareDraftWithFileInfos(rctx, userID, draft)
	}
	return drafts, nil
}

func (a *App) prepareDraftWithFileInfos(rctx request.CTX, userID string, draft *model.Draft) *model.Draft {
	if fileInfos, err := a.getFileInfosForDraft(rctx, draft); err != nil {
		rctx.Logger().Error("Failed to get files for a user's drafts", mlog.String("user_id", userID), mlog.Err(err))
	} else {
		draft.Metadata = &model.PostMetadata{}
		draft.Metadata.Files = fileInfos
	}

	return draft
}

func (a *App) getFileInfosForDraft(rctx request.CTX, draft *model.Draft) ([]*model.FileInfo, *model.AppError) {
	if len(draft.FileIds) == 0 {
		return nil, nil
	}

	allFileInfos, err := a.Srv().Store().FileInfo().GetByIds(draft.FileIds, false, true)
	if err != nil {
		return nil, model.NewAppError("GetFileInfosForDraft", "app.draft.get_for_draft.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	fileInfos := []*model.FileInfo{}
	for _, fileInfo := range allFileInfos {
		if fileInfo.PostId == "" && fileInfo.CreatorId == draft.UserId {
			fileInfos = append(fileInfos, fileInfo)
		} else {
			rctx.Logger().Debug("Invalid file id in draft", mlog.String("file_id", fileInfo.Id), mlog.String("user_id", draft.UserId))
		}
	}

	if len(fileInfos) == 0 {
		return nil, nil
	}

	a.generateMiniPreviewForInfos(rctx, fileInfos)

	return fileInfos, nil
}

func (a *App) DeleteDraft(rctx request.CTX, draft *model.Draft, connectionID string) *model.AppError {
	if !*a.Config().ServiceSettings.AllowSyncedDrafts {
		return model.NewAppError("DeleteDraft", "app.draft.feature_disabled", nil, "", http.StatusNotImplemented)
	}

	if err := a.Srv().Store().Draft().Delete(draft.UserId, draft.ChannelId, draft.RootId); err != nil {
		return model.NewAppError("DeleteDraft", "app.draft.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	draftJSON, jsonErr := json.Marshal(draft)
	if jsonErr != nil {
		rctx.Logger().Warn("Failed to encode draft to JSON")
	}

	message := model.NewWebSocketEvent(model.WebsocketEventDraftDeleted, "", draft.ChannelId, draft.UserId, nil, connectionID)
	message.Add("draft", string(draftJSON))
	a.Publish(message)

	return nil
}

// CreateDraftForGuest creates a draft for a guest user
// Guest users can create drafts for channels they will be invited to
// This enables a smoother onboarding experience where guests can prepare messages
// before being added to channels
func (a *App) CreateDraftForGuest(rctx request.CTX, draft *model.Draft, connectionID string) (*model.Draft, *model.AppError) {
	if !*a.Config().ServiceSettings.AllowSyncedDrafts {
		return nil, model.NewAppError("CreateDraftForGuest", "app.draft.feature_disabled", nil, "", http.StatusNotImplemented)
	}

	// Validate user exists and is a guest
	user, nErr := a.Srv().Store().User().Get(context.Background(), draft.UserId)
	if nErr != nil {
		return nil, model.NewAppError("CreateDraftForGuest", "app.user.get.app_error", nil, "", http.StatusInternalServerError).Wrap(nErr)
	}

	if !user.IsGuest() {
		return nil, model.NewAppError("CreateDraftForGuest", "api.draft.create_guest_draft.not_guest.error", nil, "", http.StatusForbidden)
	}

	// Verify guest has access to the channel
	channel, errCh := a.Srv().Store().Channel().Get(draft.ChannelId, true)
	if errCh != nil {
		return nil, model.NewAppError("CreateDraftForGuest", "app.channel.get.app_error", nil, "", http.StatusInternalServerError).Wrap(errCh)
	}

	if channel.DeleteAt != 0 {
		return nil, model.NewAppError("CreateDraftForGuest", "api.channel.get_channel.deleted.error", nil, "", http.StatusBadRequest)
	}

	// Verify guest is a member of the channel
	_, err := a.Srv().Store().Channel().GetMember(rctx, draft.ChannelId, draft.UserId)
	if err != nil {
		return nil, model.NewAppError("CreateDraftForGuest", "api.draft.create_guest_draft.no_channel_access.error", nil, "", http.StatusForbidden).Wrap(err)
	}

	// Set guest flag for tracking
	draft.IsGuest = true

	// Store channel metadata in draft props for future reference
	props := draft.GetProps()
	if props == nil {
		props = make(map[string]any)
	}
	props["channel_name"] = channel.Name
	props["channel_type"] = channel.Type
	draft.SetProps(props)

	// Create the draft using standard flow
	return a.UpsertDraft(rctx, draft, connectionID)
}

// resolveDraftConflict handles conflict resolution for draft synchronization
// When multiple devices edit the same draft, we merge changes using a last-write-wins strategy
// with conflict markers for user review
func (a *App) resolveDraftConflict(rctx request.CTX, existing *model.Draft, incoming *model.Draft) (bool, *model.Draft) {
	// If no sync metadata, no conflict
	existingMeta := existing.GetSyncMetadata()
	incomingMeta := incoming.GetSyncMetadata()

	if existingMeta == nil || incomingMeta == nil {
		return false, incoming
	}

	// Same device, no conflict
	if existingMeta.DeviceId == incomingMeta.DeviceId {
		return false, incoming
	}

	// Check timestamps
	timeDiff := incomingMeta.SyncedAt - existingMeta.SyncedAt
	if timeDiff > 5000 { // More than 5 seconds apart, clear win
		return false, incoming
	}

	// Conflict detected - merge drafts
	rctx.Logger().Debug("Draft sync conflict detected",
		mlog.String("channel_id", existing.ChannelId),
		mlog.String("user_id", existing.UserId),
	)

	merged := incoming.Clone()
	conflictId := model.NewId()

	// Merge messages with conflict markers
	if existing.Message != incoming.Message {
		merged.Message = incoming.Message + "\n<<<<<<< Your device\n" + existing.Message + "\n======="
	}

	// Set conflict metadata
	meta := &model.DraftSyncMetadata{
		DeviceId:   incomingMeta.DeviceId,
		SyncedAt:   model.GetMillis(),
		ConflictId: conflictId,
	}
	merged.SetSyncMetadata(meta)

	return true, merged
}

// GetChannelInfoForDraft returns channel information for draft creation
// NOTE: This method does not perform authorization checks. Callers must verify
// that the user has permission to access the channel before calling this method.
// Deprecated: Use direct channel access with proper authorization instead.
func (a *App) GetChannelInfoForDraft(rctx request.CTX, channelId string) (map[string]string, *model.AppError) {
	channel, err := a.Srv().Store().Channel().Get(channelId, true)
	if err != nil {
		return nil, model.NewAppError("GetChannelInfoForDraft", "app.channel.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	if channel.DeleteAt != 0 {
		return nil, model.NewAppError("GetChannelInfoForDraft", "api.channel.get_channel.deleted.error", nil, "", http.StatusBadRequest)
	}

	info := map[string]string{
		"id":   channel.Id,
		"name": channel.Name,
		"type": channel.Type,
	}

	return info, nil
}
