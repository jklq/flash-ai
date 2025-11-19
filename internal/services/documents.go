package services

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"flash-ai/internal/models"
)

type DocumentService struct {
	db        *sql.DB
	uploadDir string
}

func NewDocumentService(db *sql.DB, uploadDir string) *DocumentService {
	return &DocumentService{db: db, uploadDir: uploadDir}
}

func (s *DocumentService) Create(ctx context.Context, original string, docType models.DocumentType, src io.Reader) (*models.Document, error) {
	if docType != models.DocumentInformation && docType != models.DocumentExam {
		return nil, fmt.Errorf("unsupported doc type %s", docType)
	}

	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure upload dir: %w", err)
	}

	name := uuid.NewString() + filepath.Ext(original)
	storedPath := filepath.Join(s.uploadDir, name)
	out, err := os.Create(storedPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return nil, fmt.Errorf("write file: %w", err)
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO documents (original_name, stored_path, doc_type, page_count, uploaded_at)
		VALUES (?, ?, ?, 0, ?);
	`, original, storedPath, docType, now)
	if err != nil {
		return nil, fmt.Errorf("insert document: %w", err)
	}
	id, _ := res.LastInsertId()

	return &models.Document{
		ID:           id,
		OriginalName: original,
		StoredPath:   storedPath,
		Type:         docType,
		PageCount:    0,
		UploadedAt:   now,
	}, nil
}

func (s *DocumentService) UpdatePageCount(ctx context.Context, id int64, pages int) error {
	if _, err := s.db.ExecContext(ctx, `
		UPDATE documents SET page_count = ? WHERE id = ?;
	`, pages, id); err != nil {
		return fmt.Errorf("update page count: %w", err)
	}
	return nil
}

func (s *DocumentService) GetByID(ctx context.Context, id int64) (*models.Document, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, original_name, stored_path, doc_type, page_count, uploaded_at
		FROM documents WHERE id = ?;
	`, id)
	var doc models.Document
	if err := row.Scan(
		&doc.ID,
		&doc.OriginalName,
		&doc.StoredPath,
		&doc.Type,
		&doc.PageCount,
		&doc.UploadedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document %d not found", id)
		}
		return nil, fmt.Errorf("scan document: %w", err)
	}
	return &doc, nil
}
