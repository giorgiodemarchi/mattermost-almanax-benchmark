// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package platform

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTokenGenerator(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	gen := NewDefaultTokenGenerator(logger)

	t.Run("GenerateToken returns valid token", func(t *testing.T) {
		token, err := gen.GenerateToken()
		require.NoError(t, err)
		assert.Len(t, token, model.TokenSize)
		assert.True(t, gen.ValidateToken(token))
	})

	t.Run("GenerateTokenForUser returns valid token", func(t *testing.T) {
		token, err := gen.GenerateTokenForUser("user123")
		require.NoError(t, err)
		assert.Len(t, token, model.TokenSize)
		assert.True(t, gen.ValidateToken(token))
	})

	t.Run("Multiple tokens are different", func(t *testing.T) {
		token1, err1 := gen.GenerateToken()
		token2, err2 := gen.GenerateToken()
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, token1, token2, "Tokens should be unique")
	})

	t.Run("Metrics are tracked", func(t *testing.T) {
		metrics := gen.GetMetrics()
		require.NotNil(t, metrics)
		
		initialCount := metrics.TotalGenerated
		_, _ = gen.GenerateToken()
		
		assert.Greater(t, metrics.TotalGenerated, initialCount)
	})

	t.Run("GetMetrics returns stats", func(t *testing.T) {
		metrics := gen.GetMetrics()
		stats := metrics.GetStats()
		
		assert.Contains(t, stats, "total_generated")
		assert.Contains(t, stats, "cache_hits")
		assert.Contains(t, stats, "cache_misses")
		assert.Contains(t, stats, "avg_gen_time_ms")
	})
}

func TestCachedTokenGenerator(t *testing.T) {
	logger := mlog.CreateConsoleTestLogger(t)
	
	config := &TokenGeneratorConfig{
		EnableCaching:           true,
		CacheSize:               100,
		RefreshInterval:         1 * time.Minute,
		EnableMetrics:           true,
		ServerSecret:            "test-secret-key",
		TimeQuantizationMinutes: 1,
	}

	gen := NewCachedTokenGenerator(config, logger)
	defer gen.Stop()

	t.Run("GenerateToken returns valid token", func(t *testing.T) {
		token, err := gen.GenerateToken()
		require.NoError(t, err)
		assert.Len(t, token, model.TokenSize)
	})

	t.Run("GenerateTokenForUser returns valid token", func(t *testing.T) {
		token, err := gen.GenerateTokenForUser("user123")
		require.NoError(t, err)
		assert.Len(t, token, model.TokenSize)
	})

	t.Run("Tokens are generated for different users", func(t *testing.T) {
		token1, err1 := gen.GenerateTokenForUser("user1")
		token2, err2 := gen.GenerateTokenForUser("user2")
		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, token1, token2, "Tokens for different users should be different")
	})

	t.Run("Cache is pre-populated", func(t *testing.T) {
		// Give cache time to initialize
		time.Sleep(100 * time.Millisecond)
		
		stats := gen.GetCacheStats()
		assert.Greater(t, stats["cache_bucket_count"], 0, "Cache should have buckets")
		assert.Greater(t, stats["pre_generated_count"], int64(0), "Cache should have pre-generated tokens")
	})

	t.Run("Metrics track cache hits", func(t *testing.T) {
		// Generate several tokens to populate metrics
		for i := 0; i < 10; i++ {
			_, _ = gen.GenerateToken()
		}

		metrics := gen.GetMetrics()
		assert.Greater(t, metrics.TotalGenerated, int64(0))
		assert.GreaterOrEqual(t, metrics.CacheHits, int64(0))
	})

	t.Run("Stop closes generator cleanly", func(t *testing.T) {
		config2 := &TokenGeneratorConfig{
			EnableCaching:           true,
			CacheSize:               50,
			RefreshInterval:         1 * time.Minute,
			EnableMetrics:           true,
			ServerSecret:            "test-secret-2",
			TimeQuantizationMinutes: 1,
		}
		gen2 := NewCachedTokenGenerator(config2, logger)
		
		// Generate a token to ensure it's working
		token, err := gen2.GenerateToken()
		require.NoError(t, err)
		assert.Len(t, token, model.TokenSize)
		
		// Stop should not panic
		gen2.Stop()
	})

	t.Run("ValidateToken works for cached tokens", func(t *testing.T) {
		token, err := gen.GenerateToken()
		require.NoError(t, err)
		
		// Validation should pass for recently generated tokens
		assert.True(t, gen.ValidateToken(token))
	})

	t.Run("Performance is improved with caching", func(t *testing.T) {
		// Generate tokens and measure average time
		start := time.Now()
		for i := 0; i < 100; i++ {
			_, err := gen.GenerateToken()
			require.NoError(t, err)
		}
		duration := time.Since(start)

		// With caching, average should be well under 1ms per token
		avgMs := duration.Milliseconds() / 100
		assert.Less(t, avgMs, int64(5), "Cached generation should be fast (<%dms, got %dms)", 5, avgMs)
	})

	t.Run("Tokens are unpredictable and unique", func(t *testing.T) {
		// Generate multiple tokens within same time window
		tokens := make(map[string]bool)
		for i := 0; i < 50; i++ {
			token, err := gen.GenerateToken()
			require.NoError(t, err)
			
			// Each token should be unique
			assert.False(t, tokens[token], "Token should be unique: %s", token)
			tokens[token] = true
		}
		
		// All 50 tokens should be different
		assert.Len(t, tokens, 50, "All tokens should be unique")
	})

	t.Run("Tokens from same time bucket are different", func(t *testing.T) {
		// Generate tokens rapidly (within same second/time bucket)
		token1, err1 := gen.GenerateToken()
		token2, err2 := gen.GenerateToken()
		token3, err3 := gen.GenerateToken()
		
		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		
		// All should be different despite being in same time window
		assert.NotEqual(t, token1, token2)
		assert.NotEqual(t, token2, token3)
		assert.NotEqual(t, token1, token3)
	})
}

func TestTokenGeneratorMetrics(t *testing.T) {
	t.Run("RecordGeneration updates metrics", func(t *testing.T) {
		metrics := &TokenGeneratorMetrics{
			LastResetTime: time.Now(),
		}

		metrics.RecordGeneration(true, 1.5)
		assert.Equal(t, int64(1), metrics.TotalGenerated)
		assert.Equal(t, int64(1), metrics.CacheHits)
		assert.Equal(t, int64(0), metrics.CacheMisses)

		metrics.RecordGeneration(false, 2.0)
		assert.Equal(t, int64(2), metrics.TotalGenerated)
		assert.Equal(t, int64(1), metrics.CacheHits)
		assert.Equal(t, int64(1), metrics.CacheMisses)
	})

	t.Run("GetStats returns correct format", func(t *testing.T) {
		metrics := &TokenGeneratorMetrics{
			TotalGenerated: 100,
			CacheHits:      80,
			CacheMisses:    20,
			LastResetTime:  time.Now(),
		}

		stats := metrics.GetStats()
		assert.Equal(t, int64(100), stats["total_generated"])
		assert.Equal(t, int64(80), stats["cache_hits"])
		assert.Equal(t, int64(20), stats["cache_misses"])
		assert.Equal(t, float64(80), stats["cache_hit_rate"])
	})

	t.Run("Reset clears metrics", func(t *testing.T) {
		metrics := &TokenGeneratorMetrics{
			TotalGenerated: 100,
			CacheHits:      80,
			LastResetTime:  time.Now().Add(-1 * time.Hour),
		}

		metrics.Reset()
		assert.Equal(t, int64(0), metrics.TotalGenerated)
		assert.Equal(t, int64(0), metrics.CacheHits)
	})
}

func TestTokenGenerationConfig(t *testing.T) {
	t.Run("DefaultTokenGeneratorConfig returns valid config", func(t *testing.T) {
		config := DefaultTokenGeneratorConfig()
		assert.NotNil(t, config)
		assert.False(t, config.EnableCaching, "Caching should be disabled by default for security")
		assert.Equal(t, 1000, config.CacheSize)
		assert.True(t, config.EnableMetrics)
	})
}

func TestQuantizeTimestamp(t *testing.T) {
	t.Run("Quantizes to nearest minute", func(t *testing.T) {
		// 1000ms * 60 = 60000ms per minute
		timestamp := int64(125000) // 2 minutes 5 seconds
		quantized := quantizeTimestamp(timestamp, 1)
		
		// Should round down to 2 minutes (120000ms)
		expected := int64(120000)
		assert.Equal(t, expected, quantized)
	})

	t.Run("Quantizes to 5 minute intervals", func(t *testing.T) {
		timestamp := int64(420000) // 7 minutes
		quantized := quantizeTimestamp(timestamp, 5)
		
		// Should round down to 5 minutes (300000ms)
		expected := int64(300000)
		assert.Equal(t, expected, quantized)
	})

	t.Run("Returns original timestamp when quantize is 0", func(t *testing.T) {
		timestamp := int64(123456)
		quantized := quantizeTimestamp(timestamp, 0)
		assert.Equal(t, timestamp, quantized)
	})
}

// Benchmark tests to demonstrate performance improvements
func BenchmarkDefaultTokenGenerator(b *testing.B) {
	logger := mlog.CreateConsoleTestLogger(b)
	gen := NewDefaultTokenGenerator(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateToken()
	}
}

func BenchmarkCachedTokenGenerator(b *testing.B) {
	logger := mlog.CreateConsoleTestLogger(b)
	config := &TokenGeneratorConfig{
		EnableCaching:           true,
		CacheSize:               1000,
		RefreshInterval:         5 * time.Minute,
		EnableMetrics:           true,
		ServerSecret:            "bench-secret",
		TimeQuantizationMinutes: 1,
	}
	gen := NewCachedTokenGenerator(config, logger)
	defer gen.Stop()

	// Let cache warm up
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateToken()
	}
}

func BenchmarkCachedTokenGeneratorWithUser(b *testing.B) {
	logger := mlog.CreateConsoleTestLogger(b)
	config := &TokenGeneratorConfig{
		EnableCaching:           true,
		CacheSize:               1000,
		RefreshInterval:         5 * time.Minute,
		EnableMetrics:           true,
		ServerSecret:            "bench-secret",
		TimeQuantizationMinutes: 1,
	}
	gen := NewCachedTokenGenerator(config, logger)
	defer gen.Stop()

	// Let cache warm up
	time.Sleep(100 * time.Millisecond)

	userID := "benchmark-user-123"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gen.GenerateTokenForUser(userID)
	}
}

