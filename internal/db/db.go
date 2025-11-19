package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Open connects to the SQLite database and runs schema migrations.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_foreign_keys=1", path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	conn.SetMaxOpenConns(1)
	conn.SetConnMaxLifetime(0)

	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return conn, nil
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			original_name TEXT NOT NULL,
			stored_path TEXT NOT NULL UNIQUE,
			doc_type TEXT NOT NULL CHECK(doc_type IN ('information','exam')),
			page_count INTEGER NOT NULL DEFAULT 0,
			uploaded_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS concepts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			weight REAL NOT NULL DEFAULT 0,
			source_exam_ids TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS cards (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			concept_id INTEGER,
			source_document_id INTEGER,
			front TEXT NOT NULL,
			back TEXT NOT NULL,
			due DATETIME,
			stability REAL NOT NULL DEFAULT 0,
			difficulty REAL NOT NULL DEFAULT 0,
			elapsed_days INTEGER NOT NULL DEFAULT 0,
			scheduled_days INTEGER NOT NULL DEFAULT 0,
			reps INTEGER NOT NULL DEFAULT 0,
			lapses INTEGER NOT NULL DEFAULT 0,
			state INTEGER NOT NULL DEFAULT 0,
			last_review DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY(concept_id) REFERENCES concepts(id) ON DELETE SET NULL,
			FOREIGN KEY(source_document_id) REFERENCES documents(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS review_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			card_id INTEGER NOT NULL,
			rating INTEGER NOT NULL,
			scheduled_days INTEGER NOT NULL,
			elapsed_days INTEGER NOT NULL,
			state INTEGER NOT NULL,
			reviewed_at DATETIME NOT NULL,
			FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS document_topics (
			document_id INTEGER NOT NULL,
			topic TEXT NOT NULL,
			frequency INTEGER NOT NULL,
			PRIMARY KEY(document_id, topic),
			FOREIGN KEY(document_id) REFERENCES documents(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS concept_clusters (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			weight REAL NOT NULL DEFAULT 0,
			is_condensed INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(name)
		);`,
		`CREATE TABLE IF NOT EXISTS concept_cluster_members (
			cluster_id INTEGER NOT NULL,
			concept_id INTEGER NOT NULL,
			similarity_score REAL NOT NULL DEFAULT 0,
			is_primary INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (cluster_id, concept_id),
			FOREIGN KEY(cluster_id) REFERENCES concept_clusters(id) ON DELETE CASCADE,
			FOREIGN KEY(concept_id) REFERENCES concepts(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS concept_merges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_concept_id INTEGER NOT NULL,
			target_cluster_id INTEGER NOT NULL,
			merge_reason TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY(source_concept_id) REFERENCES concepts(id) ON DELETE CASCADE,
			FOREIGN KEY(target_cluster_id) REFERENCES concept_clusters(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_cards_due ON cards(due);`,
		`CREATE INDEX IF NOT EXISTS idx_cards_concept ON cards(concept_id);`,
		`CREATE INDEX IF NOT EXISTS idx_clusters_weight ON concept_clusters(weight DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_members_similarity ON concept_cluster_members(similarity_score DESC);`,
		// Add working_queue_position column to cards table for "Again" cards
		`ALTER TABLE cards ADD COLUMN working_queue_position INTEGER DEFAULT NULL;`,
		`CREATE INDEX IF NOT EXISTS idx_cards_working_queue ON cards(working_queue_position) WHERE working_queue_position IS NOT NULL;`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("execute %q: %w", stmt, err)
		}
	}

	// Ensure at least one concept row for uncategorized content for easier joins later.
	const insertDefault = `
	INSERT INTO concepts (name, description, weight, source_exam_ids, created_at, updated_at)
	SELECT ?, ?, 0, '', ?, ?
	WHERE NOT EXISTS (SELECT 1 FROM concepts WHERE name = ?);`
	now := time.Now().UTC()
	if _, err := db.Exec(insertDefault, "General", "General knowledge", now, now, "General"); err != nil {
		return fmt.Errorf("seed default concept: %w", err)
	}

	return nil
}
