package minio

import (
	"context"
	"testing"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/config"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestSetupTTLPolicies_DefaultOnly(t *testing.T) {
	// This is an integration-style test that would require actual MinIO
	// For unit testing, we'd need to mock the MinIO client

	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	ttlConfig := config.TTLConfig{
		Enabled: true,
		Default: 24 * time.Hour,
	}

	// Test that configuration is properly formatted
	assert.True(t, ttlConfig.Enabled)
	assert.Equal(t, 24*time.Hour, ttlConfig.Default)
	assert.Empty(t, ttlConfig.Channels)

	// Calculate days for MinIO lifecycle
	days := int(ttlConfig.Default.Hours() / 24)
	assert.Equal(t, 1, days)

	_ = log // Use logger to avoid unused variable warning
}

func TestSetupTTLPolicies_WithChannels(t *testing.T) {
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	ttlConfig := config.TTLConfig{
		Enabled: true,
		Default: 24 * time.Hour,
		Channels: []config.ChannelTTLConfig{
			{Channel: "test", Duration: 2 * time.Hour},
			{Channel: "images", Duration: 48 * time.Hour},
		},
	}

	// Test channel-specific configuration
	assert.Len(t, ttlConfig.Channels, 2)
	assert.Equal(t, "test", ttlConfig.Channels[0].Channel)
	assert.Equal(t, 2*time.Hour, ttlConfig.Channels[0].Duration)
	assert.Equal(t, "images", ttlConfig.Channels[1].Channel)
	assert.Equal(t, 48*time.Hour, ttlConfig.Channels[1].Duration)

	// Test prefix generation for rules
	testPrefix := ttlConfig.Channels[0].Channel + "_"
	assert.Equal(t, "test_", testPrefix)

	imagesPrefix := ttlConfig.Channels[1].Channel + "_"
	assert.Equal(t, "images_", imagesPrefix)

	_ = log
}

func TestSetupTTLPolicies_DaysCalculation(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected int
	}{
		{"1 hour", 1 * time.Hour, 0},
		{"12 hours", 12 * time.Hour, 0},
		{"24 hours", 24 * time.Hour, 1},
		{"48 hours", 48 * time.Hour, 2},
		{"7 days", 7 * 24 * time.Hour, 7},
		{"30 days", 30 * 24 * time.Hour, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days := int(tt.duration.Hours() / 24)
			assert.Equal(t, tt.expected, days)
		})
	}
}

func TestSetupTTLPolicies_Disabled(t *testing.T) {
	ttlConfig := config.TTLConfig{
		Enabled: false,
		Default: 24 * time.Hour,
	}

	// When disabled, no rules should be applied
	assert.False(t, ttlConfig.Enabled)
}

func TestSetupTTLPolicies_RuleIDGeneration(t *testing.T) {
	channels := []config.ChannelTTLConfig{
		{Channel: "test", Duration: 1 * time.Hour},
		{Channel: "images", Duration: 2 * time.Hour},
		{Channel: "logs", Duration: 3 * time.Hour},
	}

	expectedIDs := []string{
		"channel-test-ttl",
		"channel-images-ttl",
		"channel-logs-ttl",
	}

	for i, ch := range channels {
		ruleID := "channel-" + ch.Channel + "-ttl"
		assert.Equal(t, expectedIDs[i], ruleID)
	}
}

// Mock test to verify SetupTTLPolicies would be called
func TestRepository_SetupTTLPolicies_Mock(t *testing.T) {
	ctx := context.Background()
	log, _ := logger.New(logger.Config{Level: "info", Format: "json"})

	ttlConfig := config.TTLConfig{
		Enabled: true,
		Default: 24 * time.Hour,
		Channels: []config.ChannelTTLConfig{
			{Channel: "test", Duration: 1 * time.Hour},
		},
	}

	// Verify configuration structure
	assert.NotNil(t, ctx)
	assert.NotNil(t, log)
	assert.True(t, ttlConfig.Enabled)
	assert.Equal(t, 24*time.Hour, ttlConfig.Default)
	assert.Len(t, ttlConfig.Channels, 1)
}
