package ocr

import "context"

// OCRService defines the interface for OCR operations
type OCRService interface {
	// AnalyzeImage analyzes a single image (from URL or base64 data URI)
	AnalyzeImage(ctx context.Context, imageDataURI string, prompt string) (string, error)
	
	// AnalyzeMultipleImages analyzes multiple images in a single API call
	AnalyzeMultipleImages(ctx context.Context, imageDataURIs []string, prompt string) (string, error)
	
	// AnalyzeImages analyzes multiple images sequentially
	AnalyzeImages(ctx context.Context, imageDataURIs []string, prompt string) ([]string, error)
	
	// AnalyzeImagesWithProgress analyzes multiple images and calls a progress callback
	AnalyzeImagesWithProgress(
		ctx context.Context,
		imageDataURIs []string,
		prompt string,
		progressFn func(page, total int, content string),
	) ([]string, error)
	
	// ConvertPDFToImages converts each page of a PDF to base64-encoded PNG images
	ConvertPDFToImages(path string) ([]PDFPageImage, error)
	
	// ReadPDFBytes reads a PDF file and returns its bytes
	ReadPDFBytes(path string) ([]byte, error)
}

// PDFPageImage represents a single page converted to an image
type PDFPageImage struct {
	PageNumber int
	ImageData  string // base64 encoded image with data URI prefix
}

// ProgressCallback is a function type for progress reporting
type ProgressCallback func(stage, message string, current, total int)

// Config holds configuration for OCR services
type Config struct {
	// Z.AI Vision API configuration
	ZAIKey     string
	ZAIBaseURL string
	ZAIModel   string
}