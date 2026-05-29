package benchsvc

import (
	"context"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// ExecutorContractVersion is the protocol version for bench executor communication.
const ExecutorContractVersion = "v1.0.0"

const (
	// ExecutionModeProvider runs scenarios through the built-in provider/tool loop.
	ExecutionModeProvider = "provider"
	// ExecutionModeA2A delegates scenario execution to an external A2A agent.
	ExecutionModeA2A = "a2a"
)

// TriggerRequest is the payload for POST /v1/bench/trigger.
type TriggerRequest struct {
	Model             string   `json:"model"`
	Provider          string   `json:"provider,omitempty"`
	RunnerID          string   `json:"runner_id,omitempty"`
	ExecutionMode     string   `json:"execution_mode,omitempty"`
	MCPServer         string   `json:"mcp_server,omitempty"`
	ToolServer        string   `json:"tool_server,omitempty"`
	ToolServerVersion string   `json:"tool_server_version,omitempty"`
	SkillFile         string   `json:"skill_file,omitempty"`
	SkillID           string   `json:"skill_id,omitempty"`
	SkillVersion      string   `json:"skill_version,omitempty"`
	SkillSource       string   `json:"skill_source,omitempty"`
	SkillSHA256       string   `json:"skill_sha256,omitempty"`
	Scenarios         []string `json:"scenarios"`
}

// TriggerJob tracks a bench trigger execution.
type TriggerJob struct {
	ID                string             `json:"id"`
	Status            string             `json:"status"` // pending, running, completed, failed
	Model             string             `json:"model"`
	Provider          string             `json:"provider,omitempty"`
	ExecutionMode     string             `json:"execution_mode,omitempty"`
	MCPServer         string             `json:"mcp_server,omitempty"`
	ToolServer        string             `json:"tool_server,omitempty"`
	ToolServerVersion string             `json:"tool_server_version,omitempty"`
	SkillFile         string             `json:"skill_file,omitempty"`
	SkillID           string             `json:"skill_id,omitempty"`
	SkillVersion      string             `json:"skill_version,omitempty"`
	SkillSource       string             `json:"skill_source,omitempty"`
	SkillSHA256       string             `json:"skill_sha256,omitempty"`
	Total             int                `json:"total"`
	Completed         int                `json:"completed"`
	Passed            int                `json:"passed"`
	Failed            int                `json:"failed"`
	CurrentScenario   string             `json:"current_scenario,omitempty"`
	RunIDs            []string           `json:"run_ids,omitempty"`
	Progress          []ScenarioProgress `json:"progress"`
	CreatedAt         time.Time          `json:"created_at"`
}

// ScenarioProgress tracks the status of a single scenario within a job.
type ScenarioProgress struct {
	Scenario string `json:"scenario"`
	Status   string `json:"status"` // pending, running, passed, failed
	RunID    string `json:"run_id,omitempty"`
}

// ProgressUpdate is the payload sent by the bench service callback.
type ProgressUpdate struct {
	ContractVersion string `json:"contract_version,omitempty"`
	JobID           string `json:"job_id"`
	Scenario        string `json:"scenario"`
	Status          string `json:"status"`
	RunID           string `json:"run_id,omitempty"`
	Completed       int    `json:"completed"`
	Total           int    `json:"total"`
}

// RunExecutor starts a bench job against an external service.
type RunExecutor interface {
	Start(ctx context.Context, job *TriggerJob, benchURL string, apiKey string) error
}

// TriggerStore is an in-memory store for trigger jobs with SSE notification support.
type TriggerStore struct {
	mu          sync.RWMutex
	jobs        map[string]*TriggerJob
	subscribers map[string][]chan ProgressUpdate
}

// NewTriggerStore creates a new TriggerStore.
func NewTriggerStore() *TriggerStore {
	return &TriggerStore{
		jobs:        make(map[string]*TriggerJob),
		subscribers: make(map[string][]chan ProgressUpdate),
	}
}

// NewJobID generates a new ULID for a trigger job.
func NewJobID() string {
	return ulid.Make().String()
}

// Create stores a new trigger job.
func (s *TriggerStore) Create(job *TriggerJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

// Get retrieves a trigger job by ID. Returns nil if not found.
func (s *TriggerStore) Get(id string) *TriggerJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j := s.jobs[id]
	if j == nil {
		return nil
	}
	// Return a copy to avoid races.
	cp := *j
	cp.Progress = make([]ScenarioProgress, len(j.Progress))
	copy(cp.Progress, j.Progress)
	cp.RunIDs = make([]string, len(j.RunIDs))
	copy(cp.RunIDs, j.RunIDs)
	return &cp
}

// Update applies a progress update to the job and notifies subscribers.
func (s *TriggerStore) Update(u ProgressUpdate) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[u.JobID]
	if !ok {
		return false
	}

	// Update scenario progress.
	for i := range job.Progress {
		if job.Progress[i].Scenario == u.Scenario {
			job.Progress[i].Status = u.Status
			job.Progress[i].RunID = u.RunID
			break
		}
	}

	job.Completed = u.Completed
	job.Total = u.Total
	if u.RunID != "" {
		job.RunIDs = append(job.RunIDs, u.RunID)
	}

	switch u.Status {
	case "passed":
		job.Passed++
	case "failed":
		job.Failed++
	case "error":
		job.Failed++
	case "running":
		job.CurrentScenario = u.Scenario
	}

	// Determine job-level status.
	if job.Completed >= job.Total {
		if job.Failed > 0 {
			job.Status = "failed"
		} else {
			job.Status = "completed"
		}
		job.CurrentScenario = ""
	} else {
		job.Status = "running"
	}

	// Notify subscribers.
	for _, ch := range s.subscribers[u.JobID] {
		select {
		case ch <- u:
		default:
			// Drop if subscriber is slow.
		}
	}

	// Clean up subscriber channels on terminal state.
	if job.Status == "completed" || job.Status == "failed" {
		for _, ch := range s.subscribers[u.JobID] {
			close(ch)
		}
		delete(s.subscribers, u.JobID)
	}

	return true
}

// Complete applies a final runner completion update to a trigger job snapshot.
func (s *TriggerStore) Complete(jobID, status string, passed, failed int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[jobID]
	if !ok {
		return false
	}

	job.Status = status
	job.Passed = passed
	job.Failed = failed
	job.Completed = passed + failed
	job.CurrentScenario = ""

	update := ProgressUpdate{
		JobID:     jobID,
		Status:    status,
		Completed: job.Completed,
		Total:     job.Total,
	}
	for _, ch := range s.subscribers[jobID] {
		select {
		case ch <- update:
		default:
		}
		close(ch)
	}
	delete(s.subscribers, jobID)

	return true
}

// Subscribe returns a channel that receives progress updates for a job.
func (s *TriggerStore) Subscribe(jobID string) chan ProgressUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan ProgressUpdate, 16)
	s.subscribers[jobID] = append(s.subscribers[jobID], ch)
	return ch
}

// Unsubscribe removes a subscriber channel for a job.
// The channel may already be closed by Update on terminal state, so
// Unsubscribe only removes the entry without closing again.
func (s *TriggerStore) Unsubscribe(jobID string, ch chan ProgressUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	subs := s.subscribers[jobID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[jobID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}
