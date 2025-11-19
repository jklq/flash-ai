package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	fsrs "github.com/open-spaced-repetition/go-fsrs"

	"flash-ai/internal/models"
)

// ProgressCallback is called during document processing to report progress
type ProgressCallback func(step, message string, current, total int)

// IngestionService coordinates PDF parsing, AI extraction, and persistence.
type IngestionService struct {
	documents *DocumentService
	pdf       *PDFService
	ai        *AIService
	cards     *FlashcardService
	concepts  *ConceptService
}

func NewIngestionService(
	documents *DocumentService,
	pdf *PDFService,
	ai *AIService,
	cards *FlashcardService,
	concepts *ConceptService,
) *IngestionService {
	return &IngestionService{
		documents: documents,
		pdf:       pdf,
		ai:        ai,
		cards:     cards,
		concepts:  concepts,
	}
}

func (s *IngestionService) ProcessExamDocument(ctx context.Context, doc *models.Document) (*ExamExtraction, error) {
	return s.ProcessExamDocumentWithProgress(ctx, doc, nil)
}

func (s *IngestionService) ProcessExamDocumentWithProgress(ctx context.Context, doc *models.Document, progress ProgressCallback) (*ExamExtraction, error) {
	if s.ai == nil {
		return nil, ErrAIUnavailable
	}

	if progress != nil {
		progress("extract", "Starting exam topic extraction", 0, 100)
	}

	extraction, err := s.ai.ExtractExamTopicsWithProgress(ctx, doc.StoredPath, progress)
	if err != nil {
		return nil, err
	}

	if progress != nil {
		progress("save", "Saving topics to database", 90, 100)
	}

	for i, topic := range extraction.Topics {
		if topic.Frequency <= 0 {
			topic.Frequency = 1
		}
		modelTopic := models.DocumentTopic{
			DocumentID: doc.ID,
			Topic:      topic.Name,
			Frequency:  topic.Frequency,
		}
		if _, err := s.concepts.UpsertExamTopic(ctx, modelTopic, topic.Description); err != nil {
			return nil, fmt.Errorf("upsert concept %s: %w", topic.Name, err)
		}
		if progress != nil && len(extraction.Topics) > 0 {
			pct := 90 + (10 * (i + 1) / len(extraction.Topics))
			progress("save", fmt.Sprintf("Saved topic: %s", topic.Name), pct, 100)
		}
	}

	if progress != nil {
		progress("complete", "Processing complete", 100, 100)
	}

	return extraction, nil
}

func (s *IngestionService) ProcessInformationDocument(ctx context.Context, doc *models.Document) (*FlashcardExtraction, error) {
	return s.ProcessInformationDocumentWithProgress(ctx, doc, nil)
}

func (s *IngestionService) ProcessInformationDocumentWithProgress(ctx context.Context, doc *models.Document, progress ProgressCallback) (*FlashcardExtraction, error) {
	// We'll get page count from the AI model's response
	if s.ai == nil {
		return nil, ErrAIUnavailable
	}

	if progress != nil {
		progress("concepts", "Loading prior concepts", 0, 100)
	}

	allConcepts, err := s.concepts.ListConcepts(ctx, 100)
	if err != nil {
		return nil, fmt.Errorf("list concepts: %w", err)
	}

	focusConcepts := allConcepts
	if len(focusConcepts) > 12 {
		focusConcepts = focusConcepts[:12]
	}

	if progress != nil {
		progress("flashcards", "Loading existing flashcards", 2, 100)
	}

	cardSummaries, err := s.cards.ListCardSummaries(ctx, 120)
	if err != nil {
		return nil, fmt.Errorf("list existing flashcards: %w", err)
	}

	promptCtx := FlashcardPromptContext{
		FocusConcepts:    focusConcepts,
		ExistingConcepts: allConcepts,
		ExistingCards:    cardSummaries,
	}

	if progress != nil {
		progress("extract", "Extracting flashcards from document", 5, 100)
	}

	extraction, err := s.ai.GenerateFlashcardsWithProgress(ctx, doc.StoredPath, promptCtx, progress)
	if err != nil {
		return nil, err
	}

	if progress != nil {
		progress("save", "Saving flashcards to database", 80, 100)
	}

	totalConcepts := len(extraction.Concepts)
	for conceptIdx, concept := range extraction.Concepts {
		if strings.TrimSpace(concept.Name) == "" || len(concept.Cards) == 0 {
			continue
		}
		record, err := s.concepts.TouchConcept(ctx, concept.Name, concept.Description)
		if err != nil {
			return nil, fmt.Errorf("touch concept %s: %w", concept.Name, err)
		}

		var cards []models.Card
		for _, proto := range concept.Cards {
			if strings.TrimSpace(proto.Front) == "" || strings.TrimSpace(proto.Back) == "" {
				continue
			}
			card := models.Card{
				ConceptID:        sql.NullInt64{Valid: true, Int64: record.ID},
				SourceDocumentID: sql.NullInt64{Valid: true, Int64: doc.ID},
				Front:            strings.TrimSpace(proto.Front),
				Back:             strings.TrimSpace(proto.Back),
				Due:              sql.NullTime{}, // assigned in BulkUpsertCards
				Stability:        0,
				Difficulty:       0,
				ElapsedDays:      0,
				ScheduledDays:    0,
				Reps:             0,
				Lapses:           0,
				State:            int(fsrs.New),
			}
			cards = append(cards, card)
		}
		if err := s.cards.BulkUpsertCards(ctx, sql.NullInt64{Valid: true, Int64: record.ID}, sql.NullInt64{Valid: true, Int64: doc.ID}, cards); err != nil {
			return nil, fmt.Errorf("insert cards for concept %s: %w", concept.Name, err)
		}

		if progress != nil && totalConcepts > 0 {
			pct := 80 + (20 * (conceptIdx + 1) / totalConcepts)
			progress("save", fmt.Sprintf("Saved %d cards for: %s", len(concept.Cards), concept.Name), pct, 100)
		}
	}

	if progress != nil {
		progress("complete", "Processing complete", 100, 100)
	}

	return extraction, nil
}
