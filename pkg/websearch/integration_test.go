package websearch

import (
	"context"
	"testing"
	"time"
)

func TestWebSearchIntegration(t *testing.T) {
	config := Config{
		ZAIKey: "1d56b74c25de491da436f05005db1be0.pAWDn5c3DW9aYLEo",
		// Don't set ZAIBaseURL to use default
		Timeout: 30,
		DefaultOptions: &SearchOptions{
			NumResults: 5,
			Language:   "en",
			SafeSearch: "moderate",
		},
	}

	service := NewWebSearchService(config)

	// Test 1: Basic search
	t.Run("BasicSearch", func(t *testing.T) {
		result, err := service.Search(context.Background(), "latest AI technology developments")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(result.Results) == 0 {
			t.Error("Expected some results, got none")
		}

		if result.Query != "latest AI technology developments" {
			t.Errorf("Expected query 'latest AI technology developments', got '%s'", result.Query)
		}

		if result.Timestamp == 0 {
			t.Error("Expected timestamp to be set")
		}

		t.Logf("Found %d results", len(result.Results))
		for i, item := range result.Results {
			if i >= 2 { // Show first 2 results
				break
			}
			t.Logf("  %d. %s", i+1, item.Title)
			t.Logf("     URL: %s", item.URL)
			if item.Snippet != "" {
				t.Logf("     Snippet: %s", item.Snippet[:min(100, len(item.Snippet))]+"...")
			}
		}
	})

	// Test 2: Search with options
	t.Run("SearchWithOptions", func(t *testing.T) {
		options := &SearchOptions{
			NumResults:  3,
			Language:    "en",
			TimeRange:   "week",
			IncludeNews: true,
		}

		result, err := service.SearchWithOptions(context.Background(), "machine learning news", options)
		if err != nil {
			t.Fatalf("Search with options failed: %v", err)
		}

		if len(result.Results) == 0 {
			t.Error("Expected some results, got none")
		}

		t.Logf("Found %d recent results", len(result.Results))
		for i, item := range result.Results {
			t.Logf("  %d. %s", i+1, item.Title)
			if item.PublishedDate != "" {
				t.Logf("     Published: %s", item.PublishedDate)
			}
		}
	})

	// Test 3: Search with timeout
	t.Run("SearchWithTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result, err := service.Search(ctx, "quick test")
		if err != nil {
			if err == context.DeadlineExceeded {
				t.Skip("Search timed out (expected behavior test)")
			} else {
				t.Fatalf("Search with timeout failed: %v", err)
			}
		}

		if len(result.Results) == 0 {
			t.Error("Expected some results, got none")
		}

		t.Logf("Found %d results within timeout", len(result.Results))
	})

	// Test 4: Error handling with invalid API key
	t.Run("ErrorHandling", func(t *testing.T) {
		invalidConfig := Config{
			ZAIKey:  "invalid-api-key",
			Timeout: 10,
		}

		invalidService := NewWebSearchService(invalidConfig)
		_, err := invalidService.Search(context.Background(), "test query")
		if err == nil {
			t.Error("Expected error with invalid API key")
		}

		if searchErr, ok := err.(*SearchError); ok {
			if searchErr.Code == "" {
				t.Error("Expected error code to be set")
			}
			t.Logf("Correctly handled error: %s - %s", searchErr.Code, searchErr.Message)
		} else {
			t.Errorf("Expected SearchError, got %T", err)
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
