package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ZAIVisionService handles direct API calls to Z.AI Vision API
// Based on the MCP server implementation from @z_ai/mcp-server
type ZAIVisionService struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewZAIVisionService creates a new Z.AI Vision service
func NewZAIVisionService(apiKey, baseURL, model string) *ZAIVisionService {
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4/"
	}
	// Ensure baseURL ends with /
	if baseURL != "" && baseURL[len(baseURL)-1] != '/' {
		baseURL = baseURL + "/"
	}
	if model == "" {
		model = "glm-4.5v"
	}

	return &ZAIVisionService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // 5 minutes timeout
		},
	}
}

// MessageContent represents a part of a message (text or image)
type MessageContent struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL represents an image URL or base64 data URI
type ImageURL struct {
	URL string `json:"url"`
}

// ChatMessage represents a message in the chat
type ChatMessage struct {
	Role    string           `json:"role"`
	Content []MessageContent `json:"content"`
}

// ThinkingConfig enables thinking mode
type ThinkingConfig struct {
	Type string `json:"type"`
}

// VisionRequest represents the request to Z.AI Vision API
type VisionRequest struct {
	Model       string         `json:"model"`
	Messages    []ChatMessage  `json:"messages"`
	Thinking    ThinkingConfig `json:"thinking"`
	Stream      bool           `json:"stream"`
	Temperature float64        `json:"temperature"`
	TopP        float64        `json:"top_p"`
	MaxTokens   int            `json:"max_tokens"`
}

// VisionChoice represents a single choice in the response
type VisionChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// VisionResponse represents the response from Z.AI Vision API
type VisionResponse struct {
	ID      string         `json:"id"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []VisionChoice `json:"choices"`
	Usage   struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// AnalyzeImage analyzes a single image (from URL or base64 data URI)
func (s *ZAIVisionService) AnalyzeImage(ctx context.Context, imageDataURI string, prompt string) (string, error) {
	return s.AnalyzeMultipleImages(ctx, []string{imageDataURI}, prompt)
}

// AnalyzeMultipleImages analyzes multiple images in a single API call
func (s *ZAIVisionService) AnalyzeMultipleImages(ctx context.Context, imageDataURIs []string, prompt string) (string, error) {
	// Create content array with all images followed by the text prompt
	content := make([]MessageContent, 0, len(imageDataURIs)+1)

	// Add all images
	for _, imageURI := range imageDataURIs {
		content = append(content, MessageContent{
			Type: "image_url",
			ImageURL: &ImageURL{
				URL: imageURI,
			},
		})
	}

	// Add text prompt at the end
	content = append(content, MessageContent{
		Type: "text",
		Text: prompt,
	})

	// Create multimodal message with all images and text
	messages := []ChatMessage{
		{
			Role:    "user",
			Content: content,
		},
	}

	// Create request
	request := VisionRequest{
		Model:    s.model,
		Messages: messages,
		Thinking: ThinkingConfig{
			Type: "enabled",
		},
		Stream:      false,
		Temperature: 0.8,
		TopP:        0.6,
		MaxTokens:   16384,
	}

	// Marshal request
	reqBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("marshal vision request: %w", err)
	}

	// Log payload size for debugging
	payloadSizeKB := len(reqBody) / 1024
	fmt.Fprintf(os.Stderr, "Vision API request: %d images, payload size: %d KB\n", len(imageDataURIs), payloadSizeKB)

	// Retry logic for transient failures
	maxRetries := 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(os.Stderr, "Retrying vision API call (attempt %d/%d)...\n", attempt+1, maxRetries+1)
			// Wait before retry
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		// Create HTTP request
		url := s.baseURL + "chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
		if err != nil {
			lastErr = fmt.Errorf("create http request: %w", err)
			continue
		}

		// Set headers (matching the MCP server implementation)
		httpReq.Header.Set("Authorization", "Bearer "+s.apiKey)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("X-Title", "Flash-AI Vision")
		httpReq.Header.Set("Accept-Language", "en-US,en")

		// Execute request
		resp, err := s.httpClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("execute vision request: %w", err)
			continue
		}

		// Read response body
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response body: %w", err)
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("vision api error: status=%d, body=%s", resp.StatusCode, string(body))
			// Don't retry 4xx errors (client errors)
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				return "", lastErr
			}
			continue
		}

		// Parse response
		var visionResp VisionResponse
		if err := json.Unmarshal(body, &visionResp); err != nil {
			lastErr = fmt.Errorf("unmarshal vision response: %w, body=%s", err, string(body))
			continue
		}

		// Extract content
		if len(visionResp.Choices) == 0 {
			lastErr = fmt.Errorf("vision api returned no choices, response: %s", string(body))
			continue
		}

		result := visionResp.Choices[0].Message.Content
		if result == "" {
			// Log the full response for debugging
			fmt.Fprintf(os.Stderr, "WARNING: Vision API returned empty content. Response: %s\n", string(body))
			lastErr = fmt.Errorf("vision api returned empty content (attempt %d/%d)", attempt+1, maxRetries+1)
			continue
		}

		// Success!
		return result, nil
	}

	// All retries exhausted
	return "", fmt.Errorf("vision api failed after %d attempts: %w", maxRetries+1, lastErr)
}

// AnalyzeImages analyzes multiple images sequentially
func (s *ZAIVisionService) AnalyzeImages(ctx context.Context, imageDataURIs []string, prompt string) ([]string, error) {
	results := make([]string, 0, len(imageDataURIs))

	for i, imageData := range imageDataURIs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := s.AnalyzeImage(ctx, imageData, prompt)
		if err != nil {
			return nil, fmt.Errorf("analyze image %d: %w", i+1, err)
		}

		results = append(results, result)
	}

	return results, nil
}

// AnalyzeImagesWithProgress analyzes multiple images and calls a progress callback
func (s *ZAIVisionService) AnalyzeImagesWithProgress(
	ctx context.Context,
	imageDataURIs []string,
	prompt string,
	progressFn func(page, total int, content string),
) ([]string, error) {
	results := make([]string, 0, len(imageDataURIs))
	total := len(imageDataURIs)

	for i, imageData := range imageDataURIs {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := s.AnalyzeImage(ctx, imageData, prompt)
		if err != nil {
			return nil, fmt.Errorf("analyze page %d of %d: %w", i+1, total, err)
		}

		results = append(results, result)

		if progressFn != nil {
			progressFn(i+1, total, result)
		}
	}

	return results, nil
}
