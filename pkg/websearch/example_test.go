package websearch_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"flash-ai/pkg/websearch"
)

func ExampleWebSearchService_Search() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey:     "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
		ZAIBaseURL: "https://api.z.ai/api/mcp/web_search_prime/mcp",
		Timeout:    30,
		DefaultOptions: &websearch.SearchOptions{
			NumResults: 10,
			Language:   "en",
			SafeSearch: "moderate",
		},
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Perform a simple search
	result, err := searchService.Search(
		context.Background(),
		"latest AI technology developments",
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d results for query: %s\n", len(result.Results), result.Query)
	for i, item := range result.Results {
		fmt.Printf("%d. %s\n", i+1, item.Title)
		fmt.Printf("   %s\n", item.URL)
		fmt.Printf("   %s\n\n", item.Snippet)
	}
}

func ExampleWebSearchService_SearchWithOptions() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey:  "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
		Timeout: 30,
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Configure advanced search options
	options := &websearch.SearchOptions{
		NumResults: 20,
		Language:   "en",
		Region:     "us",
		TimeRange:  "week",
		SafeSearch: "moderate",
	}

	// Perform search with custom options
	result, err := searchService.SearchWithOptions(
		context.Background(),
		"machine learning breakthroughs",
		options,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d recent results\n", len(result.Results))
	fmt.Printf("Search timestamp: %d\n", result.Timestamp)
}

func ExampleWebSearchService_Search_contextTimeout() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey:  "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
		Timeout: 10, // Short timeout for demo
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Perform search with timeout
	result, err := searchService.Search(ctx, "quick search example")
	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Println("Search timed out")
			return
		}
		log.Fatal(err)
	}

	fmt.Printf("Search completed within timeout: %d results\n", len(result.Results))
}

func ExampleWebSearchService_Search_languageFilter() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey: "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
		DefaultOptions: &websearch.SearchOptions{
			Language: "zh", // Chinese results
			Region:   "cn", // China region
		},
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Search for Chinese content
	result, err := searchService.Search(
		context.Background(),
		"人工智能 最新发展",
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d Chinese results\n", len(result.Results))
	for _, item := range result.Results {
		fmt.Printf("- %s\n", item.Title)
	}
}

func ExampleWebSearchService_SearchWithOptions_timeRange() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey: "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Search for news from the last day
	options := &websearch.SearchOptions{
		TimeRange:  "day",
		NumResults: 15,
	}

	result, err := searchService.SearchWithOptions(
		context.Background(),
		"technology news",
		options,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d results from the last 24 hours\n", len(result.Results))
	for _, item := range result.Results {
		if item.PublishedDate != "" {
			fmt.Printf("- %s (%s)\n", item.Title, item.PublishedDate)
		} else {
			fmt.Printf("- %s\n", item.Title)
		}
	}
}

func ExampleWebSearchService_SearchWithOptions_safeSearch() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey: "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Search with filtering
	options := &websearch.SearchOptions{
		NumResults: 10,
		Language:   "en",
	}

	result, err := searchService.SearchWithOptions(
		context.Background(),
		"educational resources for students",
		options,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d family-friendly results\n", len(result.Results))
	for _, item := range result.Results {
		fmt.Printf("- %s (%s)\n", item.Title, item.SiteName)
	}
}

func ExampleWebSearchService_Search_errorHandling() {
	// Configure with invalid API key to demonstrate error handling
	config := websearch.Config{
		ZAIKey: "invalid-api-key",
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// This will fail with an authentication error
	result, err := searchService.Search(
		context.Background(),
		"test query",
	)
	if err != nil {
		if searchErr, ok := err.(*websearch.SearchError); ok {
			fmt.Printf("Search error code: %s\n", searchErr.Code)
			fmt.Printf("Search error message: %s\n", searchErr.Message)
			if searchErr.Details != "" {
				fmt.Printf("Error details: %s\n", searchErr.Details)
			}
			return
		}
		log.Fatal(err)
	}

	// This line won't be reached due to the error
	fmt.Printf("Unexpected success: %d results\n", len(result.Results))
}

func ExampleWebSearchService_SearchWithOptions_images() {
	// Configure the web search service
	config := websearch.Config{
		ZAIKey: "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// Search for general results
	options := &websearch.SearchOptions{
		NumResults: 15,
		Language:   "en",
	}

	result, err := searchService.SearchWithOptions(
		context.Background(),
		"beautiful nature landscapes",
		options,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d results\n", len(result.Results))
	for _, item := range result.Results {
		if item.ContentType == "image" {
			fmt.Printf("[IMAGE] %s\n", item.Title)
		} else {
			fmt.Printf("[WEB] %s\n", item.Title)
		}
	}
}

func ExampleWebSearchService_Search_defaultOptions() {
	// Configure service with comprehensive default options
	config := websearch.Config{
		ZAIKey:     "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
		ZAIBaseURL: "https://api.z.ai/api/mcp/web_search_prime/mcp",
		Timeout:    45,
		DefaultOptions: &websearch.SearchOptions{
			NumResults: 12,
			Language:   "en",
			Region:     "us",
			TimeRange:  "month",
			SafeSearch: "moderate",
		},
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	// This search will use all the default options
	result, err := searchService.Search(
		context.Background(),
		"climate change research",
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Search with defaults: %d results\n", len(result.Results))
	fmt.Printf("Query: %s\n", result.Query)
	fmt.Printf("Timestamp: %d\n", result.Timestamp)
}
