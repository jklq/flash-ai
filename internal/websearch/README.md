# Web Search Package

The `websearch` package provides a comprehensive interface for web search operations using Z.AI's search API, enabling real-time information retrieval from the web.

## Features

- **Z.AI Search API**: High-quality web search with real-time information
- **Flexible Search Options**: Support for language, region, time range, and safe search filters
- **Error Handling**: Robust error handling with retry logic and detailed error messages
- **Context Support**: Full support for context cancellation and timeouts
- **Customizable Configuration**: Configurable timeouts, default options, and API endpoints

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "flash-ai/pkg/websearch"
)

func main() {
    // Configure the web search service
    config := websearch.Config{
        ZAIKey:     "your-zai-api-key",
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
```

## Configuration

The web search service can be configured with the following options:

### Z.AI API Configuration

- `ZAIKey`: API key for Z.AI search service (required)
- `ZAIBaseURL`: Base URL for Z.AI search API (defaults to "https://api.z.ai/api/mcp/web_search_prime/mcp")

### Service Configuration

- `Timeout`: HTTP client timeout in seconds (defaults to 30)
- `DefaultOptions`: Default search options applied to all requests

## API Reference

### WebSearchService Interface

```go
type WebSearchService interface {
    Search(ctx context.Context, query string) (*SearchResult, error)
    SearchWithOptions(ctx context.Context, query string, options *SearchOptions) (*SearchResult, error)
}
```

### Methods

#### Search
Performs a web search using default options.

**Parameters:**
- `ctx`: Context for the operation
- `query`: Search query string

**Returns:**
- `*SearchResult`: Search results with metadata
- `error`: Error if any

#### SearchWithOptions
Performs a web search with custom options.

**Parameters:**
- `ctx`: Context for the operation
- `query`: Search query string
- `options`: Custom search options

**Returns:**
- `*SearchResult`: Search results with metadata
- `error`: Error if any

### Types

#### SearchResult
```go
type SearchResult struct {
    Query     string        `json:"query"`
    Results   []SearchItem  `json:"results"`
    Total     int           `json:"total,omitempty"`
    Duration  string        `json:"duration,omitempty"`
    Timestamp int64         `json:"timestamp,omitempty"`
}
```

#### SearchItem
```go
type SearchItem struct {
    Title        string `json:"title"`
    URL          string `json:"url"`
    Snippet      string `json:"snippet"`
    SiteName     string `json:"site_name,omitempty"`
    SiteIcon     string `json:"site_icon,omitempty"`
    PublishedDate string `json:"published_date,omitempty"`
    ContentType  string `json:"content_type,omitempty"`
}
```

#### SearchOptions
```go
type SearchOptions struct {
    NumResults    int    `json:"num_results,omitempty"`    // Number of results (default: 10)
    Language      string `json:"language,omitempty"`      // Language filter (e.g., "en", "zh")
    Region        string `json:"region,omitempty"`        // Region filter (e.g., "us", "cn")
    TimeRange     string `json:"time_range,omitempty"`     // Time range (e.g., "day", "week", "month")
    SafeSearch    string `json:"safe_search,omitempty"`    // Safe search level ("off", "moderate", "strict")
    IncludeImages bool   `json:"include_images,omitempty"` // Include images in results
    IncludeNews   bool   `json:"include_news,omitempty"`   // Include news in results
}
```

## Usage Examples

### Basic Search

```go
result, err := searchService.Search(ctx, "Python async programming best practices")
if err != nil {
    return err
}

fmt.Printf("Found %d results\n", len(result.Results))
```

### Advanced Search with Options

```go
options := &websearch.SearchOptions{
    NumResults: 20,
    Language:   "en",
    Region:     "us",
    TimeRange:  "month",
    SafeSearch: "moderate",
    IncludeNews: true,
}

result, err := searchService.SearchWithOptions(ctx, "machine learning news", options)
if err != nil {
    return err
}
```

### Search with Context Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

result, err := searchService.Search(ctx, "quick search")
if err != nil {
    if err == context.DeadlineExceeded {
        fmt.Println("Search timed out")
    }
    return err
}
```

### Error Handling

```go
result, err := searchService.Search(ctx, "test query")
if err != nil {
    if searchErr, ok := err.(*websearch.SearchError); ok {
        switch searchErr.Code {
        case "missing_api_key":
            fmt.Println("API key is required")
        case "invalid_api_key":
            fmt.Println("Invalid API key provided")
        case "rate_limit_exceeded":
            fmt.Println("Rate limit exceeded, please try again later")
        default:
            fmt.Printf("Search error: %s\n", searchErr.Message)
        }
    }
    return err
}
```

## Search Options Reference

### Language Codes
- `"en"` - English
- `"zh"` - Chinese
- `"es"` - Spanish
- `"fr"` - French
- `"de"` - German
- `"ja"` - Japanese
- `"ko"` - Korean

### Region Codes
- `"us"` - United States
- `"cn"` - China
- `"uk"` - United Kingdom
- `"jp"` - Japan
- `"kr"` - Korea
- `"de"` - Germany
- `"fr"` - France

### Time Range Options
- `"day"` - Last 24 hours
- `"week"` - Last 7 days
- `"month"` - Last 30 days
- `"year"` - Last 365 days

### Safe Search Levels
- `"off"` - No filtering
- `"moderate"` - Moderate filtering (default)
- `"strict"` - Strict filtering

## Error Handling

The package provides comprehensive error handling with specific error codes:

- `missing_api_key` - API key not provided
- `invalid_api_key` - API key is invalid
- `rate_limit_exceeded` - API rate limit exceeded
- `network_error` - Network connectivity issues
- `request_failed` - Request failed after retries
- `response_parse_failed` - Failed to parse API response

## Performance Considerations

- Use appropriate timeouts for your use case
- Implement client-side rate limiting if needed
- Consider caching results for frequently repeated queries
- Use context cancellation for long-running searches

## Dependencies

- Standard library only (no external dependencies)

## License

This package is part of the Flash-AI project.