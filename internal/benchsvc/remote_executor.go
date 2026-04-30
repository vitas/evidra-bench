package benchsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RemoteExecutor delegates bench job execution to an external bench service via REST.
type RemoteExecutor struct {
	ServiceURL string
	HTTPClient *http.Client
}

// NewRemoteExecutor creates a RemoteExecutor pointing at the given service URL.
func NewRemoteExecutor(serviceURL string) *RemoteExecutor {
	return &RemoteExecutor{
		ServiceURL: serviceURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// certifyRequest is the payload sent to the bench service.
type certifyRequest struct {
	ContractVersion string          `json:"contract_version"`
	JobID           string          `json:"job_id"`
	Model           string          `json:"model"`
	Provider        string          `json:"provider,omitempty"`
	Scenarios       []string        `json:"scenarios"`
	Config          map[string]any  `json:"config"`
	Callback        certifyCallback `json:"callback"`
}

type certifyCallback struct {
	ProgressURL  string `json:"progress_url"`
	EvidraURL    string `json:"evidra_url"`
	EvidraAPIKey string `json:"evidra_api_key"`
}

func buildCertifyConfig(job *TriggerJob) map[string]any {
	cfg := map[string]any{
		"timeout_per_scenario": 300,
		"evidence_mode":        job.EvidenceMode,
	}
	if job.ExecutionMode == "a2a" {
		cfg["adapter"] = "a2a"
	}
	return cfg
}

// Start sends a POST to the bench service to begin scenario execution.
func (e *RemoteExecutor) Start(ctx context.Context, job *TriggerJob, evidraURL string, apiKey string) error {
	scenarios := make([]string, len(job.Progress))
	for i, p := range job.Progress {
		scenarios[i] = p.Scenario
	}

	req := certifyRequest{
		ContractVersion: ExecutorContractVersion,
		JobID:           job.ID,
		Model:           job.Model,
		Provider:        job.Provider,
		Scenarios:       scenarios,
		Config:          buildCertifyConfig(job),
		Callback: certifyCallback{
			ProgressURL:  evidraURL + "/v1/bench/trigger/" + job.ID + "/progress",
			EvidraURL:    evidraURL,
			EvidraAPIKey: apiKey,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("benchsvc.RemoteExecutor: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.ServiceURL+"/v1/certify", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("benchsvc.RemoteExecutor: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", apiKey)
	}

	resp, err := e.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("benchsvc.RemoteExecutor: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("benchsvc.RemoteExecutor: unexpected status %d", resp.StatusCode)
	}

	return nil
}
