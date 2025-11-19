package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"flash-ai/internal/models"
)

var (
	// ErrAIUnavailable is returned when the OpenAI integration is not configured.
	ErrAIUnavailable = errors.New("openai integration is not configured")
)

type AIService struct {
	client *openai.Client
	model  string
	vision *ZAIVisionService // Direct Z.AI Vision API client
	pdf    *PDFService       // PDF to image conversion
}

func NewAIService(apiKey string, model string, apiEndpoint string, zaiKey string, zaiBaseURL string, zaiModel string, pdfService *PDFService) *AIService {
	if apiKey == "" && zaiKey == "" {
		return &AIService{}
	}

	var client *openai.Client
	if apiKey != "" {
		cfg := openai.DefaultConfig(apiKey)
		if apiEndpoint != "" {
			cfg.BaseURL = apiEndpoint
		}
		client = openai.NewClientWithConfig(cfg)
	}

	var vision *ZAIVisionService
	if zaiKey != "" {
		vision = NewZAIVisionService(zaiKey, zaiBaseURL, zaiModel)
	}

	return &AIService{
		client: client,
		model:  model,
		vision: vision,
		pdf:    pdfService,
	}
}

type FlashcardConcept struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Cards       []FlashcardPrototype `json:"cards"`
}

type FlashcardPrototype struct {
	Front string `json:"front"`
	Back  string `json:"back"`
}

type FlashcardExtraction struct {
	Concepts []FlashcardConcept `json:"concepts"`
	Notes    string             `json:"notes"`
}

// FlashcardPromptContext carries existing knowledge data into flashcard generation prompts.
type FlashcardPromptContext struct {
	FocusConcepts    []models.Concept
	ExistingConcepts []models.Concept
	ExistingCards    []models.CardSummary
}

type ExamTopicResult struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Frequency   int           `json:"frequency"`
	References  []interface{} `json:"references"`
}

type ExamExtraction struct {
	Topics []ExamTopicResult `json:"topics"`
	Notes  string            `json:"notes"`
}

func (s *AIService) disabled() bool {
	return s.client == nil || s.model == ""
}

// extractJSON removes markdown code block formatting if present and extracts the JSON
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// Remove markdown code blocks like ```json ... ``` or ``` ... ```
	if strings.HasPrefix(content, "```") {
		// Skip past the opening ``` and optional language identifier (e.g., "json")
		start := 3
		// Find the first newline to skip the language identifier line
		if newlineIdx := strings.Index(content[start:], "\n"); newlineIdx != -1 {
			start += newlineIdx + 1
		}

		// Find the closing ```
		if endIdx := strings.Index(content[start:], "```"); endIdx != -1 {
			content = content[start : start+endIdx]
		} else {
			// No closing ```, just take everything after the opening
			content = content[start:]
		}
	}

	content = strings.TrimSpace(content)

	// Additional safety: find the first { and last } to extract just the JSON object
	if startIdx := strings.Index(content, "{"); startIdx != -1 {
		if endIdx := strings.LastIndex(content, "}"); endIdx != -1 && endIdx > startIdx {
			content = content[startIdx : endIdx+1]
		}
	}

	return strings.TrimSpace(content)
}

const (
	maxContextConcepts = 30
	maxContextCards    = 80
)

func buildFocusPrompt(concepts []models.Concept) string {
	if len(concepts) == 0 {
		return "No exam data is available. Choose the most instructionally important concepts."
	}

	var builder strings.Builder
	builder.WriteString("Focus on these high-priority exam concepts (name:weight):\n")
	for _, concept := range concepts {
		name := sanitizeForPrompt(concept.Name, 120)
		if name == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("- %s (weight %.2f)\n", name, concept.Weight))
	}
	return builder.String()
}

func buildExistingKnowledgePrompt(concepts []models.Concept, cards []models.CardSummary) string {
	if len(concepts) == 0 && len(cards) == 0 {
		return "Existing knowledge base is empty. Create foundational coverage without duplicating content."
	}

	var builder strings.Builder
	builder.WriteString("Existing knowledge base:\n")

	if len(concepts) == 0 {
		builder.WriteString("- No concepts recorded yet.\n")
	} else {
		builder.WriteString("Concepts already covered:\n")
		for i, concept := range concepts {
			if i >= maxContextConcepts {
				builder.WriteString("- (additional concepts omitted)\n")
				break
			}
			name := sanitizeForPrompt(concept.Name, 120)
			if name == "" {
				name = "Unnamed concept"
			}
			builder.WriteString("- " + name)
			if concept.Description.Valid {
				desc := sanitizeForPrompt(concept.Description.String, 180)
				if desc != "" {
					builder.WriteString(": " + desc)
				}
			}
			builder.WriteString("\n")
		}
	}

	if len(cards) == 0 {
		builder.WriteString("\nNo flashcards exist yet.\n")
		return builder.String()
	}

	builder.WriteString("\nFlashcards already in the collection (avoid duplicating these):\n")
	written := 0
	for _, card := range cards {
		if written >= maxContextCards {
			builder.WriteString("- (additional cards omitted)\n")
			break
		}
		front := sanitizeForPrompt(card.Front, 200)
		back := sanitizeForPrompt(card.Back, 200)
		if front == "" || back == "" {
			continue
		}
		concept := sanitizeForPrompt(card.ConceptName, 120)
		if concept == "" {
			concept = "Unassigned"
		}
		builder.WriteString(fmt.Sprintf("- [%s] Q: %s | A: %s\n", concept, front, back))
		written++
	}

	return builder.String()
}

func sanitizeForPrompt(input string, limit int) string {
	collapsed := strings.Join(strings.Fields(strings.TrimSpace(input)), " ")
	if limit <= 0 {
		return collapsed
	}
	runes := []rune(collapsed)
	if len(runes) <= limit {
		return collapsed
	}
	if limit > 3 {
		return string(runes[:limit-3]) + "..."
	}
	return string(runes[:limit])
}

func (s *AIService) ExtractExamTopics(ctx context.Context, pdfPath string) (*ExamExtraction, error) {
	return s.ExtractExamTopicsWithProgress(ctx, pdfPath, nil)
}

func (s *AIService) ExtractExamTopicsWithProgress(ctx context.Context, pdfPath string, progress ProgressCallback) (*ExamExtraction, error) {
	if s.disabled() {
		return nil, ErrAIUnavailable
	}

	// Use Z.AI Vision API if available, otherwise fall back to OpenAI
	if s.vision != nil && s.pdf != nil {
		return s.extractExamTopicsWithVisionAndProgress(ctx, pdfPath, progress)
	}

	// Fallback to OpenAI with base64 PDF
	pdfData, err := s.pdf.ReadPDFBytes(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	payload := `Strictly respond with a JSON object {"topics":[{"name":"","description":"","frequency":0,"references":[]}], "notes":""}. Frequency is an integer representing how many times the concept/skill is targeted by the exam, inferred from the questions. If uncertain, choose a reasonable lower bound >=1. Include at most 12 topics, sorted by frequency descending. Summarize recurring skills or knowledge points concisely.`

	base64PDF := base64.StdEncoding.EncodeToString(pdfData)
	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an analyst who distills exam PDFs into skill frequency counts to drive spaced repetition planning.",
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: fmt.Sprintf("%s\n\nAnalyze this PDF content:", payload),
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: "data:application/pdf;base64," + base64PDF,
						},
					},
				},
			},
		},
		Temperature: 0.2,
		MaxTokens:   4096,
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request openai exam topics: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai returned no choices")
	}

	var extraction ExamExtraction
	jsonStr := extractJSON(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(jsonStr), &extraction); err != nil {
		// Log the raw response for debugging
		fmt.Fprintf(os.Stderr, "Failed to unmarshal exam topics. Raw response:\n%s\n", resp.Choices[0].Message.Content)
		fmt.Fprintf(os.Stderr, "Extracted JSON:\n%s\n", jsonStr)
		return nil, fmt.Errorf("unmarshal exam topics json: %w", err)
	}
	return &extraction, nil
}

// extractExamTopicsWithVision uses Z.AI Vision API to analyze each PDF page as an image
func (s *AIService) extractExamTopicsWithVision(ctx context.Context, pdfPath string) (*ExamExtraction, error) {
	return s.extractExamTopicsWithVisionAndProgress(ctx, pdfPath, nil)
}

func (s *AIService) extractExamTopicsWithVisionAndProgress(ctx context.Context, pdfPath string, progress ProgressCallback) (*ExamExtraction, error) {
	fmt.Fprintf(os.Stderr, "Converting PDF to images for vision analysis...\n")

	if progress != nil {
		progress("convert", "Converting PDF to images", 0, 100)
	}

	// Convert PDF pages to images
	pages, err := s.pdf.ConvertPDFPagesToImages(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("convert pdf to images: %w", err)
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages extracted from pdf")
	}

	if progress != nil {
		progress("analyze", fmt.Sprintf("Analyzing %d pages", len(pages)), 10, 100)
	}

	fmt.Fprintf(os.Stderr, "Processing %d pages with Z.AI Vision API (batched)...\n", len(pages))

	// Prepare prompt for vision analysis
	prompt := `Analyze these exam pages and identify key concepts, skills, or knowledge points being tested. 
For each page shown, extract the topics and estimate their importance based on question complexity and frequency.
Return your analysis as text describing the concepts found across all pages shown.`

	// Batch pages into groups (GLM-4.5v can handle multiple images per call)
	// Using smaller batches to avoid payload size limits and API timeouts
	batchSize := 8 // Smaller batch size to prevent "empty content" errors

	// Create batches
	type batch struct {
		start       int
		end         int
		pages       []PDFPageImage
		imageURIs   []string
		pageNumbers []int
	}

	var batches []batch
	for i := 0; i < len(pages); i += batchSize {
		end := i + batchSize
		if end > len(pages) {
			end = len(pages)
		}
		batchPages := pages[i:end]

		imageURIs := make([]string, len(batchPages))
		pageNumbers := make([]int, len(batchPages))
		for j, page := range batchPages {
			imageURIs[j] = page.ImageData
			pageNumbers[j] = page.PageNumber
		}

		batches = append(batches, batch{
			start:       i + 1,
			end:         end,
			pages:       batchPages,
			imageURIs:   imageURIs,
			pageNumbers: pageNumbers,
		})
	}

	// Process batches in parallel with max 10 concurrent calls
	type result struct {
		index    int
		analysis string
		err      error
	}

	results := make([]result, len(batches))
	var wg sync.WaitGroup
	var completedBatches int
	var mu sync.Mutex
	semaphore := make(chan struct{}, 10) // Max 10 concurrent API calls

	for i, b := range batches {
		wg.Add(1)
		go func(idx int, bt batch) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Fprintf(os.Stderr, "Analyzing pages %d-%d/%d...\n", bt.start, bt.end, len(pages))

			if progress != nil {
				pagesProcessed := bt.start - 1
				pct := 10 + (70 * pagesProcessed / len(pages))
				progress("analyze", fmt.Sprintf("Analyzing pages %d-%d of %d", bt.start, bt.end, len(pages)), pct, 100)
			}

			// Analyze all images in batch with a single API call
			analysis, err := s.vision.AnalyzeMultipleImages(ctx, bt.imageURIs, prompt)
			if err != nil {
				results[idx] = result{idx, "", fmt.Errorf("analyze pages %d-%d with vision: %w", bt.start, bt.end, err)}
				return
			}

			// Format with page range
			pageRange := fmt.Sprintf("Pages %d-%d", bt.pageNumbers[0], bt.pageNumbers[len(bt.pageNumbers)-1])
			results[idx] = result{idx, fmt.Sprintf("=== %s ===\n%s", pageRange, analysis), nil}

			// Report completion
			mu.Lock()
			completedBatches++
			if progress != nil {
				pct := 10 + (70 * bt.end / len(pages))
				progress("analyze", fmt.Sprintf("Completed pages %d-%d of %d", bt.start, bt.end, len(pages)), pct, 100)
			}
			mu.Unlock()
		}(i, b)
	}

	wg.Wait()

	// Check for errors and collect analyses in order
	var pageAnalyses []string
	for _, res := range results {
		if res.err != nil {
			return nil, res.err
		}
		pageAnalyses = append(pageAnalyses, res.analysis)
	}

	// Combine all page analyses
	combinedAnalysis := strings.Join(pageAnalyses, "\n\n")

	if progress != nil {
		progress("synthesize", "Synthesizing topics from all pages", 80, 100)
	}

	fmt.Fprintf(os.Stderr, "Synthesizing topics from all pages...\n")

	// Now use the LLM to synthesize the topics from all page analyses
	synthesisPrompt := `Based on the following analysis of exam pages, extract and synthesize the key topics.

Strictly respond with a JSON object {"topics":[{"name":"","description":"","frequency":0,"references":[]}], "notes":""}. 
Frequency is an integer representing how many times the concept/skill appears across pages (1-10 scale).
Include at most 12 topics, sorted by frequency descending.

Page Analyses:
` + combinedAnalysis

	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an analyst who synthesizes exam topics from detailed page analyses to drive spaced repetition planning.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: synthesisPrompt,
			},
		},
		Temperature: 0.2,
		MaxTokens:   4096,
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("synthesize exam topics: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("llm returned no choices")
	}

	var extraction ExamExtraction
	jsonStr := extractJSON(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(jsonStr), &extraction); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to unmarshal exam topics. Raw response:\n%s\n", resp.Choices[0].Message.Content)
		fmt.Fprintf(os.Stderr, "Extracted JSON:\n%s\n", jsonStr)
		return nil, fmt.Errorf("unmarshal exam topics json: %w", err)
	}

	return &extraction, nil
}

func (s *AIService) GenerateFlashcards(ctx context.Context, pdfPath string, promptCtx FlashcardPromptContext) (*FlashcardExtraction, error) {
	return s.GenerateFlashcardsWithProgress(ctx, pdfPath, promptCtx, nil)
}

func (s *AIService) GenerateFlashcardsWithProgress(ctx context.Context, pdfPath string, promptCtx FlashcardPromptContext, progress ProgressCallback) (*FlashcardExtraction, error) {
	if s.disabled() {
		return nil, ErrAIUnavailable
	}

	// Use Z.AI Vision API if available, otherwise fall back to OpenAI
	if s.vision != nil && s.pdf != nil {
		return s.generateFlashcardsWithVisionAndProgress(ctx, pdfPath, promptCtx, progress)
	}

	// Fallback to OpenAI with base64 PDF
	pdfData, err := s.pdf.ReadPDFBytes(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}

	focusPrompt := buildFocusPrompt(promptCtx.FocusConcepts)
	existingPrompt := buildExistingKnowledgePrompt(promptCtx.ExistingConcepts, promptCtx.ExistingCards)

	instruction := `Respond with JSON {"concepts":[{"name":"","description":"","cards":[{"front":"","back":""}]}], "notes":""}. 
Each concept must contain 2-4 cards. Ensure flashcards are atomic, unambiguous, and use active recall. 
Avoid repeating existing flashcards or concepts provided in the context below. 
Use Markdown sparingly in answers (only for essential formatting).`

	base64PDF := base64.StdEncoding.EncodeToString(pdfData)
	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert educator who designs spaced repetition flashcards using the FSRS algorithm.",
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: instruction + "\n\n" + focusPrompt + "\n\n" + existingPrompt + "\nAnalyze this PDF content:",
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: "data:application/pdf;base64," + base64PDF,
						},
					},
				},
			},
		},
		Temperature: 0.4,
		MaxTokens:   4096,
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("request openai flashcards: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai returned no choices")
	}

	var extraction FlashcardExtraction
	jsonStr := extractJSON(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(jsonStr), &extraction); err != nil {
		// Log the raw response for debugging
		fmt.Fprintf(os.Stderr, "Failed to unmarshal flashcards. Raw response:\n%s\n", resp.Choices[0].Message.Content)
		fmt.Fprintf(os.Stderr, "Extracted JSON:\n%s\n", jsonStr)
		return nil, fmt.Errorf("unmarshal flashcard json: %w", err)
	}
	return &extraction, nil
}

// generateFlashcardsWithVision uses Z.AI Vision API to analyze each PDF page as an image
func (s *AIService) generateFlashcardsWithVision(ctx context.Context, pdfPath string, promptCtx FlashcardPromptContext) (*FlashcardExtraction, error) {
	return s.generateFlashcardsWithVisionAndProgress(ctx, pdfPath, promptCtx, nil)
}

func (s *AIService) generateFlashcardsWithVisionAndProgress(ctx context.Context, pdfPath string, promptCtx FlashcardPromptContext, progress ProgressCallback) (*FlashcardExtraction, error) {
	fmt.Fprintf(os.Stderr, "Converting PDF to images for vision analysis...\n")

	if progress != nil {
		progress("convert", "Converting PDF to images", 10, 100)
	}

	// Convert PDF pages to images
	pages, err := s.pdf.ConvertPDFPagesToImages(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("convert pdf to images: %w", err)
	}

	if len(pages) == 0 {
		return nil, fmt.Errorf("no pages extracted from pdf")
	}

	if progress != nil {
		progress("analyze", fmt.Sprintf("Analyzing %d pages", len(pages)), 20, 100)
	}

	fmt.Fprintf(os.Stderr, "Processing %d pages with Z.AI Vision API (batched)...\n", len(pages))

	focusPrompt := buildFocusPrompt(promptCtx.FocusConcepts)

	// Prepare prompt for vision analysis
	prompt := `Analyze these pages and extract key educational content, facts, concepts, and information.
Focus on material that would be suitable for creating flashcards for spaced repetition learning.
Return your analysis as detailed text describing all important learnable content across all pages shown.

` + focusPrompt + "\n"

	// Batch pages into groups (GLM-4.5v can handle multiple images per call)
	batchSize := 2 // Smaller batch size to prevent "empty content" errors

	// Create batches
	type batch struct {
		start       int
		end         int
		pages       []PDFPageImage
		imageURIs   []string
		pageNumbers []int
	}

	var batches []batch
	for i := 0; i < len(pages); i += batchSize {
		end := i + batchSize
		if end > len(pages) {
			end = len(pages)
		}
		batchPages := pages[i:end]

		imageURIs := make([]string, len(batchPages))
		pageNumbers := make([]int, len(batchPages))
		for j, page := range batchPages {
			imageURIs[j] = page.ImageData
			pageNumbers[j] = page.PageNumber
		}

		batches = append(batches, batch{
			start:       i + 1,
			end:         end,
			pages:       batchPages,
			imageURIs:   imageURIs,
			pageNumbers: pageNumbers,
		})
	}

	// Process batches in parallel with max 10 concurrent calls
	type result struct {
		index    int
		analysis string
		err      error
	}

	results := make([]result, len(batches))
	var wg sync.WaitGroup
	var completedBatches int
	var mu sync.Mutex
	semaphore := make(chan struct{}, 10) // Max 10 concurrent API calls

	for i, b := range batches {
		wg.Add(1)
		go func(idx int, bt batch) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			fmt.Fprintf(os.Stderr, "Analyzing pages %d-%d/%d...\n", bt.start, bt.end, len(pages))

			if progress != nil {
				pagesProcessed := bt.start - 1
				pct := 20 + (50 * pagesProcessed / len(pages))
				progress("analyze", fmt.Sprintf("Analyzing pages %d-%d of %d", bt.start, bt.end, len(pages)), pct, 100)
			}

			// Analyze all images in batch with a single API call
			analysis, err := s.vision.AnalyzeMultipleImages(ctx, bt.imageURIs, prompt)
			if err != nil {
				results[idx] = result{idx, "", fmt.Errorf("analyze pages %d-%d with vision: %w", bt.start, bt.end, err)}
				return
			}

			// Format with page range
			pageRange := fmt.Sprintf("Pages %d-%d", bt.pageNumbers[0], bt.pageNumbers[len(bt.pageNumbers)-1])
			results[idx] = result{idx, fmt.Sprintf("=== %s ===\n%s", pageRange, analysis), nil}

			// Report completion
			mu.Lock()
			completedBatches++
			if progress != nil {
				pct := 20 + (50 * bt.end / len(pages))
				progress("analyze", fmt.Sprintf("Completed pages %d-%d of %d", bt.start, bt.end, len(pages)), pct, 100)
			}
			mu.Unlock()
		}(i, b)
	}

	wg.Wait()

	// Check for errors and collect analyses in order
	var pageAnalyses []string
	for _, res := range results {
		if res.err != nil {
			return nil, res.err
		}
		pageAnalyses = append(pageAnalyses, res.analysis)
	}

	// Combine all page analyses
	combinedAnalysis := strings.Join(pageAnalyses, "\n\n")

	if progress != nil {
		progress("synthesize", "Generating flashcards from content", 70, 100)
	}

	fmt.Fprintf(os.Stderr, "Generating flashcards from all pages...\n")

	// Now use the LLM to generate flashcards from all page analyses
	existingPrompt := buildExistingKnowledgePrompt(promptCtx.ExistingConcepts, promptCtx.ExistingCards)
	instruction := `Based on the following analysis of document pages, generate spaced repetition flashcards. Only make flashcards of information that is relevant to a potential exam.

Respond with JSON {"concepts":[{"name":"","description":"","cards":[{"front":"","back":""}]}], "notes":""}. 
Each concept must contain 2-4 cards. Ensure flashcards are atomic, unambiguous, and use active recall. 
Avoid repeating existing flashcards or concepts provided in the context below. 
Use Markdown sparingly in answers (only for essential formatting).

` + focusPrompt + `

Existing knowledge context:
` + existingPrompt + `

Page Analyses:
` + combinedAnalysis

	req := openai.ChatCompletionRequest{
		Model: s.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are an expert educator who designs spaced repetition flashcards using the FSRS algorithm.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: instruction,
			},
		},
		Temperature: 0.4,
		MaxTokens:   4096,
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	resp, err := s.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("generate flashcards: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("llm returned no choices")
	}

	var extraction FlashcardExtraction
	jsonStr := extractJSON(resp.Choices[0].Message.Content)
	if err := json.Unmarshal([]byte(jsonStr), &extraction); err != nil {
		// Log the raw response for debugging
		fmt.Fprintf(os.Stderr, "Failed to unmarshal flashcards. Raw response:\n%s\n", resp.Choices[0].Message.Content)
		fmt.Fprintf(os.Stderr, "Extracted JSON:\n%s\n", jsonStr)
		return nil, fmt.Errorf("unmarshal flashcard json: %w", err)
	}
	return &extraction, nil
}
