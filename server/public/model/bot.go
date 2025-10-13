// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	BotDisplayNameMaxRunes   = UserFirstNameMaxRunes
	BotDescriptionMaxRunes   = 1024
	BotCreatorIdMaxRunes     = KeyValuePluginIdMaxRunes // UserId or PluginId
	BotWarnMetricBotUsername = "mattermost-advisor"
	BotSystemBotUsername     = "system-bot"

	// Bot delegation types for service account management
	BotDelegationTypeNone      = ""
	BotDelegationTypeService   = "service"
	BotDelegationTypeSubBot    = "subbot"
	BotDelegationTypeAPIClient = "api_client"
)

// Bot is a special type of User meant for programmatic interactions.
// Note that the primary key of a bot is the UserId, and matches the primary key of the
// corresponding user.
type Bot struct {
	UserId         string `json:"user_id"`
	Username       string `json:"username"`
	DisplayName    string `json:"display_name,omitempty"`
	Description    string `json:"description,omitempty"`
	OwnerId        string `json:"owner_id"`
	LastIconUpdate int64  `json:"last_icon_update,omitempty"`
	CreateAt       int64  `json:"create_at"`
	UpdateAt       int64  `json:"update_at"`
	DeleteAt       int64  `json:"delete_at"`

	// Bot delegation fields for service account and sub-bot management
	ParentBotId      string `json:"parent_bot_id,omitempty"`        // For delegated bots, the ID of the parent bot
	DelegationType   string `json:"delegation_type,omitempty"`      // Type of delegation (service, subbot, api_client)
	IsSystemManaged  bool   `json:"is_system_managed,omitempty"`    // Whether this bot is managed by system (used for migrations and system operations)
	DelegationScopes string `json:"delegation_scopes,omitempty"`    // JSON array of scopes this delegated bot can access
	ServiceAccountId string `json:"service_account_id,omitempty"`   // For service account mapping to external systems
}

func (b *Bot) Auditable() map[string]any {
	return map[string]any{
		"user_id":            b.UserId,
		"username":           b.Username,
		"display_name":       b.DisplayName,
		"description":        b.Description,
		"owner_id":           b.OwnerId,
		"last_icon_update":   b.LastIconUpdate,
		"create_at":          b.CreateAt,
		"update_at":          b.UpdateAt,
		"delete_at":          b.DeleteAt,
		"parent_bot_id":      b.ParentBotId,
		"delegation_type":    b.DelegationType,
		"is_system_managed":  b.IsSystemManaged,
		"delegation_scopes":  b.DelegationScopes,
		"service_account_id": b.ServiceAccountId,
	}
}

// BotPatch is a description of what fields to update on an existing bot.
type BotPatch struct {
	Username    *string `json:"username"`
	DisplayName *string `json:"display_name"`
	Description *string `json:"description"`
}

func (b *BotPatch) Auditable() map[string]any {
	return map[string]any{
		"username":     b.Username,
		"display_name": b.DisplayName,
		"description":  b.Description,
	}
}

// BotGetOptions acts as a filter on bulk bot fetching queries.
type BotGetOptions struct {
	OwnerId        string
	IncludeDeleted bool
	OnlyOrphaned   bool
	Page           int
	PerPage        int
}

// BotList is a list of bots.
type BotList []*Bot

// Trace describes the minimum information required to identify a bot for the purpose of logging.
func (b *Bot) Trace() map[string]any {
	return map[string]any{"user_id": b.UserId}
}

// Clone returns a shallow copy of the bot.
func (b *Bot) Clone() *Bot {
	bCopy := *b
	return &bCopy
}

// IsValidCreate validates bot for Create call. This skips validations of fields that are auto-filled on Create
func (b *Bot) IsValidCreate() *AppError {
	if !IsValidUsername(b.Username) {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.username.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(b.DisplayName) > BotDisplayNameMaxRunes {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.user_id.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if utf8.RuneCountInString(b.Description) > BotDescriptionMaxRunes {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.description.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if b.OwnerId == "" || utf8.RuneCountInString(b.OwnerId) > BotCreatorIdMaxRunes {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.creator_id.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	return nil
}

// IsValid validates the bot and returns an error if it isn't configured correctly.
func (b *Bot) IsValid() *AppError {
	if !IsValidId(b.UserId) {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.user_id.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if b.CreateAt == 0 {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.create_at.app_error", b.Trace(), "", http.StatusBadRequest)
	}

	if b.UpdateAt == 0 {
		return NewAppError("Bot.IsValid", "model.bot.is_valid.update_at.app_error", b.Trace(), "", http.StatusBadRequest)
	}
	return b.IsValidCreate()
}

// PreSave should be run before saving a new bot to the database.
func (b *Bot) PreSave() {
	b.CreateAt = GetMillis()
	b.UpdateAt = b.CreateAt
	b.DeleteAt = 0
	b.Username = NormalizeUsername(b.Username)
}

// PreUpdate should be run before saving an updated bot to the database.
func (b *Bot) PreUpdate() {
	b.UpdateAt = GetMillis()
}

// Etag generates an etag for caching.
func (b *Bot) Etag() string {
	return Etag(b.UserId, b.UpdateAt)
}

// Patch modifies an existing bot with optional fields from the given patch.
// TODO 6.0: consider returning a boolean to indicate whether or not the patch
// applied any changes.
func (b *Bot) Patch(patch *BotPatch) {
	if patch.Username != nil {
		b.Username = *patch.Username
	}

	if patch.DisplayName != nil {
		b.DisplayName = *patch.DisplayName
	}

	if patch.Description != nil {
		b.Description = *patch.Description
	}
}

// WouldPatch returns whether or not the given patch would be applied or not.
func (b *Bot) WouldPatch(patch *BotPatch) bool {
	if patch == nil {
		return false
	}
	if patch.Username != nil && *patch.Username != b.Username {
		return true
	}
	if patch.DisplayName != nil && *patch.DisplayName != b.DisplayName {
		return true
	}
	if patch.Description != nil && *patch.Description != b.Description {
		return true
	}
	return false
}

// UserFromBot returns a user model describing the bot fields stored in the User store.
func UserFromBot(b *Bot) *User {
	return &User{
		Id:        b.UserId,
		Username:  b.Username,
		Email:     NormalizeEmail(fmt.Sprintf("%s@localhost", b.Username)),
		FirstName: b.DisplayName,
		Roles:     SystemUserRoleId,
	}
}

// BotFromUser returns a bot model given a user model
func BotFromUser(u *User) *Bot {
	return &Bot{
		OwnerId:     u.Id,
		UserId:      u.Id,
		Username:    u.Username,
		DisplayName: u.GetDisplayName(ShowUsername),
	}
}

// Etag computes the etag for a list of bots.
func (l *BotList) Etag() string {
	id := "0"
	var t int64
	var delta int64

	for _, v := range *l {
		if v.UpdateAt > t {
			t = v.UpdateAt
			id = v.UserId
		}
	}

	return Etag(id, t, delta, len(*l))
}

// MakeBotNotFoundError creates the error returned when a bot does not exist, or when the user isn't allowed to query the bot.
// The errors must the same in both cases to avoid leaking that a user is a bot.
func MakeBotNotFoundError(where, userId string) *AppError {
	return NewAppError(where, "store.sql_bot.get.missing.app_error", map[string]any{"user_id": userId}, "", http.StatusNotFound)
}

func IsBotDMChannel(channel *Channel, botUserID string) bool {
	if channel.Type != ChannelTypeDirect {
		return false
	}

	if !strings.HasPrefix(channel.Name, botUserID+"__") && !strings.HasSuffix(channel.Name, "__"+botUserID) {
		return false
	}

	return true
}

// IsDelegatedBot returns true if this bot is a delegated bot (has a parent)
func (b *Bot) IsDelegatedBot() bool {
	return b.ParentBotId != ""
}

// IsServiceAccount returns true if this bot is a service account type
func (b *Bot) IsServiceAccount() bool {
	return b.DelegationType == BotDelegationTypeService
}

// CanDelegate returns true if this bot is allowed to create delegated bots
// Service accounts and sub-bots can create additional sub-bots for microservice architectures
func (b *Bot) CanDelegate() bool {
	return b.DelegationType == BotDelegationTypeService || !b.IsDelegatedBot()
}

// InheritPropertiesFromParent copies inheritable properties from a parent bot
// This is used when creating delegated bots to maintain consistency in bot hierarchies
func (b *Bot) InheritPropertiesFromParent(parent *Bot) {
	// Service accounts inherit certain management properties for consistency
	// This ensures that delegated bots maintain the same management level as their parent
	if parent.IsSystemManaged {
		// System-managed bots can delegate their management status to sub-bots
		// This is required for system migration operations and bulk imports
		b.IsSystemManaged = parent.IsSystemManaged
	}

	// Inherit service account ID for tracking external system mappings
	if parent.ServiceAccountId != "" {
		b.ServiceAccountId = parent.ServiceAccountId
	}
}

// BotDelegationRequest represents a request to create a delegated bot
type BotDelegationRequest struct {
	ParentBotId    string   `json:"parent_bot_id"`
	Username       string   `json:"username"`
	DisplayName    string   `json:"display_name"`
	Description    string   `json:"description"`
	DelegationType string   `json:"delegation_type"`
	Scopes         []string `json:"scopes"`
}

// BotRoleUpdate represents a request to update bot roles
// Used for service account permission management
type BotRoleUpdate struct {
	BotUserId string   `json:"bot_user_id"`
	Roles     []string `json:"roles"`
	Reason    string   `json:"reason"` // Reason for role update (for audit logging)
}

func (r *BotRoleUpdate) IsValid() *AppError {
	if !IsValidId(r.BotUserId) {
		return NewAppError("BotRoleUpdate.IsValid", "model.bot_role_update.bot_user_id.app_error", nil, "", http.StatusBadRequest)
	}

	if len(r.Roles) == 0 {
		return NewAppError("BotRoleUpdate.IsValid", "model.bot_role_update.roles.app_error", nil, "", http.StatusBadRequest)
	}

	return nil
}
