package websearch

import (
	"context"
	"testing"
)

func TestSessionManagement(t *testing.T) {
	config := Config{
		ZAIKey: "1d56b74c25de491da436f05005db1be0.pAWDn5c3DW9aYLEo",
		// Don't set ZAIBaseURL to use default
		Timeout: 30,
		DefaultOptions: &SearchOptions{
			NumResults: 3,
			Language:   "en",
			SafeSearch: "moderate",
		},
	}

	service := NewWebSearchService(config)

	// Test basic search
	result, err := service.Search(context.Background(), "latest AI technology developments")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(result.Results) == 0 {
		t.Error("Expected some results, got none")
	}

	t.Logf("Success! Found %d results", len(result.Results))
	for i, item := range result.Results {
		if i >= 2 { // Show first 2 results
			break
		}
		t.Logf("  %d. %s", i+1, item.Title)
		t.Logf("     URL: %s", item.URL)
	}
}
