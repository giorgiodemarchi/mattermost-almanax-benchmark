// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

func TestCreateDelegatedBot(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should create delegated bot with valid parent", func(t *testing.T) {
		// Create a parent bot
		parentBot := &model.Bot{
			Username:    "parent-bot",
			DisplayName: "Parent Bot",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)
		require.NotNil(t, createdParent)

		// Create delegated bot
		req := &model.BotDelegationRequest{
			Username:       "child-bot",
			DisplayName:    "Child Bot",
			Description:    "Test child bot",
			DelegationType: model.BotDelegationTypeSubBot,
			Scopes:         []string{"posts:read", "posts:write"},
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.Nil(t, err)
		require.NotNil(t, delegatedBot)
		assert.Equal(t, req.Username, delegatedBot.Username)
		assert.Equal(t, createdParent.UserId, delegatedBot.ParentBotId)
		assert.Equal(t, model.BotDelegationTypeSubBot, delegatedBot.DelegationType)
	})

	t.Run("should fail with non-existent parent", func(t *testing.T) {
		req := &model.BotDelegationRequest{
			Username:    "orphan-bot",
			DisplayName: "Orphan Bot",
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, "invalid_parent_id", req)
		require.NotNil(t, err)
		require.Nil(t, delegatedBot)
	})

	t.Run("should fail with empty username", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:    "parent-bot2",
			DisplayName: "Parent Bot 2",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		req := &model.BotDelegationRequest{
			Username:    "", // Empty username
			DisplayName: "Invalid Bot",
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.NotNil(t, err)
		require.Nil(t, delegatedBot)
	})

	t.Run("should inherit owner from parent", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:    "parent-bot3",
			DisplayName: "Parent Bot 3",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		req := &model.BotDelegationRequest{
			Username:    "child-bot3",
			DisplayName: "Child Bot 3",
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.Nil(t, err)
		assert.Equal(t, createdParent.OwnerId, delegatedBot.OwnerId)
	})

	t.Run("should set default delegation type if not specified", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:    "parent-bot4",
			DisplayName: "Parent Bot 4",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		req := &model.BotDelegationRequest{
			Username:       "child-bot4",
			DisplayName:    "Child Bot 4",
			DelegationType: "", // Not specified
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.Nil(t, err)
		assert.Equal(t, model.BotDelegationTypeSubBot, delegatedBot.DelegationType)
	})
}

func TestGetDelegatedBots(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should return empty list for bot with no children", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:    "lonely-bot",
			DisplayName: "Lonely Bot",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		delegatedBots, err := th.App.GetDelegatedBots(th.Context, createdParent.UserId)
		require.Nil(t, err)
		assert.Equal(t, 0, len(delegatedBots))
	})

	t.Run("should return all delegated bots", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:    "parent-with-children",
			DisplayName: "Parent With Children",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		// Create multiple delegated bots
		for i := 0; i < 3; i++ {
			req := &model.BotDelegationRequest{
				Username:    model.NewId(),
				DisplayName: "Child Bot",
			}
			_, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
			require.Nil(t, err)
		}

		delegatedBots, err := th.App.GetDelegatedBots(th.Context, createdParent.UserId)
		require.Nil(t, err)
		assert.Equal(t, 3, len(delegatedBots))

		// Verify all bots have correct parent
		for _, bot := range delegatedBots {
			assert.Equal(t, createdParent.UserId, bot.ParentBotId)
		}
	})
}

func TestGetBotDelegationChain(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should return single bot for non-delegated bot", func(t *testing.T) {
		bot := &model.Bot{
			Username:    "standalone-bot",
			DisplayName: "Standalone Bot",
			OwnerId:     th.BasicUser.Id,
		}
		createdBot, err := th.App.CreateBot(th.Context, bot)
		require.Nil(t, err)

		chain, err := th.App.GetBotDelegationChain(th.Context, createdBot.UserId)
		require.Nil(t, err)
		assert.Equal(t, 1, len(chain))
		assert.Equal(t, createdBot.UserId, chain[0].UserId)
	})

	t.Run("should return full chain for delegated bot", func(t *testing.T) {
		// Create grandparent bot
		grandparent := &model.Bot{
			Username:    "grandparent-bot",
			DisplayName: "Grandparent Bot",
			OwnerId:     th.BasicUser.Id,
		}
		createdGrandparent, err := th.App.CreateBot(th.Context, grandparent)
		require.Nil(t, err)

		// Create parent bot
		parentReq := &model.BotDelegationRequest{
			Username:    "parent-in-chain",
			DisplayName: "Parent In Chain",
		}
		createdParent, err := th.App.CreateDelegatedBot(th.Context, createdGrandparent.UserId, parentReq)
		require.Nil(t, err)

		// Create child bot
		childReq := &model.BotDelegationRequest{
			Username:    "child-in-chain",
			DisplayName: "Child In Chain",
		}
		createdChild, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, childReq)
		require.Nil(t, err)

		// Get chain starting from child
		chain, err := th.App.GetBotDelegationChain(th.Context, createdChild.UserId)
		require.Nil(t, err)
		assert.Equal(t, 3, len(chain))

		// Verify chain order (child -> parent -> grandparent)
		assert.Equal(t, createdChild.UserId, chain[0].UserId)
		assert.Equal(t, createdParent.UserId, chain[1].UserId)
		assert.Equal(t, createdGrandparent.UserId, chain[2].UserId)
	})
}

func TestUpdateBotRoles(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should fail without system permission", func(t *testing.T) {
		bot := &model.Bot{
			Username:    "regular-bot",
			DisplayName: "Regular Bot",
			OwnerId:     th.BasicUser.Id,
		}
		createdBot, err := th.App.CreateBot(th.Context, bot)
		require.Nil(t, err)

		// Try to update roles without system permission
		newRoles := []string{model.SystemUserRoleId, model.SystemAdminRoleId}
		updatedBot, err := th.App.UpdateBotRoles(th.Context, createdBot.UserId, newRoles, false)

		// Should fail due to insufficient permissions
		require.NotNil(t, err)
		require.Nil(t, updatedBot)
	})

	t.Run("should succeed with system permission", func(t *testing.T) {
		bot := &model.Bot{
			Username:        "system-bot",
			DisplayName:     "System Bot",
			OwnerId:         th.BasicUser.Id,
			IsSystemManaged: true,
		}
		createdBot, err := th.App.CreateBot(th.Context, bot)
		require.Nil(t, err)

		// Update roles with system permission should work
		newRoles := []string{model.SystemUserRoleId, model.TeamUserRoleId}
		updatedBot, err := th.App.UpdateBotRoles(th.Context, createdBot.UserId, newRoles, true)

		require.Nil(t, err)
		require.NotNil(t, updatedBot)

		// Verify the user roles were updated
		user, appErr := th.App.GetUser(createdBot.UserId)
		require.Nil(t, appErr)
		assert.Contains(t, user.Roles, model.SystemUserRoleId)
		assert.Contains(t, user.Roles, model.TeamUserRoleId)
	})
}

func TestBotInheritance(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("should NOT inherit IsSystemManaged flag - security fix", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:        "parent-system-managed",
			DisplayName:     "Parent System Managed",
			OwnerId:         th.BasicUser.Id,
			IsSystemManaged: true, // Parent is system-managed
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)
		assert.True(t, createdParent.IsSystemManaged)

		req := &model.BotDelegationRequest{
			Username:    "child-not-inherit-system",
			DisplayName: "Child Should Not Inherit System Flag",
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.Nil(t, err)
		
		// SECURITY: IsSystemManaged should NOT be inherited to prevent privilege escalation
		assert.False(t, delegatedBot.IsSystemManaged, "IsSystemManaged should not be inherited")
	})

	t.Run("should inherit service account ID", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:         "parent-with-service-id",
			DisplayName:      "Parent With Service ID",
			OwnerId:          th.BasicUser.Id,
			ServiceAccountId: "external-service-123",
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		req := &model.BotDelegationRequest{
			Username:    "child-inherits-service-id",
			DisplayName: "Child Inherits Service ID",
		}

		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.Nil(t, err)
		assert.Equal(t, "external-service-123", delegatedBot.ServiceAccountId)
	})
}

func TestBotDelegationPermissions(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("parent can delegate bot creation", func(t *testing.T) {
		parentBot := &model.Bot{
			Username:    "delegating-parent",
			DisplayName: "Delegating Parent",
			OwnerId:     th.BasicUser.Id,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)
		assert.True(t, createdParent.CanDelegate())
	})

	t.Run("service account can delegate", func(t *testing.T) {
		serviceBot := &model.Bot{
			Username:       "service-account",
			DisplayName:    "Service Account",
			OwnerId:        th.BasicUser.Id,
			DelegationType: model.BotDelegationTypeService,
		}
		createdService, err := th.App.CreateBot(th.Context, serviceBot)
		require.Nil(t, err)
		assert.True(t, createdService.CanDelegate())
	})
}

// TestPrivilegeEscalationPrevention ensures the security fixes prevent
// privilege escalation through bot delegation
func TestPrivilegeEscalationPrevention(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	t.Run("prevent privilege escalation via IsSystemManaged inheritance", func(t *testing.T) {
		// Create a system-managed parent bot
		parentBot := &model.Bot{
			Username:        "system-parent",
			DisplayName:     "System Parent",
			OwnerId:         th.BasicUser.Id,
			IsSystemManaged: true,
		}
		createdParent, err := th.App.CreateBot(th.Context, parentBot)
		require.Nil(t, err)

		// Create delegated bot - should NOT inherit IsSystemManaged
		req := &model.BotDelegationRequest{
			Username:    "potential-escalation-bot",
			DisplayName: "Potential Escalation Bot",
		}
		delegatedBot, err := th.App.CreateDelegatedBot(th.Context, createdParent.UserId, req)
		require.Nil(t, err)

		// Verify IsSystemManaged was NOT inherited (security fix)
		assert.False(t, delegatedBot.IsSystemManaged, "delegated bot should not inherit system-managed status")

		// Attempt to escalate privileges should fail without system permission
		adminRoles := []string{model.SystemUserRoleId, model.SystemAdminRoleId}
		updatedBot, err := th.App.UpdateBotRoles(th.Context, delegatedBot.UserId, adminRoles, false)

		// Should fail due to insufficient permissions
		assert.NotNil(t, err, "role escalation should fail without system permission")
		assert.Nil(t, updatedBot)

		// Verify bot still has default roles (no escalation occurred)
		user, appErr := th.App.GetUser(delegatedBot.UserId)
		require.Nil(t, appErr)
		assert.NotContains(t, user.Roles, model.SystemAdminRoleId, "bot should not have admin role")
	})

	t.Run("plugin API cannot update bot roles", func(t *testing.T) {
		// This test verifies plugins cannot escalate bot privileges
		// The UpdateBotRoles method in plugin API should be disabled

		bot := &model.Bot{
			Username:    "plugin-bot",
			DisplayName: "Plugin Bot",
			OwnerId:     "plugin-id",
		}
		createdBot, err := th.App.CreateBot(th.Context, bot)
		require.Nil(t, err)

		// Simulate plugin API call - should fail
		pluginAPI := th.App.NewPluginAPI(th.Context, "plugin-id")
		updatedBot, err := pluginAPI.UpdateBotRoles(createdBot.UserId, []string{model.SystemAdminRoleId})

		// Should be blocked for security
		assert.NotNil(t, err, "plugin API role updates should be disabled")
		assert.Nil(t, updatedBot)
		assert.Contains(t, err.Error(), "disabled for security", "error should indicate security restriction")
	})
}

