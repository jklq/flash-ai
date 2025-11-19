package models

import (
	"database/sql"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs"
)

type DocumentType string

const (
	DocumentInformation DocumentType = "information"
	DocumentExam        DocumentType = "exam"
)

type Document struct {
	ID           int64
	OriginalName string
	StoredPath   string
	Type         DocumentType
	PageCount    int
	UploadedAt   time.Time
}

type Concept struct {
	ID            int64
	Name          string
	Description   sql.NullString
	Weight        float64
	SourceExamIDs string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ConceptCluster represents a condensed/merged concept
type ConceptCluster struct {
	ID          int64
	Name        string
	Description sql.NullString
	Weight      float64
	IsCondensed bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ConceptClusterMember represents a concept within a cluster
type ConceptClusterMember struct {
	ClusterID       int64
	ConceptID       int64
	SimilarityScore float64
	IsPrimary       bool
	ConceptName     string
	Description     sql.NullString
	Weight          float64
}

// ConceptMerge represents a merge operation
type ConceptMerge struct {
	ID               int64
	SourceConceptID  int64
	TargetClusterID  int64
	MergeReason      string
	CreatedAt        time.Time
}

// CondensedConcept is a cluster with its member concepts and flashcard counts
type CondensedConcept struct {
	Cluster        ConceptCluster
	Members        []ConceptClusterMember
	FlashcardCount int
	TotalWeight    float64
}

type Card struct {
	ID                int64
	ConceptID         sql.NullInt64
	SourceDocumentID  sql.NullInt64
	Front             string
	Back              string
	Due               sql.NullTime
	Stability         float64
	Difficulty        float64
	ElapsedDays       int
	ScheduledDays     int
	Reps              int
	Lapses            int
	State             int
	LastReview        sql.NullTime
	CreatedAt         time.Time
	UpdatedAt         time.Time
	WorkingQueuePosition sql.NullInt64  // Position in working queue for "Again" cards
	ConceptName       sql.NullString
	SourceDocumentRef sql.NullString
}

// CardSummary captures the minimal flashcard fields needed for prompt context.
type CardSummary struct {
	ConceptName string
	Front       string
	Back        string
}

type ReviewLog struct {
	ID            int64
	CardID        int64
	Rating        int
	ScheduledDays int
	ElapsedDays   int
	State         int
	ReviewedAt    time.Time
}

type DocumentTopic struct {
	DocumentID int64
	Topic      string
	Frequency  int
}

func (c *Card) ToFSRSCard() fsrs.Card {
	card := fsrs.Card{
		Stability:     c.Stability,
		Difficulty:    c.Difficulty,
		ElapsedDays:   uint64(max(c.ElapsedDays, 0)),
		ScheduledDays: uint64(max(c.ScheduledDays, 0)),
		Reps:          uint64(max(c.Reps, 0)),
		Lapses:        uint64(max(c.Lapses, 0)),
		State:         fsrs.State(max(c.State, 0)),
	}
	if c.Due.Valid {
		card.Due = c.Due.Time
	}
	if c.LastReview.Valid {
		card.LastReview = c.LastReview.Time
	}
	return card
}

func (c *Card) ApplyFSRSCard(f fsrs.Card) {
	c.Due = sql.NullTime{Time: f.Due, Valid: !f.Due.IsZero()}
	c.Stability = f.Stability
	c.Difficulty = f.Difficulty
	c.ElapsedDays = int(f.ElapsedDays)
	c.ScheduledDays = int(f.ScheduledDays)
	c.Reps = int(f.Reps)
	c.Lapses = int(f.Lapses)
	c.State = int(f.State)
	c.LastReview = sql.NullTime{Time: f.LastReview, Valid: !f.LastReview.IsZero()}
}

func max[T ~int | ~int32 | ~int64](a, b T) T {
	if a > b {
		return a
	}
	return b
}
