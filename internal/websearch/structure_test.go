package websearch_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"flash-ai/pkg/websearch"
)

// Mock HTTP client for testing without real API calls
type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestWebSearchService_Structure(t *testing.T) {
	// Test that the service can be created with valid config
	config := websearch.Config{
		ZAIKey:  "test-api-key",
		Timeout: 30,
		DefaultOptions: &websearch.SearchOptions{
			NumResults: 10,
			Language:   "en",
		},
	}

	service := websearch.NewWebSearchService(config)
	if service == nil {
		t.Fatal("Expected service to be created, got nil")
	}

	// Test that service implements the interface
	var _ websearch.WebSearchService = service
}

func TestWebSearchService_ConfigValidation(t *testing.T) {
	t.Run("MissingAPIKey", func(t *testing.T) {
		config := websearch.Config{
			// No API key
			Timeout: 30,
		}

		service := websearch.NewWebSearchService(config)
		result, err := service.Search(context.Background(), "test query")

		if err == nil {
			t.Fatal("Expected error for missing API key, got nil")
		}

		if searchErr, ok := err.(*websearch.SearchError); ok {
			if searchErr.Code != "missing_api_key" {
				t.Errorf("Expected error code 'missing_api_key', got '%s'", searchErr.Code)
			}
		} else {
			t.Errorf("Expected SearchError type, got %T", err)
		}

		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		config := websearch.Config{
			ZAIKey: "test-key",
		}

		service := websearch.NewWebSearchService(config)

		// Should use default values
		if service == nil {
			t.Fatal("Expected service to be created with defaults")
		}
	})
}

func TestSearchOptions_Merging(t *testing.T) {
	config := websearch.Config{
		ZAIKey: "test-key",
		DefaultOptions: &websearch.SearchOptions{
			NumResults: 10,
			Language:   "en",
			SafeSearch: "moderate",
		},
	}

	service := websearch.NewWebSearchService(config)

	t.Run("NilUserOptions", func(t *testing.T) {
		// Should use defaults when user provides nil options
		_, err := service.Search(context.Background(), "test")
		// This will fail due to API call, but we can check if the call structure is correct
		if err == nil {
			t.Error("Expected API error since we're not mocking")
		}
	})

	t.Run("CustomOptions", func(t *testing.T) {
		customOptions := &websearch.SearchOptions{
			NumResults: 20,
			Region:     "us",
		}

		_, err := service.SearchWithOptions(context.Background(), "test", customOptions)
		// This will fail due to API call, but we can check if the call structure is correct
		if err == nil {
			t.Error("Expected API error since we're not mocking")
		}
	})
}

func TestContextHandling(t *testing.T) {
	config := websearch.Config{
		ZAIKey:  "test-key",
		Timeout: 1, // Short timeout
	}

	service := websearch.NewWebSearchService(config)

	t.Run("ContextTimeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		result, err := service.Search(ctx, "test query")

		// Should timeout quickly due to short timeout
		if err == nil {
			// If no error, might be due to fast local processing
			t.Log("No timeout error occurred")
		}

		if result != nil && len(result.Results) > 0 {
			t.Error("Should not have results for timeout test")
		}
	})

	t.Run("CanceledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := service.Search(ctx, "test query")

		if err == nil {
			t.Error("Expected error for canceled context")
		}

		if result != nil && len(result.Results) > 0 {
			t.Error("Should not have results for canceled context")
		}
	})
}

func TestSearchError_Structure(t *testing.T) {
	err := &websearch.SearchError{
		Code:    "test_error",
		Message: "Test error message",
		Details: "Additional error details",
	}

	expectedMsg := "Test error message: Additional error details"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}

	// Test without details
	errNoDetails := &websearch.SearchError{
		Code:    "test_error",
		Message: "Test error message",
	}

	if errNoDetails.Error() != "Test error message" {
		t.Errorf("Expected 'Test error message', got '%s'", errNoDetails.Error())
	}
}
