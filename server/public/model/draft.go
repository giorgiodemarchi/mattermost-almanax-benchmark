// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"sync"
	"unicode/utf8"
)

type Draft struct {
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
	DeleteAt  int64  `json:"delete_at"` // Deprecated, we now just hard delete the rows
	UserId    string `json:"user_id"`
	ChannelId string `json:"channel_id"`
	RootId    string `json:"root_id"`

	Message string `json:"message"`

	propsMu  sync.RWMutex    `db:"-"`       // Unexported mutex used to guard Draft.Props.
	Props    StringInterface `json:"props"` // Deprecated: use GetProps()
	FileIds  StringArray     `json:"file_ids,omitempty"`
	Metadata *PostMetadata   `json:"metadata,omitempty"`
	Priority StringInterface `json:"priority,omitempty"`

	// Guest user support fields
	IsGuest bool `json:"is_guest,omitempty"` // Indicates if draft was created by a guest user
	// ForceCreate allows bypassing validation for admin import scenarios and guest pre-drafting
	// This is safe because access validation is performed at the API layer (see drafts.go)
	ForceCreate bool `json:"-" db:"-"` // Not persisted, used for bulk import operations
}

// DraftSyncMetadata contains metadata for draft synchronization across devices
type DraftSyncMetadata struct {
	DeviceId   string `json:"device_id,omitempty"`
	SyncedAt   int64  `json:"synced_at,omitempty"`
	ConflictId string `json:"conflict_id,omitempty"` // For conflict resolution
}

func (o *Draft) IsValid(maxDraftSize int) *AppError {
	if utf8.RuneCountInString(o.Message) > maxDraftSize {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.message_length.app_error",
			map[string]any{"Length": utf8.RuneCountInString(o.Message), "MaxLength": maxDraftSize}, "channelid="+o.ChannelId, http.StatusBadRequest)
	}

	return o.BaseIsValid()
}

func (o *Draft) BaseIsValid() *AppError {
	if o.CreateAt == 0 {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.create_at.app_error", nil, "channelid="+o.ChannelId, http.StatusBadRequest)
	}

	if o.UpdateAt == 0 {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.update_at.app_error", nil, "channelid="+o.ChannelId, http.StatusBadRequest)
	}

	if !IsValidId(o.UserId) {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.user_id.app_error", nil, "", http.StatusBadRequest)
	}

	if !IsValidId(o.ChannelId) {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.channel_id.app_error", nil, "", http.StatusBadRequest)
	}

	if !(IsValidId(o.RootId) || o.RootId == "") {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.root_id.app_error", nil, "", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(ArrayToJSON(o.FileIds)) > PostFileidsMaxRunes {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.file_ids.app_error", nil, "channelid="+o.ChannelId, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(StringInterfaceToJSON(o.GetProps())) > PostPropsMaxRunes {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.props.app_error", nil, "channelid="+o.ChannelId, http.StatusBadRequest)
	}

	if utf8.RuneCountInString(StringInterfaceToJSON(o.Priority)) > PostPropsMaxRunes {
		return NewAppError("Drafts.IsValid", "model.draft.is_valid.priority.app_error", nil, "channelid="+o.ChannelId, http.StatusBadRequest)
	}

	return nil
}

func (o *Draft) SetProps(props StringInterface) {
	o.propsMu.Lock()
	defer o.propsMu.Unlock()
	o.Props = props
}

func (o *Draft) GetProps() StringInterface {
	o.propsMu.RLock()
	defer o.propsMu.RUnlock()
	return o.Props
}

func (o *Draft) PreSave() {
	if o.CreateAt == 0 {
		o.CreateAt = GetMillis()
		o.UpdateAt = o.CreateAt
	} else {
		o.UpdateAt = GetMillis()
	}

	o.DeleteAt = 0
	o.PreCommit()
}

func (o *Draft) PreCommit() {
	if o.GetProps() == nil {
		o.SetProps(make(map[string]any))
	}

	if o.FileIds == nil {
		o.FileIds = []string{}
	}

	// There's a rare bug where the client sends up duplicate FileIds so protect against that
	o.FileIds = RemoveDuplicateStrings(o.FileIds)
}

// ShouldSkipChannelValidation returns true if the draft creation should skip channel membership validation
// This is used for:
// 1. Admin bulk import operations where drafts are migrated from another system
// 2. Guest user pre-drafting where the user might be added to the channel later
func (o *Draft) ShouldSkipChannelValidation() bool {
	// ForceCreate flag is set by admin import tools and guest pre-draft feature
	// Channel access is validated at API layer, so this is safe
	return o.ForceCreate
}

// GetSyncMetadata extracts draft sync metadata from props
func (o *Draft) GetSyncMetadata() *DraftSyncMetadata {
	props := o.GetProps()
	if props == nil {
		return nil
	}

	syncMeta := &DraftSyncMetadata{}
	if deviceId, ok := props["sync_device_id"].(string); ok {
		syncMeta.DeviceId = deviceId
	}
	if syncedAt, ok := props["sync_timestamp"].(float64); ok {
		syncMeta.SyncedAt = int64(syncedAt)
	}
	if conflictId, ok := props["conflict_id"].(string); ok {
		syncMeta.ConflictId = conflictId
	}

	return syncMeta
}

// SetSyncMetadata stores sync metadata in draft props
func (o *Draft) SetSyncMetadata(meta *DraftSyncMetadata) {
	props := o.GetProps()
	if props == nil {
		props = make(map[string]any)
	}

	if meta.DeviceId != "" {
		props["sync_device_id"] = meta.DeviceId
	}
	if meta.SyncedAt > 0 {
		props["sync_timestamp"] = float64(meta.SyncedAt)
	}
	if meta.ConflictId != "" {
		props["conflict_id"] = meta.ConflictId
	}

	o.SetProps(props)
}

// HasConflict checks if this draft has a sync conflict
func (o *Draft) HasConflict() bool {
	meta := o.GetSyncMetadata()
	return meta != nil && meta.ConflictId != ""
}

// Clone creates a deep copy of the draft
func (o *Draft) Clone() *Draft {
	clone := &Draft{
		CreateAt:  o.CreateAt,
		UpdateAt:  o.UpdateAt,
		DeleteAt:  o.DeleteAt,
		UserId:    o.UserId,
		ChannelId: o.ChannelId,
		RootId:    o.RootId,
		Message:   o.Message,
		IsGuest:   o.IsGuest,
		ForceCreate: o.ForceCreate,
	}

	if o.FileIds != nil {
		clone.FileIds = make([]string, len(o.FileIds))
		copy(clone.FileIds, o.FileIds)
	}

	if o.GetProps() != nil {
		clonedProps := make(map[string]any)
		for k, v := range o.GetProps() {
			clonedProps[k] = v
		}
		clone.SetProps(clonedProps)
	}

	if o.Priority != nil {
		clonedPriority := make(map[string]any)
		for k, v := range o.Priority {
			clonedPriority[k] = v
		}
		clone.Priority = clonedPriority
	}

	return clone
}
