package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ledongthuc/pdf"
)

// service implements the OCRService interface
type service struct {
	vision *visionService
	pdf    *pdfService
}

// NewOCRService creates a new OCR service with the given configuration
func NewOCRService(config Config) OCRService {
	svc := &service{
		vision: newVisionService(config.ZAIKey, config.ZAIBaseURL, config.ZAIModel),
		pdf:    newPDFService(),
	}
	return svc
}

// AnalyzeImage implements OCRService
func (s *service) AnalyzeImage(ctx context.Context, imageDataURI string, prompt string) (string, error) {
	if s.vision != nil && s.vision.isConfigured() {
		return s.vision.AnalyzeImage(ctx, imageDataURI, prompt)
	}
	
	return "", fmt.Errorf("OCR service not configured")
}

// AnalyzeMultipleImages implements OCRService
func (s *service) AnalyzeMultipleImages(ctx context.Context, imageDataURIs []string, prompt string) (string, error) {
	if s.vision != nil && s.vision.isConfigured() {
		return s.vision.AnalyzeMultipleImages(ctx, imageDataURIs, prompt)
	}
	
	return "", fmt.Errorf("OCR service not configured")
}

// AnalyzeImages implements OCRService
func (s *service) AnalyzeImages(ctx context.Context, imageDataURIs []string, prompt string) ([]string, error) {
	if s.vision != nil && s.vision.isConfigured() {
		return s.vision.AnalyzeImages(ctx, imageDataURIs, prompt)
	}
	
	return nil, fmt.Errorf("OCR service not configured")
}

// AnalyzeImagesWithProgress implements OCRService
func (s *service) AnalyzeImagesWithProgress(
	ctx context.Context,
	imageDataURIs []string,
	prompt string,
	progressFn func(page, total int, content string),
) ([]string, error) {
	if s.vision != nil && s.vision.isConfigured() {
		return s.vision.AnalyzeImagesWithProgress(ctx, imageDataURIs, prompt, progressFn)
	}
	
	return nil, fmt.Errorf("OCR service not configured")
}

// ConvertPDFToImages implements OCRService
func (s *service) ConvertPDFToImages(path string) ([]PDFPageImage, error) {
	return s.pdf.ConvertPDFToImages(path)
}

// ReadPDFBytes implements OCRService
func (s *service) ReadPDFBytes(path string) ([]byte, error) {
	return s.pdf.ReadPDFBytes(path)
}

// visionService handles Z.AI Vision API operations
type visionService struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

func newVisionService(apiKey, baseURL, model string) *visionService {
	if baseURL == "" {
		baseURL = "https://api.z.ai/api/coding/paas/v4/"
	}
	// Ensure baseURL ends with /
	if baseURL != "" && baseURL[len(baseURL)-1] != '/' {
		baseURL = baseURL + "/"
	}
	if model == "" {
		model = "glm-4.5v"
	}

	return &visionService{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // 5 minutes timeout
		},
	}
}

func (s *visionService) isConfigured() bool {
	return s.apiKey != ""
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

func (s *visionService) AnalyzeImage(ctx context.Context, imageDataURI string, prompt string) (string, error) {
	return s.AnalyzeMultipleImages(ctx, []string{imageDataURI}, prompt)
}

func (s *visionService) AnalyzeMultipleImages(ctx context.Context, imageDataURIs []string, prompt string) (string, error) {
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

func (s *visionService) AnalyzeImages(ctx context.Context, imageDataURIs []string, prompt string) ([]string, error) {
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

func (s *visionService) AnalyzeImagesWithProgress(
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

// pdfService handles PDF operations
type pdfService struct{}

func newPDFService() *pdfService {
	return &pdfService{}
}

func (s *pdfService) ReadPDFBytes(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	return bytes, nil
}

func (s *pdfService) ConvertPDFToImages(path string) ([]PDFPageImage, error) {
	// First, get the number of pages using the pdf library
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf for page count: %w", err)
	}
	numPages := r.NumPage()
	f.Close()

	if numPages == 0 {
		return nil, fmt.Errorf("pdf has no pages")
	}

	// Create a temporary directory for rendered images
	tempDir, err := os.MkdirTemp("", "pdf-render-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Use Ghostscript to render all pages at once
	// -dNOPAUSE -dBATCH: non-interactive mode
	// -sDEVICE=png16m: 24-bit color PNG
	// -r150: 150 DPI resolution (good balance between quality and size)
	// -sOutputFile: output pattern with %d for page numbers
	outputPattern := filepath.Join(tempDir, "page-%03d.png")
	cmd := exec.Command("gs",
		"-dQUIET",
		"-dSAFER",
		"-dNOPAUSE",
		"-dBATCH",
		"-sDEVICE=png16m",
		"-r150",
		fmt.Sprintf("-sOutputFile=%s", outputPattern),
		path,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ghostscript render failed: %w, stderr: %s", err, stderr.String())
	}

	// Read each rendered page and convert to base64
	var pages []PDFPageImage
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		// Ghostscript uses 1-based numbering in output
		pagePath := filepath.Join(tempDir, fmt.Sprintf("page-%03d.png", pageNum))

		// Read the rendered PNG file
		imageData, err := os.ReadFile(pagePath)
		if err != nil {
			return nil, fmt.Errorf("read rendered page %d: %w", pageNum, err)
		}

		// Encode to base64 data URI
		base64Data := base64.StdEncoding.EncodeToString(imageData)
		dataURI := "data:image/png;base64," + base64Data

		pages = append(pages, PDFPageImage{
			PageNumber: pageNum,
			ImageData:  dataURI,
		})
	}

	return pages, nil
}