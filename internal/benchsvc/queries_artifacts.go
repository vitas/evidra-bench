package benchsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// StoreArtifact upserts an artifact for a given run.
// If the artifact already exists, the data is replaced.
func (s *PgStore) StoreArtifact(ctx context.Context, runID, artifactType, contentType string, data []byte) error {
	query := `INSERT INTO bench_artifacts (run_id, artifact_type, content_type, data)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (run_id, artifact_type) DO UPDATE SET data = EXCLUDED.data, content_type = EXCLUDED.content_type`
	_, err := s.db.Exec(ctx, query, runID, artifactType, contentType, data)
	if err != nil {
		return fmt.Errorf("bench.StoreArtifact: %w", err)
	}
	return nil
}

// GetArtifact retrieves an artifact by run ID and type.
// It verifies the run belongs to the given tenant before returning data.
// Returns data, contentType, error.
func (s *PgStore) GetArtifact(ctx context.Context, tenantID string, runID, artifactType string) ([]byte, string, error) {
	query := `SELECT a.data, a.content_type FROM bench_artifacts a
		JOIN bench_runs r ON r.id = a.run_id
		WHERE r.tenant_id = $1 AND a.run_id = $2 AND a.artifact_type = $3`
	var data []byte
	var ct string
	err := s.db.QueryRow(ctx, query, tenantID, runID, artifactType).Scan(&data, &ct)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", fmt.Errorf("bench.GetArtifact: %w", err)
	}
	return data, ct, nil
}
