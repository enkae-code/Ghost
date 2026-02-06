package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	_ "modernc.org/sqlite"

	"ghost/kernel/internal/domain"
)

// SQLiteRepository manages artifact persistence in SQLite
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository and initializes the database
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Create artifacts table if not exists
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS artifacts (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		content TEXT NOT NULL,
		type TEXT NOT NULL,
		bounding_box TEXT NOT NULL,
		classification TEXT,
		summary TEXT,
		embedding TEXT
	);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		return nil, fmt.Errorf("failed to create artifacts table: %w", err)
	}

	// Migrate existing tables to add new columns if they don't exist
	migrateSQL := []string{
		"ALTER TABLE artifacts ADD COLUMN classification TEXT;",
		"ALTER TABLE artifacts ADD COLUMN summary TEXT;",
		"ALTER TABLE artifacts ADD COLUMN embedding TEXT;",
	}

	for _, stmt := range migrateSQL {
		// Ignore errors if columns already exist
		_, _ = db.Exec(stmt)
	}

	return &SQLiteRepository{db: db}, nil
}

// Save persists an artifact to the database
func (r *SQLiteRepository) Save(ctx context.Context, artifact domain.Artifact) error {
	// Serialize bounding box to JSON
	boundingBoxJSON, err := json.Marshal(artifact.BoundingBox)
	if err != nil {
		return fmt.Errorf("failed to marshal bounding box: %w", err)
	}

	insertSQL := `
	INSERT INTO artifacts (id, timestamp, content, type, bounding_box)
	VALUES (?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(
		ctx,
		insertSQL,
		artifact.ID,
		artifact.Timestamp,
		artifact.Content,
		string(artifact.Type),
		string(boundingBoxJSON),
	)

	if err != nil {
		return fmt.Errorf("failed to insert artifact: %w", err)
	}

	return nil
}

// GetLastArtifacts retrieves the last N artifacts from the database
func (r *SQLiteRepository) GetLastArtifacts(ctx context.Context, limit int) ([]domain.Artifact, error) {
	query := `
	SELECT id, timestamp, content, type, bounding_box, classification, summary
	FROM artifacts
	ORDER BY timestamp DESC
	LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []domain.Artifact
	for rows.Next() {
		var artifact domain.Artifact
		var boundingBoxJSON string
		var artifactType string
		var classification sql.NullString
		var summary sql.NullString

		err := rows.Scan(
			&artifact.ID,
			&artifact.Timestamp,
			&artifact.Content,
			&artifactType,
			&boundingBoxJSON,
			&classification,
			&summary,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan artifact: %w", err)
		}

		artifact.Type = domain.ArtifactType(artifactType)

		if err := json.Unmarshal([]byte(boundingBoxJSON), &artifact.BoundingBox); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bounding box: %w", err)
		}

		if classification.Valid {
			artifact.Classification = classification.String
		}
		if summary.Valid {
			artifact.Summary = summary.String
		}

		artifacts = append(artifacts, artifact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return artifacts, nil
}

// UpdateArtifact enriches an artifact with classification, summary, and embedding from LLM analysis
func (r *SQLiteRepository) UpdateArtifact(ctx context.Context, id string, classification string, summary string, embedding string) error {
	updateSQL := `
	UPDATE artifacts
	SET classification = ?, summary = ?, embedding = ?
	WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, updateSQL, classification, summary, embedding, id)
	if err != nil {
		return fmt.Errorf("failed to update artifact: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("artifact not found: %s", id)
	}

	return nil
}

// SearchArtifacts performs semantic search using cosine similarity
func (r *SQLiteRepository) SearchArtifacts(ctx context.Context, queryEmbedding []float32, limit int) ([]domain.Artifact, error) {
	// Get all artifacts with embeddings
	query := `
	SELECT id, timestamp, content, type, bounding_box, classification, summary, embedding
	FROM artifacts
	WHERE embedding IS NOT NULL
	ORDER BY timestamp DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts for search: %w", err)
	}
	defer rows.Close()

	var artifacts []domain.Artifact
	var results []struct {
		artifact   domain.Artifact
		embedding  []float32
		similarity float32
	}

	for rows.Next() {
		var artifact domain.Artifact
		var boundingBoxJSON string
		var artifactType string
		var classification sql.NullString
		var summary sql.NullString
		var embeddingJSON sql.NullString

		err := rows.Scan(
			&artifact.ID,
			&artifact.Timestamp,
			&artifact.Content,
			&artifactType,
			&boundingBoxJSON,
			&classification,
			&summary,
			&embeddingJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan artifact: %w", err)
		}

		artifact.Type = domain.ArtifactType(artifactType)

		if err := json.Unmarshal([]byte(boundingBoxJSON), &artifact.BoundingBox); err != nil {
			return nil, fmt.Errorf("failed to unmarshal bounding box: %w", err)
		}

		if classification.Valid {
			artifact.Classification = classification.String
		}
		if summary.Valid {
			artifact.Summary = summary.String
		}

		// Parse embedding if available
		var embedding []float32
		if embeddingJSON.Valid && embeddingJSON.String != "" {
			var embeddingSlice []float32
			if err := json.Unmarshal([]byte(embeddingJSON.String), &embeddingSlice); err == nil {
				embedding = embeddingSlice
			}
		}

		// Calculate cosine similarity
		var similarity float32
		if len(embedding) > 0 && len(queryEmbedding) > 0 {
			similarity = cosineSimilarity(queryEmbedding, embedding)
		}

		results = append(results, struct {
			artifact   domain.Artifact
			embedding  []float32
			similarity float32
		}{artifact, embedding, similarity})
	}

	// Sort by similarity and take top results
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].similarity > results[i].similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top results
	maxResults := limit
	if maxResults > len(results) {
		maxResults = len(results)
	}

	for i := 0; i < maxResults; i++ {
		artifacts = append(artifacts, results[i].artifact)
	}

	return artifacts, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// GetDB returns the underlying database connection
func (r *SQLiteRepository) GetDB() *sql.DB {
	return r.db
}

// Close closes the database connection
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}
