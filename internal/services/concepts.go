package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"flash-ai/internal/models"
)

// ConceptService manages topic weighting sourced from exam documents.
type ConceptService struct {
	db *sql.DB
}

func NewConceptService(db *sql.DB) *ConceptService {
	return &ConceptService{db: db}
}

func (s *ConceptService) ListConcepts(ctx context.Context, limit int) ([]models.Concept, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, weight, source_exam_ids, created_at, updated_at
		FROM concepts
		ORDER BY weight DESC, name ASC
		LIMIT ?;
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query concepts: %w", err)
	}
	defer rows.Close()

	var out []models.Concept
	for rows.Next() {
		var concept models.Concept
		if err := rows.Scan(
			&concept.ID,
			&concept.Name,
			&concept.Description,
			&concept.Weight,
			&concept.SourceExamIDs,
			&concept.CreatedAt,
			&concept.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan concept: %w", err)
		}
		out = append(out, concept)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate concepts: %w", err)
	}
	return out, nil
}

func (s *ConceptService) UpsertExamTopic(ctx context.Context, topic models.DocumentTopic, description string) (*models.Concept, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	var concept models.Concept
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, description, weight, source_exam_ids, created_at, updated_at
		FROM concepts
		WHERE name = ?;
	`, topic.Topic).Scan(
		&concept.ID,
		&concept.Name,
		&concept.Description,
		&concept.Weight,
		&concept.SourceExamIDs,
		&concept.CreatedAt,
		&concept.UpdatedAt,
	)

	var ids []int64
	if err == sql.ErrNoRows {
		ids = []int64{topic.DocumentID}
		raw, _ := json.Marshal(ids)
		descriptionVal := sql.NullString{Valid: description != "", String: description}
		res, execErr := tx.ExecContext(ctx, `
			INSERT INTO concepts (name, description, weight, source_exam_ids, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?);
		`, topic.Topic, descriptionVal, topic.Frequency, string(raw), now, now)
		if execErr != nil {
			return nil, fmt.Errorf("insert concept %s: %w", topic.Topic, execErr)
		}
		id, _ := res.LastInsertId()
		concept = models.Concept{
			ID:            id,
			Name:          topic.Topic,
			Description:   descriptionVal,
			Weight:        float64(topic.Frequency),
			SourceExamIDs: string(raw),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		err = nil
	} else if err != nil {
		return nil, fmt.Errorf("select concept %s: %w", topic.Topic, err)
	} else {
		if concept.SourceExamIDs != "" {
			if parseErr := json.Unmarshal([]byte(concept.SourceExamIDs), &ids); parseErr != nil {
				ids = nil
			}
		}
		if !slices.Contains(ids, topic.DocumentID) {
			ids = append(ids, topic.DocumentID)
		}
		raw, _ := json.Marshal(ids)

		weight := concept.Weight + float64(topic.Frequency)
		descriptionVal := concept.Description
		if description != "" {
			descriptionVal = sql.NullString{Valid: true, String: description}
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE concepts
			SET weight = ?, description = ?, source_exam_ids = ?, updated_at = ?
			WHERE id = ?;
		`, weight, descriptionVal, string(raw), now, concept.ID); err != nil {
			return nil, fmt.Errorf("update concept %s: %w", topic.Topic, err)
		}

		concept.Weight = weight
		concept.Description = descriptionVal
		concept.SourceExamIDs = string(raw)
		concept.UpdatedAt = now
	}

	if _, errExec := tx.ExecContext(ctx, `
		INSERT INTO document_topics (document_id, topic, frequency)
		VALUES (?, ?, ?)
		ON CONFLICT(document_id, topic) DO UPDATE SET frequency = excluded.frequency;
	`, topic.DocumentID, topic.Topic, topic.Frequency); errExec != nil {
		return nil, fmt.Errorf("upsert document topic: %w", errExec)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit concept upsert: %w", err)
	}
	return &concept, nil
}

// ListCondensedConcepts returns merged concepts with flashcard counts and member visibility
func (s *ConceptService) ListCondensedConcepts(ctx context.Context, limit int) ([]models.CondensedConcept, error) {
	if limit <= 0 {
		limit = 50
	}
	
	clusters, err := s.listConceptClusters(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list clusters: %w", err)
	}
	
	// If no clusters exist, return empty result
	if len(clusters) == 0 {
		return []models.CondensedConcept{}, nil
	}
	
	condensed := make([]models.CondensedConcept, 0, len(clusters))
	for _, cluster := range clusters {
		members, err := s.getClusterMembers(ctx, cluster.ID)
		if err != nil {
			continue // Skip clusters with errors
		}
		
		flashcardCount, err := s.getFlashcardCountForCluster(ctx, cluster.ID)
		if err != nil {
			flashcardCount = 0
		}
		
		totalWeight := cluster.Weight
		for _, member := range members {
			totalWeight += member.Weight
		}
		
		condensed = append(condensed, models.CondensedConcept{
			Cluster:        cluster,
			Members:        members,
			FlashcardCount: flashcardCount,
			TotalWeight:    totalWeight,
		})
	}
	
	return condensed, nil
}

// CondenseConcepts performs automatic concept clustering and merging
func (s *ConceptService) CondenseConcepts(ctx context.Context, similarityThreshold float64) error {
	concepts, err := s.ListConcepts(ctx, -1) // Get all concepts
	if err != nil {
		return fmt.Errorf("get all concepts: %w", err)
	}
	
	// Clear existing clusters to start fresh
	if err := s.clearExistingClusters(ctx); err != nil {
		return fmt.Errorf("clear existing clusters: %w", err)
	}
	
	// Group concepts by similarity
	clusters := s.clusterConcepts(concepts, similarityThreshold)
	
	// Create clusters and merge concepts
	for _, cluster := range clusters {
		if len(cluster.concepts) <= 1 {
			continue // No merging needed
		}
		
		if err := s.createAndMergeCluster(ctx, cluster); err != nil {
			return fmt.Errorf("create cluster: %w", err)
		}
	}
	
	return nil
}

// GetFlashcardsForConcept returns all flashcards associated with a concept cluster
func (s *ConceptService) GetFlashcardsForConcept(ctx context.Context, conceptID int64) ([]models.CardSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT c.front, c.back, c.concept_id, cc.name as concept_name
		FROM cards c
		LEFT JOIN concept_cluster_members m ON c.concept_id = m.concept_id
		LEFT JOIN concept_clusters cc ON m.cluster_id = cc.id
		WHERE c.concept_id = ? OR (m.cluster_id = ? AND cc.is_condensed = 1)
		ORDER BY cc.weight DESC, c.created_at ASC;
	`, conceptID, conceptID)
	if err != nil {
		return nil, fmt.Errorf("query concept flashcards: %w", err)
	}
	defer rows.Close()
	
	var cards []models.CardSummary
	for rows.Next() {
		var card models.CardSummary
		var conceptName sql.NullString
		if err := rows.Scan(&card.Front, &card.Back, &conceptID, &conceptName); err != nil {
			return nil, fmt.Errorf("scan card: %w", err)
		}
		card.ConceptName = nullString(conceptName)
		cards = append(cards, card)
	}
	
	return cards, nil
}

// GetConceptOverlapAnalysis shows overlapping concepts and their similarity scores
func (s *ConceptService) GetConceptOverlapAnalysis(ctx context.Context) (map[string]interface{}, error) {
	concepts, err := s.ListConcepts(ctx, -1)
	if err != nil {
		return nil, fmt.Errorf("get concepts: %w", err)
	}
	
	overlaps := make([]map[string]interface{}, 0)
	for i, concept1 := range concepts {
		for _, concept2 := range concepts[i+1:] {
			similarity := s.calculateConceptSimilarity(concept1, concept2)
			if similarity > 0.3 { // Threshold for significant overlap
				overlaps = append(overlaps, map[string]interface{}{
					"concept1":       concept1.Name,
					"concept2":       concept2.Name,
					"similarity":     similarity,
					"concept1_weight": concept1.Weight,
					"concept2_weight": concept2.Weight,
					"merge_recommendation": similarity > 0.7,
				})
			}
		}
	}
	
	return map[string]interface{}{
		"total_concepts": len(concepts),
		"potential_overlaps": overlaps,
		"analysis_date": time.Now(),
	}, nil
}

// Helper methods
func (s *ConceptService) clearExistingClusters(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM concept_merges;
		DELETE FROM concept_cluster_members;
		DELETE FROM concept_clusters;
	`)
	return err
}

func (s *ConceptService) listConceptClusters(ctx context.Context, limit int) ([]models.ConceptCluster, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, weight, is_condensed, created_at, updated_at
		FROM concept_clusters
		ORDER BY weight DESC, name ASC
		LIMIT ?;
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query clusters: %w", err)
	}
	defer rows.Close()
	
	var clusters []models.ConceptCluster
	for rows.Next() {
		var cluster models.ConceptCluster
		if err := rows.Scan(
			&cluster.ID, &cluster.Name, &cluster.Description,
			&cluster.Weight, &cluster.IsCondensed,
			&cluster.CreatedAt, &cluster.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		clusters = append(clusters, cluster)
	}
	
	return clusters, nil
}

func (s *ConceptService) getClusterMembers(ctx context.Context, clusterID int64) ([]models.ConceptClusterMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.cluster_id, m.concept_id, m.similarity_score, m.is_primary,
			   c.name, c.description, c.weight
		FROM concept_cluster_members m
		JOIN concepts c ON m.concept_id = c.id
		WHERE m.cluster_id = ?
		ORDER BY m.similarity_score DESC, m.is_primary DESC;
	`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("query cluster members: %w", err)
	}
	defer rows.Close()
	
	var members []models.ConceptClusterMember
	for rows.Next() {
		var member models.ConceptClusterMember
		if err := rows.Scan(
			&member.ClusterID, &member.ConceptID, &member.SimilarityScore, &member.IsPrimary,
			&member.ConceptName, &member.Description, &member.Weight,
		); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, member)
	}
	
	return members, nil
}

func (s *ConceptService) getFlashcardCountForCluster(ctx context.Context, clusterID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT c.id)
		FROM cards c
		JOIN concept_cluster_members m ON c.concept_id = m.concept_id
		WHERE m.cluster_id = ? AND c.concept_id IS NOT NULL;
	`, clusterID).Scan(&count)
	
	if err != nil {
		return 0, fmt.Errorf("count cluster flashcards: %w", err)
	}
	
	return count, nil
}

type conceptCluster struct {
	concepts    []models.Concept
	avgWeight   float64
	primaryName string
}

func (s *ConceptService) clusterConcepts(concepts []models.Concept, threshold float64) []conceptCluster {
	if len(concepts) == 0 {
		return []conceptCluster{}
	}
	
	// Sort concepts by weight (descending) to prioritize important concepts
	sortedConcepts := make([]models.Concept, len(concepts))
	copy(sortedConcepts, concepts)
	for i := 0; i < len(sortedConcepts)-1; i++ {
		for j := i + 1; j < len(sortedConcepts); j++ {
			if sortedConcepts[j].Weight > sortedConcepts[i].Weight {
				sortedConcepts[i], sortedConcepts[j] = sortedConcepts[j], sortedConcepts[i]
			}
		}
	}
	
	var clusters []conceptCluster
	processed := make(map[int64]bool)
	
	for _, concept := range sortedConcepts {
		if processed[concept.ID] {
			continue
		}
		
		// Start a new cluster with this concept as potential primary
		newCluster := conceptCluster{
			concepts:    []models.Concept{concept},
			avgWeight:   concept.Weight,
			primaryName: concept.Name,
		}
		processed[concept.ID] = true
		
		// Find all concepts that should be in this cluster
		for _, otherConcept := range concepts {
			if processed[otherConcept.ID] {
				continue
			}
			
			if s.shouldMergeConcept(&newCluster, otherConcept, threshold) {
				newCluster.concepts = append(newCluster.concepts, otherConcept)
				processed[otherConcept.ID] = true
			}
		}
		
		// Update cluster properties
		newCluster.avgWeight = s.calculateAverageWeight(newCluster.concepts)
		newCluster.primaryName = s.selectPrimaryName(newCluster.concepts)
		
		clusters = append(clusters, newCluster)
	}
	
	return clusters
}

func (s *ConceptService) shouldMergeConcept(cluster *conceptCluster, concept models.Concept, threshold float64) bool {
	for _, clusterConcept := range cluster.concepts {
		similarity := s.calculateConceptSimilarity(clusterConcept, concept)
		if similarity >= threshold {
			return true
		}
	}
	return false
}

func (s *ConceptService) calculateConceptSimilarity(c1, c2 models.Concept) float64 {
	// Enhanced similarity calculation based on multiple factors
	nameSim := s.calculateNameSimilarity(c1.Name, c2.Name)
	
	// Description similarity (if available)
	descSim := 0.0
	if c1.Description.Valid && c2.Description.Valid {
		descSim = s.calculateNameSimilarity(c1.Description.String, c2.Description.String)
	}
	
	// Weight similarity - how close are the importance scores
	maxWeight := max(c1.Weight, c2.Weight)
	if maxWeight == 0 {
		maxWeight = 1.0 // Avoid division by zero
	}
	weightSim := 1.0 - min(abs(c1.Weight-c2.Weight)/maxWeight, 1.0)
	
	// Combined similarity with weighted factors
	// Name is most important, then description, then weight
	return 0.6*nameSim + 0.25*descSim + 0.15*weightSim
}

func (s *ConceptService) calculateNameSimilarity(name1, name2 string) float64 {
	// Simple token-based similarity
	tokens1 := strings.Fields(strings.ToLower(name1))
	tokens2 := strings.Fields(strings.ToLower(name2))
	
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}
	
	common := 0
	tokenSet1 := make(map[string]bool)
	for _, token := range tokens1 {
		tokenSet1[token] = true
	}
	
	for _, token := range tokens2 {
		if tokenSet1[token] {
			common++
		}
	}
	
	total := len(tokens1) + len(tokens2)
	if total == 0 {
		return 0.0
	}
	
	return (2.0 * float64(common)) / float64(total)
}

func (s *ConceptService) calculateAverageWeight(concepts []models.Concept) float64 {
	if len(concepts) == 0 {
		return 0.0
	}
	
	total := 0.0
	for _, concept := range concepts {
		total += concept.Weight
	}
	return total / float64(len(concepts))
}

func (s *ConceptService) selectPrimaryName(concepts []models.Concept) string {
	// Select the concept with highest weight as primary
	if len(concepts) == 0 {
		return ""
	}
	
	primary := concepts[0]
	for _, concept := range concepts[1:] {
		if concept.Weight > primary.Weight {
			primary = concept
		}
	}
	
	return primary.Name
}

func (s *ConceptService) createAndMergeCluster(ctx context.Context, cluster conceptCluster) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	
	now := time.Now()
	
	// Create cluster
	res, err := tx.ExecContext(ctx, `
		INSERT INTO concept_clusters (name, description, weight, is_condensed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?);
	`, cluster.primaryName, sql.NullString{}, cluster.avgWeight, true, now, now)
	if err != nil {
		return fmt.Errorf("insert cluster: %w", err)
	}
	
	clusterID, _ := res.LastInsertId()
	
	// Add members
	for i, concept := range cluster.concepts {
		similarity := 0.5 // Default similarity
		if i > 0 {
			// Calculate similarity with primary concept
			similarity = s.calculateConceptSimilarity(cluster.concepts[0], concept)
		}
		
		_, err := tx.ExecContext(ctx, `
			INSERT INTO concept_cluster_members (cluster_id, concept_id, similarity_score, is_primary)
			VALUES (?, ?, ?, ?);
		`, clusterID, concept.ID, similarity, i == 0)
		if err != nil {
			return fmt.Errorf("insert cluster member: %w", err)
		}
		
		// Record merge
		_, err = tx.ExecContext(ctx, `
			INSERT INTO concept_merges (source_concept_id, target_cluster_id, merge_reason, created_at)
			VALUES (?, ?, ?, ?);
		`, concept.ID, clusterID, "automatic_similarity_clustering", now)
		if err != nil {
			return fmt.Errorf("insert merge record: %w", err)
		}
	}
	
	return tx.Commit()
}

// Helper functions
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}

func (s *ConceptService) TouchConcept(ctx context.Context, name, description string) (*models.Concept, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	now := time.Now().UTC()
	var concept models.Concept
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, description, weight, source_exam_ids, created_at, updated_at
		FROM concepts WHERE name = ?;
	`, name).Scan(
		&concept.ID,
		&concept.Name,
		&concept.Description,
		&concept.Weight,
		&concept.SourceExamIDs,
		&concept.CreatedAt,
		&concept.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		descriptionVal := sql.NullString{Valid: description != "", String: description}
		res, execErr := tx.ExecContext(ctx, `
			INSERT INTO concepts (name, description, weight, source_exam_ids, created_at, updated_at)
			VALUES (?, ?, 0, '[]', ?, ?);
		`, name, descriptionVal, now, now)
		if execErr != nil {
			return nil, fmt.Errorf("insert concept %s: %w", name, execErr)
		}
		id, _ := res.LastInsertId()
		concept = models.Concept{
			ID:            id,
			Name:          name,
			Description:   descriptionVal,
			Weight:        0,
			SourceExamIDs: "[]",
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		err = nil
	} else if err != nil {
		return nil, fmt.Errorf("select concept %s: %w", name, err)
	}

	if description != "" && (!concept.Description.Valid || concept.Description.String != description) {
		if _, err = tx.ExecContext(ctx, `
			UPDATE concepts SET description = ?, updated_at = ? WHERE id = ?;
		`, description, now, concept.ID); err != nil {
			return nil, fmt.Errorf("update concept description: %w", err)
		}
		concept.Description = sql.NullString{Valid: true, String: description}
		concept.UpdatedAt = now
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit concept touch: %w", err)
	}
	return &concept, nil
}
