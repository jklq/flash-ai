package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// service implements the WebSearchService interface
type service struct {
	config     *Config
	client     *http.Client
	sessionID  string
	sessionMux sync.RWMutex
}

// NewWebSearchService creates a new web search service with the given configuration
func NewWebSearchService(config Config) WebSearchService {
	// Set default values
	if config.ZAIBaseURL == "" {
		config.ZAIBaseURL = "https://api.z.ai/api/mcp/web_search_prime/mcp"
	}
	if config.Timeout == 0 {
		config.Timeout = 30
	}
	if config.DefaultOptions == nil {
		config.DefaultOptions = &SearchOptions{
			NumResults: 10,
			Language:   "en",
			SafeSearch: "off",
		}
	}

	return &service{
		config: &config,
		client: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
	}
}

// Search implements WebSearchService
func (s *service) Search(ctx context.Context, query string) (*SearchResult, error) {
	return s.SearchWithOptions(ctx, query, s.config.DefaultOptions)
}

// SearchWithOptions implements WebSearchService
func (s *service) SearchWithOptions(ctx context.Context, query string, options *SearchOptions) (*SearchResult, error) {
	if s.config.ZAIKey == "" {
		return nil, &SearchError{
			Code:    "missing_api_key",
			Message: "Z.AI API key is required",
		}
	}

	// Ensure MCP session is initialized
	if err := s.ensureSession(ctx); err != nil {
		return nil, err
	}

	// Merge options with defaults
	searchOptions := s.mergeOptions(options)

	// Create MCP request
	req := s.createMCPRequest(query, searchOptions)

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, &SearchError{
			Code:    "marshal_error",
			Message: "Failed to marshal search request",
			Details: err.Error(),
		}
	}

	// Execute request with retry logic
	result, err := s.executeRequestWithRetry(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	// Add metadata
	result.Query = query
	result.Timestamp = time.Now().Unix()

	return result, nil
}

// mergeOptions merges user options with defaults
func (s *service) mergeOptions(userOptions *SearchOptions) *SearchOptions {
	if userOptions == nil {
		userOptions = &SearchOptions{}
	}

	merged := *s.config.DefaultOptions

	// Override with user options
	if userOptions.NumResults > 0 {
		merged.NumResults = userOptions.NumResults
	}
	if userOptions.Language != "" {
		merged.Language = userOptions.Language
	}
	if userOptions.Region != "" {
		merged.Region = userOptions.Region
	}
	if userOptions.TimeRange != "" {
		merged.TimeRange = userOptions.TimeRange
	}
	if userOptions.SafeSearch != "" {
		merged.SafeSearch = userOptions.SafeSearch
	}
	if userOptions.IncludeImages {
		merged.IncludeImages = userOptions.IncludeImages
	}
	if userOptions.IncludeNews {
		merged.IncludeNews = userOptions.IncludeNews
	}

	return &merged
}

// ensureSession ensures MCP session is initialized
func (s *service) ensureSession(ctx context.Context) error {
	s.sessionMux.RLock()
	if s.sessionID != "" {
		s.sessionMux.RUnlock()
		return nil
	}
	s.sessionMux.RUnlock()

	// Need to initialize session
	return s.initializeSession(ctx)
}

// initializeSession initializes MCP session
func (s *service) initializeSession(ctx context.Context) error {
	s.sessionMux.Lock()
	defer s.sessionMux.Unlock()

	// Double-check after acquiring write lock
	if s.sessionID != "" {
		return nil
	}

	// Create initialization request
	initReq := &MCPRequest{
		JSONRPC: "2.0",
		ID:      "init",
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"clientInfo": map[string]interface{}{
				"name":    "Flash-AI WebSearch",
				"version": "1.0",
			},
		},
	}

	// Marshal request
	reqBody, err := json.Marshal(initReq)
	if err != nil {
		return &SearchError{
			Code:    "session_init_failed",
			Message: "Failed to marshal initialization request",
			Details: err.Error(),
		}
	}

	// Create HTTP request
	url := s.config.ZAIBaseURL
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return &SearchError{
			Code:    "session_init_failed",
			Message: "Failed to create initialization request",
			Details: err.Error(),
		}
	}

	// Set headers
	httpReq.Header.Set("Authorization", "Bearer "+s.config.ZAIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("User-Agent", "Flash-AI WebSearch/1.0")

	// Execute request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return &SearchError{
			Code:    "session_init_failed",
			Message: "Failed to execute initialization request",
			Details: err.Error(),
		}
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return s.handleHTTPError(resp.StatusCode, body)
	}

	// Extract session ID from response headers
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		return &SearchError{
			Code:    "session_init_failed",
			Message: "No session ID returned from MCP server",
		}
	}

	// Store session ID
	s.sessionID = sessionID

	return nil
}

// MCPRequest represents a Model Context Protocol request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// MCPToolParams represents parameters for tool calls
type MCPToolParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

// MCPResponse represents a Model Context Protocol response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// MCPToolResult represents the result of a tool call
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent represents content in MCP responses
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// createMCPRequest creates an MCP request for web search
func (s *service) createMCPRequest(query string, options *SearchOptions) *MCPRequest {
	arguments := map[string]interface{}{
		"search_query": query,
	}

	// Map options to MCP schema
	if options.NumResults > 0 {
		arguments["count"] = options.NumResults
	}

	if options.TimeRange != "" {
		// Map time range to MCP format
		searchRecencyMap := map[string]string{
			"day":   "oneDay",
			"week":  "oneWeek",
			"month": "oneMonth",
			"year":  "oneYear",
		}
		if mcpTimeRange, ok := searchRecencyMap[options.TimeRange]; ok {
			arguments["search_recency_filter"] = mcpTimeRange
		}
	}

	if options.Region != "" {
		// Map region to MCP location format
		locationMap := map[string]string{
			"cn": "cn",
			"us": "us",
		}
		if mcpLocation, ok := locationMap[options.Region]; ok {
			arguments["location"] = mcpLocation
		}
	}

	// Language can be used to determine location if region is not set
	if options.Language != "" && options.Region == "" {
		if options.Language == "zh" {
			arguments["location"] = "cn"
		} else {
			arguments["location"] = "us"
		}
	}

	// Set content size based on num results (more results = more detailed)
	if options.NumResults >= 15 {
		arguments["content_size"] = "high"
	} else {
		arguments["content_size"] = "medium"
	}

	return &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: MCPToolParams{
			Name:      "webSearchPrime",
			Arguments: arguments,
		},
	}
}

// executeRequestWithRetry executes the HTTP request with retry logic
func (s *service) executeRequestWithRetry(ctx context.Context, reqBody []byte) (*SearchResult, error) {
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {

		result, err := s.executeSingleRequest(ctx, reqBody)
		if err != nil {
			lastErr = err

			// Don't retry on client errors (4xx)
			if searchErr, ok := err.(*SearchError); ok {
				if searchErr.Code == "invalid_api_key" || searchErr.Code == "rate_limit_exceeded" {
					return nil, err
				}
				// Retry on session errors by reinitializing session
				if searchErr.Code == "no_session" || strings.Contains(searchErr.Message, "session") {
					// Clear session and retry
					s.sessionMux.Lock()
					s.sessionID = ""
					s.sessionMux.Unlock()
					continue
				}
			}
			continue
		}

		return result, nil
	}

	return nil, &SearchError{
		Code:    "request_failed",
		Message: "Web search API failed after retries",
		Details: lastErr.Error(),
	}
}

// executeSingleRequest executes a single HTTP request
func (s *service) executeSingleRequest(ctx context.Context, reqBody []byte) (*SearchResult, error) {
	// Get current session ID
	s.sessionMux.RLock()
	sessionID := s.sessionID
	s.sessionMux.RUnlock()

	if sessionID == "" {
		return nil, &SearchError{
			Code:    "no_session",
			Message: "MCP session not initialized",
		}
	}

	// Create HTTP request using MCP endpoint
	url := s.config.ZAIBaseURL
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, &SearchError{
			Code:    "request_creation_failed",
			Message: "Failed to create HTTP request",
			Details: err.Error(),
		}
	}

	// Set headers
	httpReq.Header.Set("Authorization", "Bearer "+s.config.ZAIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set("Mcp-Session-Id", sessionID)
	httpReq.Header.Set("User-Agent", "Flash-AI WebSearch/1.0")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Execute request
	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, &SearchError{
			Code:    "network_error",
			Message: "Network request failed",
			Details: err.Error(),
		}
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &SearchError{
			Code:    "response_read_failed",
			Message: "Failed to read response body",
			Details: err.Error(),
		}
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, s.handleHTTPError(resp.StatusCode, body)
	}

	// Parse SSE response and extract JSON
	jsonData, err := s.parseSSEResponse(body)
	if err != nil {
		return nil, &SearchError{
			Code:    "sse_parse_failed",
			Message: "Failed to parse SSE response",
			Details: fmt.Sprintf("parse error: %s, body: %s", err.Error(), string(body)),
		}
	}

	// Parse MCP response from extracted JSON
	var mcpResp MCPResponse
	if err := json.Unmarshal(jsonData, &mcpResp); err != nil {
		return nil, &SearchError{
			Code:    "response_parse_failed",
			Message: "Failed to parse MCP response",
			Details: fmt.Sprintf("parse error: %s, body: %s", err.Error(), string(jsonData)),
		}
	}

	// Extract search results from MCP response
	return s.extractSearchResults(&mcpResp, jsonData)
}

// handleHTTPError converts HTTP errors to SearchError
func (s *service) handleHTTPError(statusCode int, body []byte) *SearchError {
	var errorResponse struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Details string `json:"details,omitempty"`
		} `json:"error"`
	}

	// Try to parse error response
	if err := json.Unmarshal(body, &errorResponse); err == nil && errorResponse.Error.Code != "" {
		// Check for session-related errors
		if strings.Contains(strings.ToLower(errorResponse.Error.Message), "session") ||
			strings.Contains(strings.ToLower(errorResponse.Error.Code), "session") {
			// Clear session on session errors
			s.sessionMux.Lock()
			s.sessionID = ""
			s.sessionMux.Unlock()
		}
		return &SearchError{
			Code:    errorResponse.Error.Code,
			Message: errorResponse.Error.Message,
			Details: errorResponse.Error.Details,
		}
	}

	// Fallback to generic error
	message := "HTTP request failed"
	switch statusCode {
	case 400:
		message = "Bad request - invalid parameters"
	case 401:
		message = "Unauthorized - invalid API key"
	case 403:
		message = "Forbidden - insufficient permissions"
	case 429:
		message = "Rate limit exceeded"
	case 500:
		message = "Internal server error"
	case 502:
		message = "Bad gateway"
	case 503:
		message = "Service unavailable"
	}

	return &SearchError{
		Code:    fmt.Sprintf("http_%d", statusCode),
		Message: message,
		Details: string(body),
	}
}

// extractSearchResults extracts search results from MCP response
func (s *service) extractSearchResults(mcpResp *MCPResponse, rawBody []byte) (*SearchResult, error) {
	// Check for MCP errors
	if mcpResp.Error != nil {
		return nil, &SearchError{
			Code:    "mcp_error",
			Message: mcpResp.Error.Message,
			Details: mcpResp.Error.Data,
		}
	}

	// Extract result from MCP response
	if mcpResp.Result == nil {
		return nil, &SearchError{
			Code:    "empty_response",
			Message: "MCP response returned no result",
			Details: string(rawBody),
		}
	}

	// Try to parse as tool result
	resultBytes, err := json.Marshal(mcpResp.Result)
	if err != nil {
		return nil, &SearchError{
			Code:    "result_marshal_failed",
			Message: "Failed to marshal MCP result",
			Details: err.Error(),
		}
	}

	var toolResult MCPToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		return nil, &SearchError{
			Code:    "tool_result_parse_failed",
			Message: "Failed to parse tool result",
			Details: fmt.Sprintf("parse error: %s, result: %s", err.Error(), string(resultBytes)),
		}
	}

	// Check if tool call resulted in an error
	if toolResult.IsError {
		errorMsg := "Tool call failed"
		if len(toolResult.Content) > 0 {
			errorMsg = toolResult.Content[0].Text
		}
		return nil, &SearchError{
			Code:    "tool_error",
			Message: errorMsg,
		}
	}

	// Extract text content and parse as JSON search results
	if len(toolResult.Content) == 0 {
		return &SearchResult{Results: []SearchItem{}}, nil
	}

	textContent := toolResult.Content[0].Text
	if textContent == "" {
		return &SearchResult{Results: []SearchItem{}}, nil
	}

	// Debug: log the raw text content
	fmt.Fprintf(os.Stderr, "Raw text content: %s\n", textContent)

	// The text content is JSON wrapped in quotes and escaped
	// First, unquote string if it starts and ends with quotes
	if strings.HasPrefix(textContent, `"`) && strings.HasSuffix(textContent, `"`) {
		unquoted, err := strconv.Unquote(textContent)
		if err != nil {
			// If unquoting fails, try to use as-is
		} else {
			textContent = unquoted
		}
	}

	// Try to parse the text content as JSON search results
	var searchResult SearchResult
	if err := json.Unmarshal([]byte(textContent), &searchResult); err != nil {
		// Try to parse as the actual response format from API (array of results)
		var apiResults []struct {
			Refer       string `json:"refer"`
			Title       string `json:"title"`
			Link        string `json:"link"`
			Media       string `json:"media"`
			Content     string `json:"content"`
			Icon        string `json:"icon"`
			PublishDate string `json:"publish_date"`
		}

		if err := json.Unmarshal([]byte(textContent), &apiResults); err == nil {
			var results []SearchItem
			for _, item := range apiResults {
				results = append(results, SearchItem{
					Title:         item.Title,
					URL:           item.Link,
					Snippet:       item.Content,
					SiteName:      "",
					SiteIcon:      item.Icon,
					PublishedDate: item.PublishDate,
					ContentType:   item.Media,
				})
			}
			return &SearchResult{Results: results}, nil
		}

		// If all parsing fails, return the text as a single result
		return &SearchResult{
			Results: []SearchItem{
				{
					Title:   "Search Result",
					Snippet: textContent,
					URL:     "",
				},
			},
		}, nil
	}

	return &searchResult, nil
}

// parseSSEResponse parses Server-Sent Events format and extracts JSON data
func (s *service) parseSSEResponse(body []byte) ([]byte, error) {
	bodyStr := string(body)
	lines := strings.Split(bodyStr, "\n")

	var dataLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			dataContent := strings.TrimPrefix(line, "data:")
			dataContent = strings.TrimSpace(dataContent)
			if dataContent != "" {
				dataLines = append(dataLines, dataContent)
			}
		}
	}

	if len(dataLines) == 0 {
		return nil, fmt.Errorf("no data found in SSE response")
	}

	// Join all data lines (usually just one)
	jsonData := strings.Join(dataLines, "")
	return []byte(jsonData), nil
}
