package ocr_test

import (
	"context"
	"fmt"
	"log"

	"flash-ai/pkg/ocr"
)

func ExampleOCRService_AnalyzeImage() {
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
}

func ExampleOCRService_ConvertPDFToImages() {
	// Configure the OCR service
	config := ocr.Config{
		ZAIKey:   "your-zai-api-key",
		ZAIModel: "glm-4.5v",
	}

	// Create OCR service
	ocrService := ocr.NewOCRService(config)

	// Convert PDF to images
	pages, err := ocrService.ConvertPDFToImages("document.pdf")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Converted PDF to %d images\n", len(pages))
	for _, page := range pages {
		fmt.Printf("Page %d: %s\n", page.PageNumber, page.ImageData[:50]+"...")
	}
}

func ExampleOCRService_AnalyzeImagesWithProgress() {
	// Configure the OCR service
	config := ocr.Config{
		ZAIKey:   "your-zai-api-key",
		ZAIModel: "glm-4.5v",
	}

	// Create OCR service
	ocrService := ocr.NewOCRService(config)

	// Convert PDF to images first
	pages, err := ocrService.ConvertPDFToImages("document.pdf")
	if err != nil {
		log.Fatal(err)
	}

	// Extract image URIs
	imageURIs := make([]string, len(pages))
	for i, page := range pages {
		imageURIs[i] = page.ImageData
	}

	// Analyze with progress callback
	results, err := ocrService.AnalyzeImagesWithProgress(
		context.Background(),
		imageURIs,
		"Extract all text from each page",
		func(page, total int, content string) {
			fmt.Printf("Completed page %d/%d\n", page, total)
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Analyzed %d pages\n", len(results))
}