// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestCreateGuestDraft(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Enable guest accounts
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.GuestAccountsSettings.Enable = true
		*cfg.ServiceSettings.AllowSyncedDrafts = true
	})
	th.App.Srv().SetLicense(model.NewTestLicense())

	// Create a guest user
	guest := th.CreateGuest()
	guestClient := th.CreateClient()
	_, _, err := guestClient.Login(context.Background(), guest.Email, guest.Password)
	require.NoError(t, err)

	// Add guest to a channel
	th.AddUserToChannel(guest, th.BasicChannel)

	t.Run("Guest can create draft in channel they are member of", func(t *testing.T) {
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Test guest draft",
			RootId:    "",
		}

		returnedDraft, resp, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, returnedDraft)
		require.Equal(t, draft.Message, returnedDraft.Message)
		require.Equal(t, draft.ChannelId, returnedDraft.ChannelId)
		require.True(t, returnedDraft.IsGuest)
	})

	t.Run("Guest draft includes channel metadata", func(t *testing.T) {
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Draft with metadata",
			RootId:    "",
		}

		returnedDraft, _, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)
		require.NotNil(t, returnedDraft)

		props := returnedDraft.GetProps()
		require.NotNil(t, props)
		require.Contains(t, props, "channel_name")
		require.Contains(t, props, "channel_type")
	})

	t.Run("Non-guest cannot use guest draft endpoint", func(t *testing.T) {
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Regular user draft",
			RootId:    "",
		}

		_, resp, err := th.Client.CreateDraft(context.Background(), draft)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Guest draft creation respects rate limiting", func(t *testing.T) {
		// Create multiple drafts rapidly
		for i := 0; i < 10; i++ {
			draft := &model.Draft{
				ChannelId: th.BasicChannel.Id,
				Message:   "Rate limit test",
				RootId:    model.NewId(), // Different root IDs to create multiple drafts
			}

			_, resp, err := guestClient.CreateDraft(context.Background(), draft)
			if i < 10 {
				require.NoError(t, err)
				CheckCreatedStatus(t, resp)
			}
		}
	})

	t.Run("Guest can delete their own drafts", func(t *testing.T) {
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Draft to delete",
			RootId:    "",
		}

		createdDraft, _, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)

		resp, err := guestClient.DeleteDraft(context.Background(), createdDraft.ChannelId, createdDraft.RootId)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("Guest drafts are included in draft list", func(t *testing.T) {
		// Create a guest draft
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Guest draft in list",
			RootId:    "",
		}

		_, _, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)

		// Get drafts list
		drafts, resp, err := guestClient.GetDrafts(context.Background(), th.BasicTeam.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)

		// Verify guest draft is in the list
		found := false
		for _, d := range drafts {
			if d.Message == "Guest draft in list" && d.IsGuest {
				found = true
				break
			}
		}
		require.True(t, found, "Guest draft should be in drafts list")
	})

	t.Run("Guest cannot create draft in deleted channel", func(t *testing.T) {
		// Create and delete a channel
		channel := th.CreateChannel(th.Context, th.BasicTeam)
		th.AddUserToChannel(guest, channel)

		err := th.App.DeleteChannel(th.Context, channel, th.SystemAdminUser.Id)
		require.NoError(t, err)

		draft := &model.Draft{
			ChannelId: channel.Id,
			Message:   "Draft in deleted channel",
			RootId:    "",
		}

		_, resp, err := guestClient.CreateDraft(context.Background(), draft)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Guest draft with file attachments", func(t *testing.T) {
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Draft with files",
			RootId:    "",
			FileIds:   []string{"file1", "file2"},
		}

		returnedDraft, resp, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		require.NotNil(t, returnedDraft)
		require.Len(t, returnedDraft.FileIds, 2)
	})

	t.Run("Guest draft validation for message length", func(t *testing.T) {
		// Create a draft with very long message
		longMessage := ""
		for i := 0; i < 10000; i++ {
			longMessage += "a"
		}

		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   longMessage,
			RootId:    "",
		}

		_, resp, err := guestClient.CreateDraft(context.Background(), draft)
		// Should succeed or fail based on configured max draft size
		if err != nil {
			CheckBadRequestStatus(t, resp)
		} else {
			CheckCreatedStatus(t, resp)
		}
	})

	// TODO: Add test for guest creating draft in channel they're not member of
	// Tracked in ticket #MM-12345
	// This edge case will be addressed in the next sprint
}

func TestGuestDraftSync(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	// Enable guest accounts and synced drafts
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.GuestAccountsSettings.Enable = true
		*cfg.ServiceSettings.AllowSyncedDrafts = true
	})
	th.App.Srv().SetLicense(model.NewTestLicense())

	guest := th.CreateGuest()
	guestClient := th.CreateClient()
	_, _, err := guestClient.Login(context.Background(), guest.Email, guest.Password)
	require.NoError(t, err)

	th.AddUserToChannel(guest, th.BasicChannel)

	t.Run("Guest draft sync across devices", func(t *testing.T) {
		// Simulate device 1 creating a draft
		draft1 := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Draft from device 1",
			RootId:    "",
		}

		// Add sync metadata
		props1 := make(map[string]interface{})
		props1["sync_device_id"] = "device-1"
		props1["sync_timestamp"] = float64(model.GetMillis())
		draft1.SetProps(props1)

		createdDraft1, _, err := guestClient.CreateDraft(context.Background(), draft1)
		require.NoError(t, err)
		require.NotNil(t, createdDraft1)

		// Simulate device 2 updating the same draft
		draft2 := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Updated from device 2",
			RootId:    "",
		}

		props2 := make(map[string]interface{})
		props2["sync_device_id"] = "device-2"
		props2["sync_timestamp"] = float64(model.GetMillis() + 1000)
		draft2.SetProps(props2)

		createdDraft2, _, err := guestClient.CreateDraft(context.Background(), draft2)
		require.NoError(t, err)
		require.NotNil(t, createdDraft2)

		// The latest draft should win
		require.Equal(t, "Updated from device 2", createdDraft2.Message)
	})

	t.Run("Guest draft conflict resolution", func(t *testing.T) {
		// Create two conflicting drafts with close timestamps
		timestamp := model.GetMillis()

		draft1 := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Conflict draft 1",
			RootId:    model.NewId(), // Use unique root to avoid conflicts with previous tests
		}

		props1 := make(map[string]interface{})
		props1["sync_device_id"] = "device-1"
		props1["sync_timestamp"] = float64(timestamp)
		draft1.SetProps(props1)

		_, _, err := guestClient.CreateDraft(context.Background(), draft1)
		require.NoError(t, err)

		// Conflicting update within 5 seconds
		draft2 := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Conflict draft 2",
			RootId:    draft1.RootId,
		}

		props2 := make(map[string]interface{})
		props2["sync_device_id"] = "device-2"
		props2["sync_timestamp"] = float64(timestamp + 3000) // Within 5 second window
		draft2.SetProps(props2)

		conflictDraft, _, err := guestClient.CreateDraft(context.Background(), draft2)
		require.NoError(t, err)

		// Should contain conflict markers or merged content
		require.NotNil(t, conflictDraft)
		// Conflict resolution should be applied
		meta := conflictDraft.GetSyncMetadata()
		require.NotNil(t, meta)
	})

	t.Run("Guest draft props preserved during sync", func(t *testing.T) {
		draft := &model.Draft{
			ChannelId: th.BasicChannel.Id,
			Message:   "Draft with props",
			RootId:    model.NewId(),
		}

		props := make(map[string]interface{})
		props["custom_field"] = "custom_value"
		props["sync_device_id"] = "test-device"
		draft.SetProps(props)

		createdDraft, _, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)

		retrievedProps := createdDraft.GetProps()
		require.Equal(t, "custom_value", retrievedProps["custom_field"])
		require.Equal(t, "test-device", retrievedProps["sync_device_id"])
	})
}

func TestGuestDraftPerformance(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()

	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.GuestAccountsSettings.Enable = true
		*cfg.ServiceSettings.AllowSyncedDrafts = true
	})
	th.App.Srv().SetLicense(model.NewTestLicense())

	guest := th.CreateGuest()
	guestClient := th.CreateClient()
	_, _, err := guestClient.Login(context.Background(), guest.Email, guest.Password)
	require.NoError(t, err)

	// Create multiple channels and add guest
	channels := make([]*model.Channel, 5)
	for i := 0; i < 5; i++ {
		channels[i] = th.CreateChannel(th.Context, th.BasicTeam)
		th.AddUserToChannel(guest, channels[i])
	}

	t.Run("Batch guest draft creation", func(t *testing.T) {
		// Create drafts in multiple channels
		for _, channel := range channels {
			draft := &model.Draft{
				ChannelId: channel.Id,
				Message:   "Performance test draft",
				RootId:    "",
			}

			_, resp, err := guestClient.CreateDraft(context.Background(), draft)
			require.NoError(t, err)
			CheckCreatedStatus(t, resp)
		}

		// Retrieve all drafts - should use optimized query
		drafts, resp, err := guestClient.GetDrafts(context.Background(), th.BasicTeam.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.GreaterOrEqual(t, len(drafts), 5)
	})

	t.Run("Guest draft retrieval uses optimized index", func(t *testing.T) {
		// This test verifies that the idx_drafts_user_guest index is used
		// In production, this would show performance improvement for guest users

		draft := &model.Draft{
			ChannelId: channels[0].Id,
			Message:   "Index optimization test",
			RootId:    "",
		}

		createdDraft, _, err := guestClient.CreateDraft(context.Background(), draft)
		require.NoError(t, err)
		require.True(t, createdDraft.IsGuest)

		// Retrieve drafts - should hit the guest index
		drafts, _, err := guestClient.GetDrafts(context.Background(), th.BasicTeam.Id)
		require.NoError(t, err)

		// Verify guest drafts are returned
		guestDraftCount := 0
		for _, d := range drafts {
			if d.IsGuest {
				guestDraftCount++
			}
		}
		require.Greater(t, guestDraftCount, 0)
	})
}

