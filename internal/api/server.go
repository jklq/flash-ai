package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	fsrs "github.com/open-spaced-repetition/go-fsrs"

	"flash-ai/internal/models"
	"flash-ai/internal/services"
)

const maxMultipartMemory = 8 << 20 // 8 MB

type Server struct {
	mux        *http.ServeMux
	flashcards *services.FlashcardService
	concepts   *services.ConceptService
	documents  *services.DocumentService
	ingestion  *services.IngestionService
	jobs       *JobManager
}

type DocumentResult struct {
	DocumentID int64       `json:"documentId"`
	Name       string      `json:"name"`
	Pages      int         `json:"pages"`
	Status     string      `json:"status"`
	Message    string      `json:"message,omitempty"`
	Payload    interface{} `json:"payload,omitempty"`
}

func NewServer(
	flashcards *services.FlashcardService,
	concepts *services.ConceptService,
	documents *services.DocumentService,
	ingestion *services.IngestionService,
) *Server {
	s := &Server{
		mux:        http.NewServeMux(),
		flashcards: flashcards,
		concepts:   concepts,
		documents:  documents,
		ingestion:  ingestion,
		jobs:       NewJobManager(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealth)
	s.mux.HandleFunc("/api/cards/next", s.handleGetNextCard)
	s.mux.HandleFunc("/api/cards/all", s.handleGetAllFlashcards)
	s.mux.HandleFunc("/api/cards/stats", s.handleGetCardsStats)
	s.mux.HandleFunc("/api/cards/", s.handleCardActions)
	s.mux.HandleFunc("/api/topics", s.handleListTopics)
	s.mux.HandleFunc("/api/topics/condensed", s.handleListCondensedTopics)
	s.mux.HandleFunc("/api/topics/condense", s.handleCondenseTopics)
	s.mux.HandleFunc("/api/topics/", s.handleTopicActions)
	s.mux.HandleFunc("/api/documents", s.handleUploadDocuments)
	s.mux.HandleFunc("/api/documents/jobs", s.handleJobs)
	s.mux.HandleFunc("/api/documents/jobs/", s.handleJobStatus)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleGetNextCard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	card, err := s.flashcards.NextCard(r.Context())
	if err != nil {
		if err == services.ErrNoDueCards {
			writeJSON(w, http.StatusOK, map[string]any{
				"card":    nil,
				"message": "No cards due. Come back later!",
			})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"card": map[string]any{
			"id":        card.ID,
			"front":     card.Front,
			"back":      card.Back,
			"due":       nullTimeToString(card.Due),
			"concept":   nullString(card.ConceptName),
			"source":    nullString(card.SourceDocumentRef),
			"state":     card.State,
			"stability": card.Stability,
		},
	})
}

func (s *Server) handleGetAllFlashcards(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	cards, err := s.flashcards.ListAllFlashcards(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	count, err := s.flashcards.GetFlashcardCount(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]map[string]any, 0, len(cards))
	for _, card := range cards {
		out = append(out, map[string]any{
			"id":        card.ID,
			"front":     card.Front,
			"back":      card.Back,
			"due":       nullTimeToString(card.Due),
			"concept":   nullString(card.ConceptName),
			"source":    nullString(card.SourceDocumentRef),
			"state":     card.State,
			"stability": card.Stability,
			"created_at": card.CreatedAt.Format(timeLayout),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"flashcards": out,
		"total":      count,
	})
}

func (s *Server) handleGetCardsStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	stats, err := s.flashcards.GetDueCardsStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"stats": stats,
	})
}

func (s *Server) handleCardActions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/cards/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "review" {
		http.NotFound(w, r)
		return
	}

	cardID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	var payload reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	rating, err := parseRating(payload.Rating)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	card, logEntry, err := s.flashcards.ReviewCard(r.Context(), cardID, rating)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"card": map[string]any{
			"id":    card.ID,
			"due":   nullTimeToString(card.Due),
			"state": card.State,
		},
		"log": map[string]any{
			"rating": logEntry.Rating,
			"due_in": logEntry.ScheduledDays,
			"updated": logEntry.ReviewedAt.Format(
				timeLayout,
			),
		},
	})
}

type reviewRequest struct {
	Rating string `json:"rating"`
}

func (s *Server) handleListTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	concepts, err := s.concepts.ListConcepts(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]map[string]any, 0, len(concepts))
	for _, concept := range concepts {
		out = append(out, map[string]any{
			"id":          concept.ID,
			"name":        concept.Name,
			"description": nullSQLString(concept.Description),
			"weight":      concept.Weight,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"topics": out})
}

func (s *Server) handleListCondensedTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	condensed, err := s.concepts.ListCondensedConcepts(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := make([]map[string]any, 0, len(condensed))
	for _, concept := range condensed {
		members := make([]map[string]any, 0, len(concept.Members))
		for _, member := range concept.Members {
			members = append(members, map[string]any{
				"id":               member.ConceptID,
				"name":             member.ConceptName,
				"description":      nullString(member.Description),
				"weight":           member.Weight,
				"similarity_score": member.SimilarityScore,
				"is_primary":       member.IsPrimary,
			})
		}

		out = append(out, map[string]any{
			"cluster": map[string]any{
				"id":          concept.Cluster.ID,
				"name":        concept.Cluster.Name,
				"description": nullSQLString(concept.Cluster.Description),
				"weight":      concept.Cluster.Weight,
			},
			"members":         members,
			"flashcard_count": concept.FlashcardCount,
			"total_weight":    concept.TotalWeight,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"condensed_topics": out})
}

func (s *Server) handleCondenseTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var payload struct {
		Threshold float64 `json:"threshold"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}

	// Default threshold if not provided
	threshold := payload.Threshold
	if threshold == 0 {
		threshold = 0.5
	}

	if err := s.concepts.CondenseConcepts(r.Context(), threshold); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"message": fmt.Sprintf("Concepts condensed with threshold %.2f", threshold),
	})
}

func (s *Server) handleTopicActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/topics/")
	path = strings.Trim(path, "/")

	if strings.HasPrefix(path, "flashcards/") {
		// Get flashcards for a concept
		conceptIDStr := strings.TrimPrefix(path, "flashcards/")
		conceptIDStr = strings.Trim(conceptIDStr, "/")

		conceptID, err := strconv.ParseInt(conceptIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid concept id")
			return
		}

		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		cards, err := s.concepts.GetFlashcardsForConcept(r.Context(), conceptID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		out := make([]map[string]any, 0, len(cards))
		for _, card := range cards {
			out = append(out, map[string]any{
				"front":        card.Front,
				"back":         card.Back,
				"concept_name": card.ConceptName,
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{"flashcards": out})
		return
	}

	if strings.HasPrefix(path, "analysis") {
		// Get concept overlap analysis (handles both /api/topics/analysis and /api/topics/analysis/)
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		analysis, err := s.concepts.GetConceptOverlapAnalysis(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, analysis)
		return
	}

	// Topic not found
	http.NotFound(w, r)
}

func (s *Server) handleUploadDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	if form := r.MultipartForm; form != nil {
		defer form.RemoveAll()
	}

	docType := models.DocumentType(r.FormValue("docType"))
	if docType != models.DocumentInformation && docType != models.DocumentExam {
		writeError(w, http.StatusBadRequest, "docType must be 'information' or 'exam'")
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files uploaded")
		return
	}

	results := make([]DocumentResult, 0, len(files))
	for _, file := range files {
		result, err := s.processDocument(r.Context(), file, docType, nil)
		if err != nil {
			result.Status = FileStatusError
		}
		results = append(results, result)
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/documents/jobs" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	s.handleCreateUploadJob(w, r)
}

func (s *Server) handleCreateUploadJob(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	docType := models.DocumentType(r.FormValue("docType"))
	if docType != models.DocumentInformation && docType != models.DocumentExam {
		writeError(w, http.StatusBadRequest, "docType must be 'information' or 'exam'")
		return
	}

	form := r.MultipartForm
	if form == nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "no files uploaded")
		return
	}

	fileNames := make([]string, len(files))
	for i, file := range files {
		fileNames[i] = file.Filename
	}

	fileHeaders := append([]*multipart.FileHeader(nil), files...)
	jobID, snapshot := s.jobs.CreateJob(fileNames)

	go s.runUploadJob(context.Background(), jobID, docType, fileHeaders, form)

	writeJSON(w, http.StatusAccepted, snapshot)
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/api/documents/jobs/") {
		http.NotFound(w, r)
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/api/documents/jobs/")
	jobID = strings.Trim(jobID, "/")
	if jobID == "" {
		http.NotFound(w, r)
		return
	}

	job, ok := s.jobs.GetJob(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) runUploadJob(ctx context.Context, jobID string, docType models.DocumentType, files []*multipart.FileHeader, form *multipart.Form) {
	defer func() {
		if form != nil {
			_ = form.RemoveAll()
		}
	}()

	if ctx == nil {
		ctx = context.Background()
	}

	s.jobs.MarkProcessing(jobID)
	for idx, file := range files {
		s.jobs.MarkFileStarted(jobID, idx)
		progress := func(step, message string, current, total int) {
			s.jobs.UpdateFileProgress(jobID, idx, step, message, current, total)
		}
		result, err := s.processDocument(ctx, file, docType, progress)
		if err != nil {
			s.jobs.MarkFileError(jobID, idx, err.Error(), result)
			continue
		}
		s.jobs.MarkFileComplete(jobID, idx, result)
	}
	s.jobs.MarkCompleted(jobID)
}

func (s *Server) processDocument(ctx context.Context, file *multipart.FileHeader, docType models.DocumentType, progress services.ProgressCallback) (DocumentResult, error) {
	result := DocumentResult{
		Name:   file.Filename,
		Status: FileStatusError,
	}

	src, err := file.Open()
	if err != nil {
		result.Message = err.Error()
		return result, fmt.Errorf("open file %s: %w", file.Filename, err)
	}
	defer src.Close()

	doc, err := s.documents.Create(ctx, file.Filename, docType, src)
	if err != nil {
		result.Message = err.Error()
		return result, fmt.Errorf("create document %s: %w", file.Filename, err)
	}

	result.DocumentID = doc.ID
	result.Pages = doc.PageCount

	var payload interface{}
	switch docType {
	case models.DocumentExam:
		if progress != nil {
			payload, err = s.ingestion.ProcessExamDocumentWithProgress(ctx, doc, progress)
		} else {
			payload, err = s.ingestion.ProcessExamDocument(ctx, doc)
		}
	case models.DocumentInformation:
		if progress != nil {
			payload, err = s.ingestion.ProcessInformationDocumentWithProgress(ctx, doc, progress)
		} else {
			payload, err = s.ingestion.ProcessInformationDocument(ctx, doc)
		}
	default:
		err = fmt.Errorf("unsupported document type: %s", docType)
	}

	if err != nil {
		result.Message = err.Error()
		result.Payload = payload
		return result, err
	}

	result.Status = "ok"
	result.Payload = payload
	return result, nil
}

const timeLayout = time.RFC3339

func parseRating(raw string) (fsrs.Rating, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "again":
		return fsrs.Again, nil
	case "hard":
		return fsrs.Hard, nil
	case "good":
		return fsrs.Good, nil
	case "easy":
		return fsrs.Easy, nil
	default:
		return 0, fmt.Errorf("unknown rating %q", raw)
	}
}

func nullTimeToString(t sql.NullTime) *string {
	if t.Valid {
		str := t.Time.Format(timeLayout)
		return &str
	}
	return nil
}

func nullString(v sql.NullString) *string {
	if v.Valid {
		str := v.String
		return &str
	}
	return nil
}

func nullSQLString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
