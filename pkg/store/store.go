// Package store provides structured result storage with SQLite + JSONL backup.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// RunRecord is a single benchmark run stored in the database.
type RunRecord struct {
	ID               string    `json:"id"`
	ScenarioID       string    `json:"scenario_id"`
	Model            string    `json:"model"`
	Provider         string    `json:"provider"`
	Adapter          string    `json:"adapter"`
	Passed           bool      `json:"passed"`
	Duration         float64   `json:"duration_seconds"`
	ExitCode         int       `json:"exit_code"`
	Turns            int       `json:"turns"`
	MemoryWindow     int       `json:"memory_window"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	EstimatedCost    float64   `json:"estimated_cost_usd"`
	ChecksPassed     int       `json:"checks_passed"`
	ChecksTotal      int       `json:"checks_total"`
	ChecksJSON       string    `json:"checks_json,omitempty"`
	MetadataJSON     string    `json:"metadata_json,omitempty"`
	ArtifactDir      string    `json:"artifact_dir"`
	CreatedAt        time.Time `json:"created_at"`
}

// Store manages the results database and JSONL backup.
type Store struct {
	db        *sql.DB
	dbPath    string
	jsonlPath string
}

// Open opens or creates the results store.
func Open(runsDir string) (*Store, error) {
	dbPath := filepath.Join(runsDir, "bench.db")
	jsonlPath := filepath.Join(runsDir, "results.jsonl")

	if err := os.MkdirAll(runsDir, 0755); err != nil {
		return nil, fmt.Errorf("store.Open: mkdir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("store.Open: %w", err)
	}

	s := &Store{db: db, dbPath: dbPath, jsonlPath: jsonlPath}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id               TEXT PRIMARY KEY,
			scenario_id      TEXT NOT NULL,
			model            TEXT NOT NULL DEFAULT '',
			provider         TEXT NOT NULL DEFAULT '',
			adapter          TEXT NOT NULL DEFAULT '',
			passed           BOOLEAN NOT NULL DEFAULT 0,
			duration_seconds REAL NOT NULL DEFAULT 0,
			exit_code        INTEGER NOT NULL DEFAULT 0,
			turns            INTEGER NOT NULL DEFAULT 0,
			memory_window    INTEGER NOT NULL DEFAULT -1,
			prompt_tokens    INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			estimated_cost   REAL NOT NULL DEFAULT 0,
			checks_passed    INTEGER NOT NULL DEFAULT 0,
			checks_total     INTEGER NOT NULL DEFAULT 0,
			checks_json      TEXT NOT NULL DEFAULT '',
			metadata_json    TEXT NOT NULL DEFAULT '',
			artifact_dir     TEXT NOT NULL DEFAULT '',
			created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_runs_scenario ON runs(scenario_id);
		CREATE INDEX IF NOT EXISTS idx_runs_model ON runs(model);
		CREATE INDEX IF NOT EXISTS idx_runs_provider ON runs(provider);
		CREATE INDEX IF NOT EXISTS idx_runs_created ON runs(created_at);
	`)
	if err != nil {
		return err
	}
	return s.ensureColumn("runs", "metadata_json", "TEXT NOT NULL DEFAULT ''")
}

// Insert adds a run record to the database and appends to JSONL backup.
func (s *Store) Insert(r RunRecord) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO runs (
			id, scenario_id, model, provider, adapter, passed,
			duration_seconds, exit_code, turns, memory_window,
			prompt_tokens, completion_tokens, estimated_cost,
			checks_passed, checks_total, checks_json, metadata_json, artifact_dir, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.Passed,
		r.Duration, r.ExitCode, r.Turns, r.MemoryWindow,
		r.PromptTokens, r.CompletionTokens, r.EstimatedCost,
		r.ChecksPassed, r.ChecksTotal, r.ChecksJSON, r.MetadataJSON, r.ArtifactDir, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store.Insert: %w", err)
	}
	return s.appendJSONL(r)
}

func (s *Store) appendJSONL(r RunRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.jsonlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}

// Query returns runs matching the given filters.
func (s *Store) Query(filters QueryFilters) ([]RunRecord, error) {
	query := "SELECT id, scenario_id, model, provider, adapter, passed, duration_seconds, exit_code, turns, memory_window, prompt_tokens, completion_tokens, estimated_cost, checks_passed, checks_total, checks_json, metadata_json, artifact_dir, created_at FROM runs WHERE 1=1"
	var args []any

	if filters.ScenarioID != "" {
		query += " AND scenario_id = ?"
		args = append(args, filters.ScenarioID)
	}
	if filters.Model != "" {
		query += " AND model = ?"
		args = append(args, filters.Model)
	}
	if filters.Provider != "" {
		query += " AND provider = ?"
		args = append(args, filters.Provider)
	}
	if filters.PassedOnly {
		query += " AND passed = 1"
	}
	if filters.FailedOnly {
		query += " AND passed = 0"
	}
	if !filters.Since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filters.Since)
	}
	query += " ORDER BY created_at DESC"
	if filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.Query: %w", err)
	}
	defer rows.Close()

	var records []RunRecord
	for rows.Next() {
		var r RunRecord
		if err := rows.Scan(&r.ID, &r.ScenarioID, &r.Model, &r.Provider, &r.Adapter, &r.Passed, &r.Duration, &r.ExitCode, &r.Turns, &r.MemoryWindow, &r.PromptTokens, &r.CompletionTokens, &r.EstimatedCost, &r.ChecksPassed, &r.ChecksTotal, &r.ChecksJSON, &r.MetadataJSON, &r.ArtifactDir, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("store.Query: scan: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// QueryFilters specifies which runs to return.
type QueryFilters struct {
	ScenarioID string
	Model      string
	Provider   string
	PassedOnly bool
	FailedOnly bool
	Since      time.Time
	Limit      int
}

// Stats returns aggregate statistics.
func (s *Store) Stats() (*StatsResult, error) {
	var st StatsResult
	err := s.db.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN passed THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN NOT passed THEN 1 ELSE 0 END),0) FROM runs").Scan(&st.TotalRuns, &st.PassCount, &st.FailCount)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT scenario_id, COUNT(*), SUM(CASE WHEN passed THEN 1 ELSE 0 END) FROM runs GROUP BY scenario_id ORDER BY scenario_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ss ScenarioStat
		if err := rows.Scan(&ss.ScenarioID, &ss.Runs, &ss.Passed); err != nil {
			return nil, err
		}
		st.ByScenario = append(st.ByScenario, ss)
	}
	return &st, rows.Err()
}

// StatsResult holds aggregate statistics.
type StatsResult struct {
	TotalRuns  int
	PassCount  int
	FailCount  int
	ByScenario []ScenarioStat
}

// ScenarioStat holds per-scenario stats.
type ScenarioStat struct {
	ScenarioID string
	Runs       int
	Passed     int
}

// Rebuild recreates the database from the JSONL backup file.
func (s *Store) Rebuild() (int, error) {
	data, err := os.ReadFile(s.jsonlPath)
	if err != nil {
		return 0, fmt.Errorf("store.Rebuild: read jsonl: %w", err)
	}
	// Drop and recreate
	if _, err := s.db.Exec("DELETE FROM runs"); err != nil {
		return 0, err
	}
	count := 0
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var r RunRecord
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		if _, err := s.db.Exec(`
			INSERT OR REPLACE INTO runs (
				id, scenario_id, model, provider, adapter, passed,
				duration_seconds, exit_code, turns, memory_window,
				prompt_tokens, completion_tokens, estimated_cost,
				checks_passed, checks_total, checks_json, metadata_json, artifact_dir, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.ID, r.ScenarioID, r.Model, r.Provider, r.Adapter, r.Passed,
			r.Duration, r.ExitCode, r.Turns, r.MemoryWindow,
			r.PromptTokens, r.CompletionTokens, r.EstimatedCost,
			r.ChecksPassed, r.ChecksTotal, r.ChecksJSON, r.MetadataJSON, r.ArtifactDir, r.CreatedAt,
		); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

func (s *Store) ensureColumn(table, column, definition string) error {
	_, err := s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "duplicate column name") {
		return nil
	}
	return err
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := range data {
		if data[i] == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
