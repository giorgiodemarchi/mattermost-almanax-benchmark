// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

// SearchRankingEngine handles relevance scoring and ranking for search results
type SearchRankingEngine struct {
	config *model.Config
}

// NewSearchRankingEngine creates a new search ranking engine
func NewSearchRankingEngine(config *model.Config) *SearchRankingEngine {
	return &SearchRankingEngine{
		config: config,
	}
}

// RankSearchResults applies relevance scoring to search results based on multiple factors
func (e *SearchRankingEngine) RankSearchResults(posts []*model.Post, searchTerms string, options *model.SearchRankingOptions) []*model.Post {
	if options == nil || !options.UseRelevanceScoring {
		return posts
	}

	// Calculate scores for each post
	scores := make(map[string]float64)
	for _, post := range posts {
		score := e.calculateRelevanceScore(post, searchTerms, options)
		scores[post.Id] = score
	}

	// Sort posts by score (descending)
	sort.Slice(posts, func(i, j int) bool {
		scoreI := scores[posts[i].Id]
		scoreJ := scores[posts[j].Id]
		if scoreI != scoreJ {
			return scoreI > scoreJ
		}
		// Fall back to creation time for ties
		return posts[i].CreateAt > posts[j].CreateAt
	})

	return posts
}

// calculateRelevanceScore computes a relevance score for a post based on multiple factors
func (e *SearchRankingEngine) calculateRelevanceScore(post *model.Post, searchTerms string, options *model.SearchRankingOptions) float64 {
	score := 0.0

	// Text match relevance (TF-IDF inspired)
	score += e.calculateTextMatchScore(post, searchTerms) * e.getWeight("text_match", options)

	// Recency boost
	if options.BoostRecent {
		score += e.calculateRecencyScore(post) * e.getWeight("recency", options)
	}

	// Engagement signals
	if options.BoostReactions {
		score += e.calculateEngagementScore(post) * e.getWeight("engagement", options)
	}

	// Thread importance (posts with more replies rank higher)
	score += e.calculateThreadImportanceScore(post) * e.getWeight("thread_importance", options)

	// Message length (prefer substantive posts)
	score += e.calculateLengthScore(post) * e.getWeight("length", options)

	return score
}

// calculateTextMatchScore computes score based on term frequency and position
func (e *SearchRankingEngine) calculateTextMatchScore(post *model.Post, searchTerms string) float64 {
	if searchTerms == "" {
		return 0.0
	}

	terms := strings.Fields(strings.ToLower(searchTerms))
	message := strings.ToLower(post.Message)
	score := 0.0

	for _, term := range terms {
		// Count occurrences
		count := strings.Count(message, term)
		if count > 0 {
			// TF component (logarithmic scaling to diminish returns)
			tf := 1.0 + math.Log(float64(count))
			score += tf

			// Boost for exact phrase matches
			if strings.Contains(message, term) {
				score += 0.5
			}

			// Position bonus: terms near the start score higher
			firstOccurrence := strings.Index(message, term)
			if firstOccurrence != -1 {
				positionBonus := 1.0 / (1.0 + float64(firstOccurrence)/100.0)
				score += positionBonus * 0.3
			}
		}
	}

	// Normalize by message length (prevent bias toward long posts)
	messageWords := len(strings.Fields(message))
	if messageWords > 0 {
		score = score / math.Sqrt(float64(messageWords))
	}

	return score
}

// calculateRecencyScore gives higher scores to recent posts
func (e *SearchRankingEngine) calculateRecencyScore(post *model.Post) float64 {
	now := time.Now().UnixMilli()
	ageInHours := float64(now-post.CreateAt) / (1000.0 * 3600.0)

	// Exponential decay: newer posts score much higher
	// Half-life of 24 hours
	halfLife := 24.0
	score := math.Exp(-ageInHours * math.Ln2 / halfLife)

	return score * 2.0 // Scale factor
}

// calculateEngagementScore based on reactions and other signals
func (e *SearchRankingEngine) calculateEngagementScore(post *model.Post) float64 {
	score := 0.0

	// Parse reactions from post metadata if available
	if reactions := post.GetProp("reactions"); reactions != nil {
		// Assuming reactions is a count or array
		score += 0.5 // Boost for having reactions
	}

	// Pinned posts are important
	if post.IsPinned {
		score += 1.0
	}

	return score
}

// calculateThreadImportanceScore based on reply count
func (e *SearchRankingEngine) calculateThreadImportanceScore(post *model.Post) float64 {
	// ReplyCount would need to be passed or fetched
	// For now, root posts get a small boost as they start conversations
	if post.RootId == "" {
		return 0.3
	}
	return 0.0
}

// calculateLengthScore prefers posts with substantive content
func (e *SearchRankingEngine) calculateLengthScore(post *model.Post) float64 {
	wordCount := len(strings.Fields(post.Message))

	// Optimal length: 20-200 words
	if wordCount < 5 {
		// Very short posts get penalized
		return -0.2
	} else if wordCount >= 5 && wordCount <= 200 {
		// Sweet spot
		return 0.3
	} else {
		// Very long posts get slight penalty
		return 0.1
	}
}

// getWeight retrieves the weight for a scoring factor
func (e *SearchRankingEngine) getWeight(factor string, options *model.SearchRankingOptions) float64 {
	if options.CustomWeights != nil {
		if weight, ok := options.CustomWeights[factor]; ok {
			return weight
		}
	}

	// Default weights
	defaults := map[string]float64{
		"text_match":        3.0,
		"recency":           1.5,
		"engagement":        1.0,
		"thread_importance": 0.8,
		"length":            0.5,
	}

	if weight, ok := defaults[factor]; ok {
		return weight
	}

	return 1.0
}

// ApplyCustomRankingWeights allows fine-tuning of ranking factors
func (e *SearchRankingEngine) ApplyCustomRankingWeights(options *model.SearchRankingOptions, adjustments map[string]float64) {
	if options.CustomWeights == nil {
		options.CustomWeights = make(map[string]float64)
	}

	for factor, weight := range adjustments {
		options.CustomWeights[factor] = weight
	}
}

// GetRankingStats returns statistics about ranking performance
func (e *SearchRankingEngine) GetRankingStats(posts []*model.Post, searchTerms string) map[string]interface{} {
	if len(posts) == 0 {
		return map[string]interface{}{
			"total_posts": 0,
		}
	}

	// Calculate distribution statistics
	avgMessageLength := 0
	for _, post := range posts {
		avgMessageLength += len(post.Message)
	}
	avgMessageLength /= len(posts)

	oldestPost := posts[0].CreateAt
	newestPost := posts[0].CreateAt
	for _, post := range posts {
		if post.CreateAt < oldestPost {
			oldestPost = post.CreateAt
		}
		if post.CreateAt > newestPost {
			newestPost = post.CreateAt
		}
	}

	return map[string]interface{}{
		"total_posts":         len(posts),
		"avg_message_length":  avgMessageLength,
		"time_span_hours":     float64(newestPost-oldestPost) / (1000.0 * 3600.0),
		"search_terms":        searchTerms,
		"ranking_applied":     true,
	}
}

// OptimizeRankingForUser personalizes ranking based on user behavior
// This would integrate with user analytics in a production system
func (e *SearchRankingEngine) OptimizeRankingForUser(userId string, options *model.SearchRankingOptions) *model.SearchRankingOptions {
	// Placeholder for user-specific optimizations
	// In production, this would:
	// - Analyze user's past search patterns
	// - Boost channels/users they interact with frequently
	// - Adjust recency weight based on their typical search timeframe
	
	if options == nil {
		options = &model.SearchRankingOptions{
			UseRelevanceScoring: true,
			BoostRecent:        true,
			BoostReactions:     true,
		}
	}

	return options
}

// ExplainRanking provides debugging information about why a post ranked where it did
func (e *SearchRankingEngine) ExplainRanking(post *model.Post, searchTerms string, options *model.SearchRankingOptions) map[string]float64 {
	explanation := make(map[string]float64)

	if options == nil {
		options = &model.SearchRankingOptions{UseRelevanceScoring: true}
	}

	explanation["text_match"] = e.calculateTextMatchScore(post, searchTerms) * e.getWeight("text_match", options)
	explanation["recency"] = e.calculateRecencyScore(post) * e.getWeight("recency", options)
	explanation["engagement"] = e.calculateEngagementScore(post) * e.getWeight("engagement", options)
	explanation["thread_importance"] = e.calculateThreadImportanceScore(post) * e.getWeight("thread_importance", options)
	explanation["length"] = e.calculateLengthScore(post) * e.getWeight("length", options)

	totalScore := 0.0
	for _, score := range explanation {
		totalScore += score
	}
	explanation["total"] = totalScore

	return explanation
}

