package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs"

	"flash-ai/internal/models"
)

var (
	// ErrNoDueCards indicates that there are no cards ready to review.
	ErrNoDueCards = errors.New("no due cards")
)

// FlashcardService orchestrates card scheduling and persistence with FSRS.
type FlashcardService struct {
	db     *sql.DB
	params fsrs.Parameters
}

func NewFlashcardService(db *sql.DB) *FlashcardService {
	params := fsrs.DefaultParam()
	return &FlashcardService{db: db, params: params}
}

// NextCard returns the next card due for review with working queue support.
// Priority order: 1) Cards in working queue, 2) Due cards, 3) Oldest unseen card
func (s *FlashcardService) NextCard(ctx context.Context) (*models.Card, error) {
	now := time.Now().UTC()

	// First, check for cards in the working queue (cards marked "Again")
	card, err := s.fetchCard(ctx, `
		SELECT c.id, c.concept_id, c.source_document_id, c.front, c.back,
			   c.due, c.stability, c.difficulty, c.elapsed_days, c.scheduled_days,
			   c.reps, c.lapses, c.state, c.last_review, c.created_at, c.updated_at,
			   c.working_queue_position, co.name, d.original_name
		FROM cards c
		LEFT JOIN concepts co ON c.concept_id = co.id
		LEFT JOIN documents d ON c.source_document_id = d.id
		WHERE c.working_queue_position IS NOT NULL
		ORDER BY c.working_queue_position ASC
		LIMIT 1;
	`)
	if err == nil {
		return card, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Second, check for due cards
	card, err = s.fetchCard(ctx, `
		SELECT c.id, c.concept_id, c.source_document_id, c.front, c.back,
			   c.due, c.stability, c.difficulty, c.elapsed_days, c.scheduled_days,
			   c.reps, c.lapses, c.state, c.last_review, c.created_at, c.updated_at,
			   c.working_queue_position, co.name, d.original_name
		FROM cards c
		LEFT JOIN concepts co ON c.concept_id = co.id
		LEFT JOIN documents d ON c.source_document_id = d.id
		WHERE c.due IS NOT NULL AND c.due <= ? AND c.working_queue_position IS NULL
		ORDER BY c.due ASC
		LIMIT 1;
	`, now)
	if err == nil {
		return card, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Finally, return the oldest unseen card
	card, err = s.fetchCard(ctx, `
		SELECT c.id, c.concept_id, c.source_document_id, c.front, c.back,
			   c.due, c.stability, c.difficulty, c.elapsed_days, c.scheduled_days,
			   c.reps, c.lapses, c.state, c.last_review, c.created_at, c.updated_at,
			   c.working_queue_position, co.name, d.original_name
		FROM cards c
		LEFT JOIN concepts co ON c.concept_id = co.id
		LEFT JOIN documents d ON c.source_document_id = d.id
		WHERE c.working_queue_position IS NULL
		ORDER BY c.due IS NULL DESC, c.created_at ASC
		LIMIT 1;
	`)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoDueCards
		}
		return nil, err
	}
	return card, nil
}

func (s *FlashcardService) fetchCard(ctx context.Context, query string, args ...any) (*models.Card, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	card := &models.Card{}
	if err := row.Scan(
		&card.ID,
		&card.ConceptID,
		&card.SourceDocumentID,
		&card.Front,
		&card.Back,
		&card.Due,
		&card.Stability,
		&card.Difficulty,
		&card.ElapsedDays,
		&card.ScheduledDays,
		&card.Reps,
		&card.Lapses,
		&card.State,
		&card.LastReview,
		&card.CreatedAt,
		&card.UpdatedAt,
		&card.WorkingQueuePosition,
		&card.ConceptName,
		&card.SourceDocumentRef,
	); err != nil {
		return nil, err
	}
	return card, nil
}

// ReviewCard updates the scheduling information based on the user's rating.
func (s *FlashcardService) ReviewCard(ctx context.Context, cardID int64, rating fsrs.Rating) (*models.Card, *models.ReviewLog, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	card := &models.Card{}
	row := tx.QueryRowContext(ctx, `
		SELECT id, concept_id, source_document_id, front, back, due, stability, difficulty,
		       elapsed_days, scheduled_days, reps, lapses, state, last_review, created_at, updated_at,
		       working_queue_position
		FROM cards
		WHERE id = ?;
	`, cardID)
	if err = row.Scan(
		&card.ID,
		&card.ConceptID,
		&card.SourceDocumentID,
		&card.Front,
		&card.Back,
		&card.Due,
		&card.Stability,
		&card.Difficulty,
		&card.ElapsedDays,
		&card.ScheduledDays,
		&card.Reps,
		&card.Lapses,
		&card.State,
		&card.LastReview,
		&card.CreatedAt,
		&card.UpdatedAt,
		&card.WorkingQueuePosition,
	); err != nil {
		return nil, nil, fmt.Errorf("load card %d: %w", cardID, err)
	}

	now := time.Now().UTC()
	fsrsCard := card.ToFSRSCard()
	scheduling := s.params.Repeat(fsrsCard, now)
	info, ok := scheduling[rating]
	if !ok {
		return nil, nil, fmt.Errorf("rating %d not supported", rating)
	}
	card.ApplyFSRSCard(info.Card)
	card.UpdatedAt = now

	// Handle working queue logic
	if rating == fsrs.Again {
		// Add to working queue if rated "Again"
		if err := s.addToWorkingQueue(ctx, tx, cardID); err != nil {
			return nil, nil, fmt.Errorf("add to working queue: %w", err)
		}
	} else {
		// Remove from working queue if rated anything else
		if err := s.removeFromWorkingQueue(ctx, tx, cardID); err != nil {
			return nil, nil, fmt.Errorf("remove from working queue: %w", err)
		}
	}

	if _, err = tx.ExecContext(ctx, `
		UPDATE cards
		SET due = ?, stability = ?, difficulty = ?, elapsed_days = ?, scheduled_days = ?,
		    reps = ?, lapses = ?, state = ?, last_review = ?, updated_at = ?, working_queue_position = ?
		WHERE id = ?;
	`,
		nullTimePtr(card.Due),
		card.Stability,
		card.Difficulty,
		card.ElapsedDays,
		card.ScheduledDays,
		card.Reps,
		card.Lapses,
		card.State,
		nullTimePtr(card.LastReview),
		card.UpdatedAt,
		nullInt64Ptr(card.WorkingQueuePosition),
		card.ID,
	); err != nil {
		return nil, nil, fmt.Errorf("update card %d: %w", card.ID, err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO review_logs (card_id, rating, scheduled_days, elapsed_days, state, reviewed_at)
		VALUES (?, ?, ?, ?, ?, ?);
	`, card.ID, info.ReviewLog.Rating, info.ReviewLog.ScheduledDays, info.ReviewLog.ElapsedDays, info.ReviewLog.State, now); err != nil {
		return nil, nil, fmt.Errorf("insert review log: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit review: %w", err)
	}

	log := &models.ReviewLog{
		CardID:        card.ID,
		Rating:        int(info.ReviewLog.Rating),
		ScheduledDays: int(info.ReviewLog.ScheduledDays),
		ElapsedDays:   int(info.ReviewLog.ElapsedDays),
		State:         int(info.ReviewLog.State),
		ReviewedAt:    now,
	}

	return card, log, nil
}

// addToWorkingQueue adds a card to the working queue with a position within the working queue size limit
func (s *FlashcardService) addToWorkingQueue(ctx context.Context, tx *sql.Tx, cardID int64) error {
	const workingQueueSize = 20

	// Check if card is already in the working queue
	var existingPosition sql.NullInt64
	err := tx.QueryRowContext(ctx, "SELECT working_queue_position FROM cards WHERE id = ?", cardID).Scan(&existingPosition)
	if err != nil {
		return fmt.Errorf("check existing position: %w", err)
	}

	// If already in queue, don't add again
	if existingPosition.Valid {
		return nil
	}

	// Get the current max position in the working queue
	var maxPosition sql.NullInt64
	err = tx.QueryRowContext(ctx, "SELECT MAX(working_queue_position) FROM cards WHERE working_queue_position IS NOT NULL").Scan(&maxPosition)
	if err != nil {
		return fmt.Errorf("get max position: %w", err)
	}

	// Calculate new position
	newPosition := int64(1)
	if maxPosition.Valid {
		newPosition = maxPosition.Int64 + 1
	}

	// If we exceed the working queue size, remove the oldest card
	if newPosition > workingQueueSize {
		// Find and remove the card with the smallest position (oldest in queue)
		var oldestCardID int64
		err = tx.QueryRowContext(ctx, "SELECT id FROM cards WHERE working_queue_position IS NOT NULL ORDER BY working_queue_position ASC LIMIT 1").Scan(&oldestCardID)
		if err != nil {
			return fmt.Errorf("find oldest card: %w", err)
		}

		_, err = tx.ExecContext(ctx, "UPDATE cards SET working_queue_position = NULL WHERE id = ?", oldestCardID)
		if err != nil {
			return fmt.Errorf("remove oldest card: %w", err)
		}

		// Shift all remaining cards up by one position
		_, err = tx.ExecContext(ctx, "UPDATE cards SET working_queue_position = working_queue_position - 1 WHERE working_queue_position IS NOT NULL")
		if err != nil {
			return fmt.Errorf("shift positions: %w", err)
		}

		// Use the freed position
		newPosition = workingQueueSize
	}

	// Add the card to the working queue
	_, err = tx.ExecContext(ctx, "UPDATE cards SET working_queue_position = ? WHERE id = ?", newPosition, cardID)
	if err != nil {
		return fmt.Errorf("add card to queue: %w", err)
	}

	return nil
}

// removeFromWorkingQueue removes a card from the working queue and shifts other cards
func (s *FlashcardService) removeFromWorkingQueue(ctx context.Context, tx *sql.Tx, cardID int64) error {
	// Get the position of the card being removed
	var position sql.NullInt64
	err := tx.QueryRowContext(ctx, "SELECT working_queue_position FROM cards WHERE id = ?", cardID).Scan(&position)
	if err != nil {
		return fmt.Errorf("get card position: %w", err)
	}

	// If card is not in queue, nothing to do
	if !position.Valid {
		return nil
	}

	// Remove the card from the queue
	_, err = tx.ExecContext(ctx, "UPDATE cards SET working_queue_position = NULL WHERE id = ?", cardID)
	if err != nil {
		return fmt.Errorf("remove card from queue: %w", err)
	}

	// Shift all cards with higher positions down by one
	_, err = tx.ExecContext(ctx, "UPDATE cards SET working_queue_position = working_queue_position - 1 WHERE working_queue_position > ?", position.Int64)
	if err != nil {
		return fmt.Errorf("shift positions down: %w", err)
	}

	return nil
}

// BulkUpsertCards inserts or updates flashcards under a concept and source document.
func (s *FlashcardService) BulkUpsertCards(ctx context.Context, conceptID, documentID sql.NullInt64, cards []models.Card) error {
	if len(cards) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO cards (concept_id, source_document_id, front, back, due, stability, difficulty, elapsed_days,
		                   scheduled_days, reps, lapses, state, last_review, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`)
	if err != nil {
		return fmt.Errorf("prepare card insert: %w", err)
	}
	defer stmt.Close()

	for i := range cards {
		card := &cards[i]
		card.CreatedAt = now
		card.UpdatedAt = now
		if !card.Due.Valid {
			card.Due = sql.NullTime{Time: now, Valid: true}
		}
		if _, err = stmt.ExecContext(ctx,
			nullInt64Ptr(conceptID),
			nullInt64Ptr(documentID),
			card.Front,
			card.Back,
			nullTimePtr(card.Due),
			card.Stability,
			card.Difficulty,
			card.ElapsedDays,
			card.ScheduledDays,
			card.Reps,
			card.Lapses,
			card.State,
			nullTimePtr(card.LastReview),
			card.CreatedAt,
			card.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert card %q: %w", card.Front, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit bulk insert: %w", err)
	}
	return nil
}

// ListCardSummaries returns a stable slice of recent flashcards for prompt context.
func (s *FlashcardService) ListCardSummaries(ctx context.Context, limit int) ([]models.CardSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(co.name, ''), c.front, c.back
		FROM cards c
		LEFT JOIN concepts co ON c.concept_id = co.id
		ORDER BY c.created_at DESC
		LIMIT ?;
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list card summaries: %w", err)
	}
	defer rows.Close()

	var summaries []models.CardSummary
	for rows.Next() {
		var summary models.CardSummary
		if err := rows.Scan(&summary.ConceptName, &summary.Front, &summary.Back); err != nil {
			return nil, fmt.Errorf("scan card summary: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate card summaries: %w", err)
	}
	return summaries, nil
}

// ListAllFlashcards returns all flashcards with their details for the overview page
func (s *FlashcardService) ListAllFlashcards(ctx context.Context) ([]models.Card, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.concept_id, c.source_document_id, c.front, c.back,
			   c.due, c.stability, c.difficulty, c.elapsed_days, c.scheduled_days,
			   c.reps, c.lapses, c.state, c.last_review, c.created_at, c.updated_at,
			   c.working_queue_position, co.name, d.original_name
		FROM cards c
		LEFT JOIN concepts co ON c.concept_id = co.id
		LEFT JOIN documents d ON c.source_document_id = d.id
		ORDER BY c.created_at DESC;
	`)
	if err != nil {
		return nil, fmt.Errorf("list all flashcards: %w", err)
	}
	defer rows.Close()

	var cards []models.Card
	for rows.Next() {
		var card models.Card
		if err := rows.Scan(
			&card.ID,
			&card.ConceptID,
			&card.SourceDocumentID,
			&card.Front,
			&card.Back,
			&card.Due,
			&card.Stability,
			&card.Difficulty,
			&card.ElapsedDays,
			&card.ScheduledDays,
			&card.Reps,
			&card.Lapses,
			&card.State,
			&card.LastReview,
			&card.CreatedAt,
			&card.UpdatedAt,
			&card.WorkingQueuePosition,
			&card.ConceptName,
			&card.SourceDocumentRef,
		); err != nil {
			return nil, fmt.Errorf("scan flashcard: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate flashcards: %w", err)
	}
	return cards, nil
}

// GetFlashcardCount returns the total number of flashcards
func (s *FlashcardService) GetFlashcardCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cards;").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get flashcard count: %w", err)
	}
	return count, nil
}

// GetDueCardsCount returns the number of cards that are due for review
func (s *FlashcardService) GetDueCardsCount(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cards WHERE due IS NOT NULL AND due <= ?;",
		now).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("get due cards count: %w", err)
	}
	return count, nil
}

// GetDueCardsStats returns statistics about due cards
func (s *FlashcardService) GetDueCardsStats(ctx context.Context) (map[string]int, error) {
	now := time.Now().UTC()
	stats := make(map[string]int)
	
	// Get total cards
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM cards;").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("get total cards count: %w", err)
	}
	stats["total"] = total
	
	// Get due cards
	var due int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cards WHERE due IS NOT NULL AND due <= ?;",
		now).Scan(&due)
	if err != nil {
		return nil, fmt.Errorf("get due cards count: %w", err)
	}
	stats["due"] = due
	
	// Get new cards (never reviewed)
	var new int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cards WHERE state = 0;").Scan(&new)
	if err != nil {
		return nil, fmt.Errorf("get new cards count: %w", err)
	}
	stats["new"] = new
	
	// Get learning cards
	var learning int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cards WHERE state = 1;").Scan(&learning)
	if err != nil {
		return nil, fmt.Errorf("get learning cards count: %w", err)
	}
	stats["learning"] = learning
	
	// Get review cards (graduated)
	var review int
	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cards WHERE state = 2;").Scan(&review)
	if err != nil {
		return nil, fmt.Errorf("get review cards count: %w", err)
	}
	stats["review"] = review
	
	return stats, nil
}

func nullTimePtr(t sql.NullTime) any {
	if t.Valid {
		return t.Time
	}
	return nil
}

func nullInt64Ptr(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
