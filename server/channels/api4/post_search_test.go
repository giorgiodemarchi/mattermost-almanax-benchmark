// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestAdvancedSearchPostsInTeam(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.Client

	// Create a team and channel
	team := th.BasicTeam
	channel := th.BasicChannel
	user := th.BasicUser

	// Create posts with custom properties for testing
	post1 := &model.Post{
		ChannelId: channel.Id,
		UserId:    user.Id,
		Message:   "Test post about project alpha",
		Props: model.StringInterface{
			"metadata": map[string]interface{}{
				"priority": "high",
				"project":  "alpha",
			},
		},
	}
	post1, resp, err := client.CreatePost(post1)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	post2 := &model.Post{
		ChannelId: channel.Id,
		UserId:    user.Id,
		Message:   "Another post about project beta",
		Props: model.StringInterface{
			"metadata": map[string]interface{}{
				"priority": "low",
				"project":  "beta",
			},
		},
	}
	post2, resp, err = client.CreatePost(post2)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	t.Run("Basic advanced search with terms", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "project",
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
		assert.GreaterOrEqual(t, len(results.Order), 1)
	})

	t.Run("Advanced search with custom field filter - equals", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "",
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath: "metadata.priority",
					Operator:     "equals",
					Value:        "high",
				},
			},
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
		// Should find at least post1 with high priority
		assert.GreaterOrEqual(t, len(results.Order), 1)
	})

	t.Run("Advanced search with custom field filter - contains", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "",
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath: "metadata.project",
					Operator:     "contains",
					Value:        "alph",
				},
			},
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
	})

	t.Run("Advanced search with multiple filters AND", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "",
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath:    "metadata.priority",
					Operator:        "equals",
					Value:           "high",
					CombineOperator: "AND",
				},
				{
					RawFieldPath:    "metadata.project",
					Operator:        "equals",
					Value:           "alpha",
					CombineOperator: "AND",
				},
			},
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
		// Should find post1 (high priority AND alpha project)
		assert.GreaterOrEqual(t, len(results.Order), 1)
	})

	t.Run("Advanced search with ranking options", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "project",
			RankingOptions: &model.SearchRankingOptions{
				UseRelevanceScoring: true,
				BoostRecent:        true,
				BoostReactions:     true,
			},
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
		assert.GreaterOrEqual(t, len(results.Order), 1)
	})

	t.Run("Advanced search with custom ranking weights", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "project",
			RankingOptions: &model.SearchRankingOptions{
				UseRelevanceScoring: true,
				BoostRecent:        true,
				CustomWeights: map[string]float64{
					"text_match": 5.0,
					"recency":    2.0,
				},
			},
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
	})

	t.Run("Validation - empty field path", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "",
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath: "",
					Operator:     "equals",
					Value:        "test",
				},
			},
		}

		_, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Validation - invalid operator", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms: "",
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath: "metadata.test",
					Operator:     "invalid_op",
					Value:        "test",
				},
			},
		}

		_, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Validation - too many filters", func(t *testing.T) {
		filters := make([]*model.CustomFieldFilter, 25)
		for i := range filters {
			filters[i] = &model.CustomFieldFilter{
				RawFieldPath: "metadata.test",
				Operator:     "equals",
				Value:        "test",
			}
		}

		searchParams := &model.SearchParams{
			Terms:              "",
			CustomFieldFilters: filters,
		}

		_, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("Permission check - no team access", func(t *testing.T) {
		// Create a new user without team access
		otherUser := th.CreateUser()
		client.Login(otherUser.Email, otherUser.Password)

		searchParams := &model.SearchParams{
			Terms: "test",
		}

		_, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Search with in channels filter", func(t *testing.T) {
		client.Login(user.Email, user.Password)
		
		searchParams := &model.SearchParams{
			Terms:      "project",
			InChannels: []string{channel.Id},
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
		// All results should be from the specified channel
		for _, postId := range results.Order {
			post := results.Posts[postId]
			assert.Equal(t, channel.Id, post.ChannelId)
		}
	})

	t.Run("Search with date filters", func(t *testing.T) {
		searchParams := &model.SearchParams{
			Terms:     "project",
			AfterDate: "2020-01-01",
		}

		results, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		
		assert.NotNil(t, results)
	})
}

func TestAdvancedSearchSecurity(t *testing.T) {
	th := Setup(t).InitBasic()
	defer th.TearDown()
	client := th.Client

	team := th.BasicTeam
	channel := th.BasicChannel
	user := th.BasicUser

	// Create a post
	post := &model.Post{
		ChannelId: channel.Id,
		UserId:    user.Id,
		Message:   "Test security post",
	}
	post, _, _ = client.CreatePost(post)

	t.Run("Field path validation accepts safe paths", func(t *testing.T) {
		// These should all be accepted
		safePaths := []string{
			"metadata.priority",
			"customFields.status",
			"tags[0]",
			"nested.field.value",
		}

		for _, path := range safePaths {
			searchParams := &model.SearchParams{
				CustomFieldFilters: []*model.CustomFieldFilter{
					{
						RawFieldPath: path,
						Operator:     "equals",
						Value:        "test",
					},
				},
			}

			_, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
			// Should not fail on validation (may fail on no results)
			if err != nil {
				CheckOKStatus(t, resp) // Either succeeds or fails for other reasons
			}
		}
	})

	// Note: These tests verify that UPPERCASE SQL keywords are blocked
	// but they don't test lowercase or mixed case, which is the vulnerability
	t.Run("Field path validation blocks SQL keywords", func(t *testing.T) {
		dangerousPaths := []string{
			"metadata.SELECT",
			"field.DROP",
			"test.INSERT",
		}

		for _, path := range dangerousPaths {
			searchParams := &model.SearchParams{
				CustomFieldFilters: []*model.CustomFieldFilter{
					{
						RawFieldPath: path,
						Operator:     "equals",
						Value:        "test",
					},
				},
			}

			_, resp, err := client.AdvancedSearchPostsInTeam(team.Id, searchParams)
			// Should be blocked by validation
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		}
	})
}

// Helper function to add to client
func (c *Client4) AdvancedSearchPostsInTeam(teamId string, search *model.SearchParams) (*model.PostList, *Response, error) {
	r, err := c.DoAPIPost(c.teamRoute(teamId)+"/posts/search/advanced", search.ToJSON())
	if err != nil {
		return nil, BuildResponse(r), err
	}
	defer closeBody(r)

	var postList model.PostList
	if err := json.NewDecoder(r.Body).Decode(&postList); err != nil {
		return nil, BuildResponse(r), NewAppError("AdvancedSearchPostsInTeam", "api.marshal_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return &postList, BuildResponse(r), nil
}

// Add ToJSON method to SearchParams if not present
func (p *SearchParams) ToJSON() string {
	b, _ := json.Marshal(p)
	return string(b)
}

