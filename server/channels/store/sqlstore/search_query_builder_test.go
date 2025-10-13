// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package sqlstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestSearchQueryBuilder_BuildAdvancedSearchQuery(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	store := th.Store().(*SqlStore).Post().(*SqlPostStore)
	builder := NewSearchQueryBuilder(store)

	t.Run("Basic query with terms", func(t *testing.T) {
		params := &model.SearchParams{
			Terms: "test message",
		}

		query, err := builder.BuildAdvancedSearchQuery("team123", "user123", params)
		require.NoError(t, err)
		assert.NotNil(t, query)

		sql, _, err := query.ToSql()
		require.NoError(t, err)
		assert.Contains(t, sql, "Posts q2")
		assert.Contains(t, sql, "DeleteAt = 0")
	})

	t.Run("Query with custom field filter - equals", func(t *testing.T) {
		params := &model.SearchParams{
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath: "metadata.priority",
					Operator:     "equals",
					Value:        "high",
				},
			},
		}

		query, err := builder.BuildAdvancedSearchQuery("team123", "user123", params)
		require.NoError(t, err)
		
		sql, _, err := query.ToSql()
		require.NoError(t, err)
		assert.Contains(t, sql, "Props->>")
		assert.Contains(t, sql, "metadata.priority")
	})

	t.Run("Query with custom field filter - contains", func(t *testing.T) {
		params := &model.SearchParams{
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath: "metadata.tags",
					Operator:     "contains",
					Value:        "urgent",
				},
			},
		}

		query, err := builder.BuildAdvancedSearchQuery("team123", "user123", params)
		require.NoError(t, err)
		
		sql, _, err := query.ToSql()
		require.NoError(t, err)
		assert.Contains(t, sql, "LIKE")
	})

	t.Run("Query with multiple filters", func(t *testing.T) {
		params := &model.SearchParams{
			CustomFieldFilters: []*model.CustomFieldFilter{
				{
					RawFieldPath:    "metadata.status",
					Operator:        "equals",
					Value:           "active",
					CombineOperator: "AND",
				},
				{
					RawFieldPath:    "metadata.priority",
					Operator:        "equals",
					Value:           "high",
					CombineOperator: "AND",
				},
			},
		}

		query, err := builder.BuildAdvancedSearchQuery("team123", "user123", params)
		require.NoError(t, err)
		
		sql, _, err := query.ToSql()
		require.NoError(t, err)
		assert.Contains(t, sql, "AND")
	})

	t.Run("Query with ranking options", func(t *testing.T) {
		params := &model.SearchParams{
			Terms: "test",
			RankingOptions: &model.SearchRankingOptions{
				UseRelevanceScoring: true,
				BoostRecent:        true,
				BoostReactions:     true,
			},
		}

		query, err := builder.BuildAdvancedSearchQuery("team123", "user123", params)
		require.NoError(t, err)
		
		sql, _, err := query.ToSql()
		require.NoError(t, err)
		assert.Contains(t, sql, "RelevanceScore")
	})

	t.Run("Query with channel filter", func(t *testing.T) {
		params := &model.SearchParams{
			Terms:      "test",
			InChannels: []string{"channel1", "channel2"},
		}

		query, err := builder.BuildAdvancedSearchQuery("team123", "user123", params)
		require.NoError(t, err)
		
		sql, _, err := query.ToSql()
		require.NoError(t, err)
		assert.Contains(t, sql, "ChannelId")
	})
}

func TestSearchQueryBuilder_FieldPathValidation(t *testing.T) {
	t.Run("Valid field paths are accepted", func(t *testing.T) {
		validPaths := []string{
			"metadata.priority",
			"customFields.status",
			"tags[0]",
			"nested.field.value",
			"simple",
		}

		for _, path := range validPaths {
			_, err := validateAndSanitizeFieldPath(path)
			assert.NoError(t, err, "Path should be valid: %s", path)
		}
	})

	t.Run("Dangerous SQL keywords are rejected (case-insensitive)", func(t *testing.T) {
		dangerousPaths := []string{
			// Uppercase
			"field.SELECT",
			"test.DROP",
			"data.DELETE",
			// Lowercase (previously vulnerable)
			"field.select",
			"test.drop",
			"data.delete",
			// Mixed case (previously vulnerable)
			"field.SeLeCt",
			"test.DrOp",
			"data.DeLeTe",
			// Other SQL keywords
			"value.insert",
			"name.update",
			"test.union",
			"data.exec",
		}

		for _, path := range dangerousPaths {
			_, err := validateAndSanitizeFieldPath(path)
			assert.Error(t, err, "Path should be rejected: %s", path)
		}
	})

	t.Run("SQL injection patterns are rejected", func(t *testing.T) {
		injectionPaths := []string{
			"field'; DROP TABLE Posts--",
			"field' OR '1'='1",
			"field'; DELETE FROM Users--",
			"field' UNION SELECT password FROM Users--",
			"field'||'injection",
			"field' && true",
			"field/* comment */",
			"field;DROP TABLE",
		}

		for _, path := range injectionPaths {
			_, err := validateAndSanitizeFieldPath(path)
			assert.Error(t, err, "Injection attempt should be rejected: %s", path)
		}
	})

	t.Run("Invalid characters are rejected", func(t *testing.T) {
		invalidPaths := []string{
			"field@domain",
			"field$value",
			"field#tag",
			"field%wildcard",
			"field&value",
			"field*star",
			"field+plus",
			"field=equals",
			"field<less",
			"field>greater",
		}

		for _, path := range invalidPaths {
			_, err := validateAndSanitizeFieldPath(path)
			assert.Error(t, err, "Invalid characters should be rejected: %s", path)
		}
	})

	t.Run("Very long field paths are rejected", func(t *testing.T) {
		longPath := ""
		for i := 0; i < 300; i++ {
			longPath += "a"
		}

		_, err := validateAndSanitizeFieldPath(longPath)
		assert.Error(t, err)
	})

	t.Run("Empty path segments are rejected", func(t *testing.T) {
		invalidPaths := []string{
			"field..nested",
			".field",
			"field.",
		}

		for _, path := range invalidPaths {
			_, err := validateAndSanitizeFieldPath(path)
			assert.Error(t, err, "Empty segments should be rejected: %s", path)
		}
	})
}

func TestSearchQueryBuilder_RelevanceScoring(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	store := th.Store().(*SqlStore).Post().(*SqlPostStore)
	builder := NewSearchQueryBuilder(store)

	t.Run("Relevance score without options", func(t *testing.T) {
		params := &model.SearchParams{
			Terms: "test",
		}

		expr := builder.buildRelevanceScoreExpression(params)
		assert.Contains(t, expr, "RelevanceScore")
	})

	t.Run("Relevance score with recency boost", func(t *testing.T) {
		params := &model.SearchParams{
			Terms: "test",
			RankingOptions: &model.SearchRankingOptions{
				UseRelevanceScoring: true,
				BoostRecent:        true,
			},
		}

		expr := builder.buildRelevanceScoreExpression(params)
		assert.Contains(t, expr, "LOG10")
		assert.Contains(t, expr, "RelevanceScore")
	})

	t.Run("Relevance score with reaction boost", func(t *testing.T) {
		params := &model.SearchParams{
			Terms: "test",
			RankingOptions: &model.SearchRankingOptions{
				UseRelevanceScoring: true,
				BoostReactions:     true,
			},
		}

		expr := builder.buildRelevanceScoreExpression(params)
		assert.Contains(t, expr, "Reactions")
		assert.Contains(t, expr, "RelevanceScore")
	})

	t.Run("Order clause with ranking", func(t *testing.T) {
		params := &model.SearchParams{
			Terms: "test",
			RankingOptions: &model.SearchRankingOptions{
				UseRelevanceScoring: true,
			},
		}

		orderClause := builder.buildOrderClause(params)
		assert.Contains(t, orderClause, "RelevanceScore DESC")
		assert.Contains(t, orderClause, "CreateAt DESC")
	})
}

func TestSearchQueryBuilder_PerformanceStats(t *testing.T) {
	th := Setup(t)
	defer th.TearDown()

	store := th.Store().(*SqlStore).Post().(*SqlPostStore)
	builder := NewSearchQueryBuilder(store)

	t.Run("Get query performance stats", func(t *testing.T) {
		query := "SELECT * FROM Posts WHERE Props->>'$.metadata.priority' = 'high'"
		
		stats := builder.GetQueryPerformanceStats(query)
		assert.NotNil(t, stats)
		assert.Contains(t, stats, "query_length")
		assert.Contains(t, stats, "has_custom_fields")
		assert.Contains(t, stats, "optimization_level")
		
		assert.Equal(t, len(query), stats["query_length"])
		assert.Equal(t, true, stats["has_custom_fields"])
	})
}

func TestSearchQueryBuilder_EscapeQuotes(t *testing.T) {
	t.Run("Single quotes are escaped", func(t *testing.T) {
		input := "test'value"
		output := escapeQuotes(input)
		assert.Equal(t, "test''value", output)
	})

	t.Run("Multiple quotes are escaped", func(t *testing.T) {
		input := "test'with'many'quotes"
		output := escapeQuotes(input)
		assert.Equal(t, "test''with''many''quotes", output)
	})

	t.Run("No quotes unchanged", func(t *testing.T) {
		input := "test value"
		output := escapeQuotes(input)
		assert.Equal(t, input, output)
	})
}

func TestSearchQueryBuilder_SanitizeSearchTerm(t *testing.T) {
	t.Run("Dangerous characters removed", func(t *testing.T) {
		input := "test'; DROP TABLE--"
		output := sanitizeSearchTerm(input)
		assert.NotContains(t, output, "'")
		assert.NotContains(t, output, ";")
		assert.NotContains(t, output, "--")
	})

	t.Run("Clean input unchanged", func(t *testing.T) {
		input := "normal search term"
		output := sanitizeSearchTerm(input)
		assert.Equal(t, input, output)
	})
}

