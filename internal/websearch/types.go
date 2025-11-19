package websearch

import "context"

// WebSearchService defines the interface for web search operations
type WebSearchService interface {
	// Search performs a web search with the given query
	Search(ctx context.Context, query string) (*SearchResult, error)
	
	// SearchWithOptions performs a web search with additional options
	SearchWithOptions(ctx context.Context, query string, options *SearchOptions) (*SearchResult, error)
}

// SearchResult represents the response from a web search operation
type SearchResult struct {
	Query     string        `json:"query"`
	Results   []SearchItem  `json:"results"`
	Total     int           `json:"total,omitempty"`
	Duration  string        `json:"duration,omitempty"`
	Timestamp int64         `json:"timestamp,omitempty"`
}

// SearchItem represents a single search result item
type SearchItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	SiteName    string `json:"site_name,omitempty"`
	SiteIcon    string `json:"site_icon,omitempty"`
	PublishedDate string `json:"published_date,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

// SearchOptions provides additional configuration for search operations
type SearchOptions struct {
	// Number of results to return (default: 10)
	NumResults int `json:"num_results,omitempty"`
	
	// Language filter (e.g., "en", "zh")
	Language string `json:"language,omitempty"`
	
	// Region filter (e.g., "us", "cn")
	Region string `json:"region,omitempty"`
	
	// Time range filter (e.g., "day", "week", "month", "year")
	TimeRange string `json:"time_range,omitempty"`
	
	// Safe search level ("off", "moderate", "strict")
	SafeSearch string `json:"safe_search,omitempty"`
	
	// Include images in results
	IncludeImages bool `json:"include_images,omitempty"`
	
	// Include news in results
	IncludeNews bool `json:"include_news,omitempty"`
}

// Config holds configuration for web search services
type Config struct {
	// Z.AI API configuration
	ZAIKey     string
	ZAIBaseURL string
	
	// Default search options
	DefaultOptions *SearchOptions
	
	// HTTP client timeout in seconds (default: 30)
	Timeout int
}

// SearchError represents an error that occurred during search
type SearchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *SearchError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}