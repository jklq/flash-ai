package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"flash-ai/pkg/websearch"
)

func main() {
	// NOTE: The web search service requires a specific API key from Z.AI
	// You can get one from: https://z.ai/manage-apikey/apikey-list
	// The API key in .env file is for other Z.AI services, not web search
	config := websearch.Config{
		ZAIKey:     "7f8be0b71f0a4baeb163d2bb58b1eca6.u39u2gBBcPD8fv9G",
		ZAIBaseURL: "https://api.z.ai/api/mcp/web_search_prime",
		Timeout:    30,
		DefaultOptions: &websearch.SearchOptions{
			NumResults: 3,
			Language:   "en",
			SafeSearch: "moderate",
		},
	}

	// Create web search service
	searchService := websearch.NewWebSearchService(config)

	fmt.Println("Testing Web Search Package...")
	fmt.Println("================================")

	// Test 1: Basic search
	fmt.Println("\n1. Testing basic search:")
	fmt.Println("Query: 'latest AI technology developments'")
	
	result, err := searchService.Search(
		context.Background(),
		"latest AI technology developments",
	)
	if err != nil {
		log.Printf("Basic search failed: %v", err)
	} else {
		fmt.Printf("✓ Success! Found %d results\n", len(result.Results))
		fmt.Printf("Query: %s\n", result.Query)
		fmt.Printf("Timestamp: %d\n", result.Timestamp)
		
		for i, item := range result.Results {
			if i >= 2 { // Show first 2 results
				break
			}
			fmt.Printf("  %d. %s\n", i+1, item.Title)
			fmt.Printf("     URL: %s\n", item.URL)
			fmt.Printf("     Snippet: %s\n", item.Snippet[:min(100, len(item.Snippet))] + "...")
		}
	}

	// Test 2: Search with options
	fmt.Println("\n2. Testing search with custom options:")
	fmt.Println("Query: 'machine learning news' (last week, include news)")
	
	options := &websearch.SearchOptions{
		NumResults: 3,
		Language:   "en",
		TimeRange:  "week",
		IncludeNews: true,
	}
	
	result2, err := searchService.SearchWithOptions(
		context.Background(),
		"machine learning news",
		options,
	)
	if err != nil {
		log.Printf("Advanced search failed: %v", err)
	} else {
		fmt.Printf("✓ Success! Found %d recent results\n", len(result2.Results))
		for i, item := range result2.Results {
			fmt.Printf("  %d. %s\n", i+1, item.Title)
			if item.PublishedDate != "" {
				fmt.Printf("     Published: %s\n", item.PublishedDate)
			}
		}
	}

	// Test 3: Search with timeout
	fmt.Println("\n3. Testing search with timeout:")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	result3, err := searchService.Search(ctx, "quick test")
	if err != nil {
		if err == context.DeadlineExceeded {
			fmt.Printf("✗ Search timed out (expected behavior test)\n")
		} else {
			log.Printf("Timeout search failed: %v", err)
		}
	} else {
		fmt.Printf("✓ Success! Found %d results within timeout\n", len(result3.Results))
	}

	// Test 4: Error handling with invalid query (if applicable)
	fmt.Println("\n4. Testing error handling:")
	
	// Test with empty query to see error handling
	result4, err := searchService.Search(context.Background(), "")
	if err != nil {
		fmt.Printf("✓ Error handling works: %v\n", err)
		if searchErr, ok := err.(*websearch.SearchError); ok {
			fmt.Printf("  Error code: %s\n", searchErr.Code)
		}
	} else {
		fmt.Printf("✓ Empty query returned %d results\n", len(result4.Results))
	}

	fmt.Println("\n================================")
	fmt.Println("Web Search Package Test Complete!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}