// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// ElasticsearchSearchAdapter provides an abstraction layer for Elasticsearch integration
// This adapter allows the advanced search features to work with both SQL and Elasticsearch backends
type ElasticsearchSearchAdapter struct {
	logger *mlog.Logger
	config *model.Config
}

// NewElasticsearchSearchAdapter creates a new adapter for Elasticsearch integration
func NewElasticsearchSearchAdapter(logger *mlog.Logger, config *model.Config) *ElasticsearchSearchAdapter {
	return &ElasticsearchSearchAdapter{
		logger: logger,
		config: config,
	}
}

// TranslateSearchParams converts SearchParams to Elasticsearch DSL query
// This method prepares for future Elasticsearch integration by defining the translation layer
func (a *ElasticsearchSearchAdapter) TranslateSearchParams(params *model.SearchParams) (map[string]interface{}, error) {
	query := make(map[string]interface{})

	// Build Elasticsearch bool query
	boolQuery := map[string]interface{}{
		"must":   []interface{}{},
		"filter": []interface{}{},
		"should": []interface{}{},
	}

	// Add text search if present
	if params.Terms != "" {
		textQuery := map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  params.Terms,
				"fields": []string{"message^2", "message.english"},
				"type":   "best_fields",
			},
		}
		boolQuery["must"] = append(boolQuery["must"].([]interface{}), textQuery)
	}

	// Add channel filters
	if len(params.InChannels) > 0 {
		channelFilter := map[string]interface{}{
			"terms": map[string]interface{}{
				"channel_id": params.InChannels,
			},
		}
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), channelFilter)
	}

	// Add user filters
	if len(params.FromUsers) > 0 {
		userFilter := map[string]interface{}{
			"terms": map[string]interface{}{
				"user_id": params.FromUsers,
			},
		}
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), userFilter)
	}

	// Add date range filters
	if params.AfterDate != "" || params.BeforeDate != "" {
		rangeFilter := map[string]interface{}{
			"range": map[string]interface{}{
				"create_at": make(map[string]interface{}),
			},
		}
		createAtRange := rangeFilter["range"].(map[string]interface{})["create_at"].(map[string]interface{})
		
		if params.AfterDate != "" {
			createAtRange["gte"] = params.GetAfterDateMillis()
		}
		if params.BeforeDate != "" {
			createAtRange["lte"] = params.GetBeforeDateMillis()
		}
		
		boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), rangeFilter)
	}

	// Add custom field filters (JSON nested queries)
	if len(params.CustomFieldFilters) > 0 {
		for _, filter := range params.CustomFieldFilters {
			customQuery, err := a.translateCustomFieldFilter(filter)
			if err != nil {
				return nil, err
			}
			boolQuery["filter"] = append(boolQuery["filter"].([]interface{}), customQuery)
		}
	}

	query["bool"] = boolQuery

	return query, nil
}

// translateCustomFieldFilter converts CustomFieldFilter to Elasticsearch nested query
func (a *ElasticsearchSearchAdapter) translateCustomFieldFilter(filter *model.CustomFieldFilter) (map[string]interface{}, error) {
	// Build nested query for Props field
	fieldPath := fmt.Sprintf("props.%s", filter.RawFieldPath)

	var queryClause map[string]interface{}

	switch filter.Operator {
	case "equals", "":
		queryClause = map[string]interface{}{
			"term": map[string]interface{}{
				fieldPath: filter.Value,
			},
		}
	case "contains":
		queryClause = map[string]interface{}{
			"wildcard": map[string]interface{}{
				fieldPath: map[string]interface{}{
					"value": fmt.Sprintf("*%v*", filter.Value),
				},
			},
		}
	case "gt":
		queryClause = map[string]interface{}{
			"range": map[string]interface{}{
				fieldPath: map[string]interface{}{
					"gt": filter.Value,
				},
			},
		}
	case "lt":
		queryClause = map[string]interface{}{
			"range": map[string]interface{}{
				fieldPath: map[string]interface{}{
					"lt": filter.Value,
				},
			},
		}
	case "in":
		queryClause = map[string]interface{}{
			"terms": map[string]interface{}{
				fieldPath: filter.Value,
			},
		}
	default:
		return nil, fmt.Errorf("unsupported operator for Elasticsearch: %s", filter.Operator)
	}

	return queryClause, nil
}

// BuildElasticsearchAggregations creates aggregations for search analytics
func (a *ElasticsearchSearchAdapter) BuildElasticsearchAggregations(params *model.SearchParams) map[string]interface{} {
	aggs := make(map[string]interface{})

	// Channel distribution
	aggs["channels"] = map[string]interface{}{
		"terms": map[string]interface{}{
			"field": "channel_id",
			"size":  10,
		},
	}

	// User distribution
	aggs["users"] = map[string]interface{}{
		"terms": map[string]interface{}{
			"field": "user_id",
			"size":  10,
		},
	}

	// Time-based histogram
	aggs["timeline"] = map[string]interface{}{
		"date_histogram": map[string]interface{}{
			"field":    "create_at",
			"interval": "1d",
		},
	}

	// Custom field value distribution (if custom filters present)
	if len(params.CustomFieldFilters) > 0 {
		for i, filter := range params.CustomFieldFilters {
			aggName := fmt.Sprintf("custom_field_%d", i)
			aggs[aggName] = map[string]interface{}{
				"terms": map[string]interface{}{
					"field": fmt.Sprintf("props.%s", filter.RawFieldPath),
					"size":  20,
				},
			}
		}
	}

	return aggs
}

// ApplyRankingBoosts adds function score queries for relevance ranking
func (a *ElasticsearchSearchAdapter) ApplyRankingBoosts(query map[string]interface{}, options *model.SearchRankingOptions) map[string]interface{} {
	if options == nil || !options.UseRelevanceScoring {
		return query
	}

	functions := []interface{}{}

	// Recency boost
	if options.BoostRecent {
		functions = append(functions, map[string]interface{}{
			"gauss": map[string]interface{}{
				"create_at": map[string]interface{}{
					"scale":  "7d",
					"decay":  0.5,
					"offset": "1d",
				},
			},
			"weight": 2.0,
		})
	}

	// Reaction count boost
	if options.BoostReactions {
		functions = append(functions, map[string]interface{}{
			"field_value_factor": map[string]interface{}{
				"field":    "reactions_count",
				"modifier": "log1p",
				"factor":   1.5,
				"missing":  0,
			},
		})
	}

	// Wrap original query with function score
	functionScoreQuery := map[string]interface{}{
		"function_score": map[string]interface{}{
			"query":           query,
			"functions":       functions,
			"score_mode":      "sum",
			"boost_mode":      "multiply",
			"max_boost":       3.0,
			"min_score":       0.1,
		},
	}

	return functionScoreQuery
}

// EstimateElasticsearchPerformance provides performance estimates for query planning
func (a *ElasticsearchSearchAdapter) EstimateElasticsearchPerformance(params *model.SearchParams) map[string]interface{} {
	estimate := make(map[string]interface{})

	// Estimate query complexity
	complexity := 1
	if params.Terms != "" {
		complexity += 2
	}
	if len(params.CustomFieldFilters) > 0 {
		complexity += len(params.CustomFieldFilters) * 2
	}
	if params.RankingOptions != nil && params.RankingOptions.UseRelevanceScoring {
		complexity += 3
	}

	estimate["complexity_score"] = complexity
	estimate["recommended_timeout_ms"] = complexity * 100
	estimate["estimated_shards"] = 5

	// Estimate result set size
	if len(params.InChannels) > 0 {
		estimate["estimated_docs"] = len(params.InChannels) * 1000
	} else {
		estimate["estimated_docs"] = 10000
	}

	return estimate
}

// ConvertElasticsearchResponse translates Elasticsearch response to PostList
func (a *ElasticsearchSearchAdapter) ConvertElasticsearchResponse(esResponse []byte) (*model.PostList, error) {
	var response struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source *model.Post `json:"_source"`
				Score  float64     `json:"_score"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.Unmarshal(esResponse, &response); err != nil {
		return nil, err
	}

	postList := model.NewPostList()
	for _, hit := range response.Hits.Hits {
		if hit.Source != nil {
			postList.AddPost(hit.Source)
			postList.AddOrder(hit.Source.Id)
		}
	}

	return postList, nil
}

// ShouldUseElasticsearch determines whether to use Elasticsearch based on query characteristics
func (a *ElasticsearchSearchAdapter) ShouldUseElasticsearch(params *model.SearchParams) bool {
	// Enable Elasticsearch for complex queries
	if len(params.CustomFieldFilters) > 3 {
		return true
	}

	if params.RankingOptions != nil && params.RankingOptions.UseRelevanceScoring {
		return true
	}

	// Check config setting
	if a.config.ElasticsearchSettings.EnableSearching != nil && *a.config.ElasticsearchSettings.EnableSearching {
		return true
	}

	return false
}

// ValidateElasticsearchConnection tests connectivity to Elasticsearch cluster
func (a *ElasticsearchSearchAdapter) ValidateElasticsearchConnection(ctx context.Context) error {
	// Placeholder for actual connection validation
	// In production, this would ping the Elasticsearch cluster
	a.logger.Info("Elasticsearch connection validation", mlog.Bool("simulated", true))
	return nil
}

// GetElasticsearchIndexMappings returns the index mappings for posts
func (a *ElasticsearchSearchAdapter) GetElasticsearchIndexMappings() map[string]interface{} {
	return map[string]interface{}{
		"properties": map[string]interface{}{
			"id":         map[string]string{"type": "keyword"},
			"create_at":  map[string]string{"type": "date"},
			"message":    map[string]interface{}{
				"type": "text",
				"fields": map[string]interface{}{
					"english": map[string]string{
						"type": "text",
						"analyzer": "english",
					},
				},
			},
			"channel_id": map[string]string{"type": "keyword"},
			"user_id":    map[string]string{"type": "keyword"},
			"props":      map[string]string{"type": "object"},
		},
	}
}

