# OCR Package

The `ocr` package provides a comprehensive interface for Optical Character Recognition (OCR) operations, including PDF processing and image analysis using Z.AI Vision models.

## Features

- **Z.AI Vision API**: High-quality OCR and image analysis
- **PDF processing**: Convert PDF pages to images for OCR analysis
- **Batch processing**: Analyze multiple images efficiently
- **Progress tracking**: Built-in progress callbacks for long-running operations
- **Error handling**: Robust error handling with retry logic

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "flash-ai/pkg/ocr"
)

func main() {
    // Configure the OCR service
    config := ocr.Config{
        ZAIKey:     "your-zai-api-key",
        ZAIBaseURL: "https://api.z.ai/api/coding/paas/v4/",
        ZAIModel:   "glm-4.5v",
    }
    
    // Create OCR service
    ocrService := ocr.NewOCRService(config)
    
    // Analyze a single image
    result, err := ocrService.AnalyzeImage(
        context.Background(),
        "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
        "Extract all text from this image",
    )
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("OCR Result:", result)
    
    // Convert PDF to images
    pages, err := ocrService.ConvertPDFToImages("document.pdf")
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Converted PDF to %d images\n", len(pages))
}
```

## Configuration

The OCR service can be configured with the following options:

### Z.AI Vision API

- `ZAIKey`: API key for Z.AI Vision service
- `ZAIBaseURL`: Base URL for Z.AI API (defaults to "https://api.z.ai/api/coding/paas/v4/")
- `ZAIModel`: Model name (defaults to "glm-4.5v")

## API Reference

### OCRService Interface

```go
type OCRService interface {
    AnalyzeImage(ctx context.Context, imageDataURI string, prompt string) (string, error)
    AnalyzeMultipleImages(ctx context.Context, imageDataURIs []string, prompt string) (string, error)
    AnalyzeImages(ctx context.Context, imageDataURIs []string, prompt string) ([]string, error)
    AnalyzeImagesWithProgress(ctx context.Context, imageDataURIs []string, prompt string, progressFn func(page, total int, content string)) ([]string, error)
    ConvertPDFToImages(path string) ([]PDFPageImage, error)
    ReadPDFBytes(path string) ([]byte, error)
}
```

### Methods

#### AnalyzeImage
Analyzes a single image using the configured vision API.

**Parameters:**
- `ctx`: Context for the operation
- `imageDataURI`: Image as base64 data URI (e.g., "data:image/png;base64,...")
- `prompt`: Text prompt for the vision model

**Returns:**
- `string`: Analysis result
- `error`: Error if any

#### AnalyzeMultipleImages
Analyzes multiple images in a single API call (more efficient than individual calls).

**Parameters:**
- `ctx`: Context for the operation
- `imageDataURIs`: Slice of image data URIs
- `prompt`: Text prompt for the vision model

**Returns:**
- `string`: Combined analysis result
- `error`: Error if any

#### AnalyzeImages
Analyzes multiple images sequentially, returning individual results for each image.

**Parameters:**
- `ctx`: Context for the operation
- `imageDataURIs`: Slice of image data URIs
- `prompt`: Text prompt for the vision model

**Returns:**
- `[]string`: Individual analysis results for each image
- `error`: Error if any

#### AnalyzeImagesWithProgress
Analyzes multiple images with progress reporting.

**Parameters:**
- `ctx`: Context for the operation
- `imageDataURIs`: Slice of image data URIs
- `prompt`: Text prompt for the vision model
- `progressFn`: Callback function called for each completed image

**Returns:**
- `[]string`: Individual analysis results for each image
- `error`: Error if any

#### ConvertPDFToImages
Converts each page of a PDF to base64-encoded PNG images.

**Parameters:**
- `path`: Path to the PDF file

**Returns:**
- `[]PDFPageImage`: Slice of page images with page numbers
- `error`: Error if any

#### ReadPDFBytes
Reads a PDF file and returns its bytes.

**Parameters:**
- `path`: Path to the PDF file

**Returns:**
- `[]byte`: PDF file bytes
- `error`: Error if any

### Types

#### PDFPageImage
```go
type PDFPageImage struct {
    PageNumber int
    ImageData  string // base64 encoded image with data URI prefix
}
```

#### ProgressCallback
```go
type ProgressCallback func(stage, message string, current, total int)
```

## Dependencies

- `github.com/ledongthuc/pdf`: PDF processing
- Ghostscript: Required for PDF to image conversion (must be installed on system)

## Error Handling

The package provides comprehensive error handling:

- API errors are wrapped with context
- Retry logic for transient failures
- Detailed error messages for debugging

## Performance Considerations

- Use `AnalyzeMultipleImages` for batch processing when possible
- PDF conversion uses Ghostscript for high-quality rendering
- Progress callbacks help with user experience for long operations
- Concurrent processing is handled internally for optimal performance

## License

This package is part of the Flash-AI project.