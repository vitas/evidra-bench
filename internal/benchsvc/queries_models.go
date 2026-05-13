package benchsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ListEnabledModels returns models available to a tenant.
// A model is available if it has a platform API key env var or
// the tenant has an enabled provider entry for that model.
func (s *PgStore) ListEnabledModels(ctx context.Context, tenantID string) ([]EnabledModel, error) {
	rows, err := s.db.Query(ctx, `
		SELECT m.id, m.display_name, m.provider, m.api_base_url, m.api_key_env,
		       m.input_cost_per_mtok, m.output_cost_per_mtok
		FROM bench_models m
		LEFT JOIN bench_tenant_providers tp
		  ON tp.model_id = m.id AND tp.tenant_id = $1 AND tp.enabled = true
		WHERE m.api_key_env != '' OR tp.tenant_id IS NOT NULL
		ORDER BY m.provider, m.display_name
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("benchsvc.ListEnabledModels: %w", err)
	}
	defer rows.Close()

	var models []EnabledModel
	for rows.Next() {
		var model EnabledModel
		if err := rows.Scan(
			&model.ID,
			&model.DisplayName,
			&model.Provider,
			&model.APIBaseURL,
			&model.APIKeyEnv,
			&model.InputCostPerMtok,
			&model.OutputCostPerMtok,
		); err != nil {
			return nil, fmt.Errorf("benchsvc.ListEnabledModels scan: %w", err)
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("benchsvc.ListEnabledModels rows: %w", err)
	}
	return models, nil
}

// UpsertTenantProvider inserts or updates a tenant provider override for a model.
func (s *PgStore) UpsertTenantProvider(ctx context.Context, tenantID, modelID string, cfg TenantProviderConfig) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO bench_tenant_providers (tenant_id, model_id, api_key_enc, api_base_url, rate_limit, monthly_budget, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, true)
		ON CONFLICT (tenant_id, model_id) DO UPDATE SET
			api_key_enc = CASE WHEN $3 != '' THEN $3 ELSE bench_tenant_providers.api_key_enc END,
			api_base_url = CASE WHEN $4 != '' THEN $4 ELSE bench_tenant_providers.api_base_url END,
			rate_limit = CASE WHEN $5 > 0 THEN $5 ELSE bench_tenant_providers.rate_limit END,
			monthly_budget = CASE WHEN $6 > 0 THEN $6 ELSE bench_tenant_providers.monthly_budget END,
			enabled = true,
			updated_at = NOW()
	`, tenantID, modelID, cfg.APIKeyEnc, cfg.APIBaseURL, cfg.RateLimit, cfg.MonthlyBudget)
	if err != nil {
		return fmt.Errorf("benchsvc.UpsertTenantProvider: %w", err)
	}
	return nil
}

// DeleteTenantProvider removes a tenant-specific provider override for a model.
func (s *PgStore) DeleteTenantProvider(ctx context.Context, tenantID, modelID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM bench_tenant_providers
		WHERE tenant_id = $1 AND model_id = $2
	`, tenantID, modelID)
	if err != nil {
		return fmt.Errorf("benchsvc.DeleteTenantProvider: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateGlobalModel updates platform-level defaults for a model.
func (s *PgStore) UpdateGlobalModel(ctx context.Context, modelID string, cfg GlobalModelConfig) error {
	result, err := s.db.Exec(ctx, `
		UPDATE bench_models SET
			api_base_url = CASE WHEN $2 != '' THEN $2 ELSE api_base_url END,
			api_key_env = CASE WHEN $3 != '' THEN $3 ELSE api_key_env END,
			updated_at = NOW()
		WHERE id = $1
	`, modelID, cfg.APIBaseURL, cfg.APIKeyEnv)
	if err != nil {
		return fmt.Errorf("benchsvc.UpdateGlobalModel: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("benchsvc.UpdateGlobalModel: model %q not found", modelID)
	}
	return nil
}

// ResolveModelProvider looks up a model's provider and base URL from the global catalog.
func (s *PgStore) ResolveModelProvider(ctx context.Context, modelID string) (*ModelProviderInfo, error) {
	var info ModelProviderInfo
	err := s.db.QueryRow(ctx,
		`SELECT provider, api_base_url, api_key_env FROM bench_models WHERE id = $1`, modelID,
	).Scan(&info.Provider, &info.APIBaseURL, &info.APIKeyEnv)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("benchsvc.ResolveModelProvider: model %q: %w", modelID, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("benchsvc.ResolveModelProvider: %w", err)
	}
	return &info, nil
}
