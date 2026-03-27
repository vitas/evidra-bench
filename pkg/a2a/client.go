package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultPollInterval = 2 * time.Second
const defaultProtocolVersion = "0.3"

// Client is a minimal JSON-RPC A2A client.
type Client struct {
	BaseURL      string
	HTTPClient   *http.Client
	PollInterval time.Duration
}

// AgentCard contains the fields needed for discovery.
type AgentCard struct {
	Name                string           `json:"name"`
	URL                 string           `json:"url"`
	SupportedInterfaces []AgentInterface `json:"supportedInterfaces,omitempty"`
}

// AgentInterface declares a concrete A2A endpoint.
type AgentInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
	ProtocolVersion string `json:"protocolVersion"`
}

// TaskResult is the normalized outcome of a remote A2A task.
type TaskResult struct {
	AgentName string
	RPCURL    string
	TaskID    string
	ContextID string
	State     string
	Output    string
	Completed bool
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type sendResultEnvelope struct {
	Task    *task    `json:"task"`
	Message *message `json:"message"`
}

type task struct {
	ID        string     `json:"id"`
	ContextID string     `json:"contextId"`
	Status    taskStatus `json:"status"`
	Artifacts []artifact `json:"artifacts"`
}

type taskStatus struct {
	State   string   `json:"state"`
	Message *message `json:"message,omitempty"`
}

type artifact struct {
	Parts []part `json:"parts"`
}

type message struct {
	MessageID string `json:"messageId,omitempty"`
	Role      string `json:"role,omitempty"`
	Parts     []part `json:"parts"`
}

type part struct {
	Kind string `json:"kind,omitempty"`
	Text string `json:"text,omitempty"`
}

// NewClient creates a client with sane defaults.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:      strings.TrimRight(baseURL, "/"),
		HTTPClient:   httpClient,
		PollInterval: defaultPollInterval,
	}
}

// Discover fetches the public agent card.
func (c *Client) Discover(ctx context.Context) (*AgentCard, error) {
	cardURL, err := resolveURL(c.BaseURL, "/.well-known/agent-card.json")
	if err != nil {
		return nil, fmt.Errorf("a2a: resolve agent card url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("a2a: build agent card request: %w", err)
	}
	req.Header.Set("A2A-Version", defaultProtocolVersion)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: discover agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("a2a: discover agent card: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var card AgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, fmt.Errorf("a2a: decode agent card: %w", err)
	}
	return &card, nil
}

// RunTextTask sends a text task prompt and waits for a terminal response.
func (c *Client) RunTextTask(ctx context.Context, requestID, prompt string) (*TaskResult, error) {
	card, err := c.Discover(ctx)
	if err != nil {
		return nil, err
	}

	rpcURL, version, err := endpointForCard(c.BaseURL, card)
	if err != nil {
		return nil, err
	}

	raw, err := c.call(ctx, rpcURL, version, requestID, "message/send", map[string]any{
		"message": map[string]any{
			"kind":      "message",
			"messageId": requestID,
			"role":      "user",
			"parts":     []map[string]any{{"kind": "text", "text": prompt}},
		},
	})
	if err != nil {
		return nil, err
	}

	result, err := c.parseSendResult(card.Name, rpcURL, raw)
	if err != nil {
		return nil, err
	}
	if result.State == "" || result.Completed || isTerminalFailure(result.State) {
		return result, nil
	}
	if result.TaskID == "" {
		return nil, fmt.Errorf("a2a: non-terminal task missing id")
	}

	pollEvery := c.PollInterval
	if pollEvery <= 0 {
		pollEvery = defaultPollInterval
	}

	for {
		timer := time.NewTimer(pollEvery)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		raw, err = c.call(ctx, rpcURL, version, requestID, "tasks/get", map[string]any{
			"id": result.TaskID,
		})
		if err != nil {
			return nil, err
		}

		result, err = c.parseTaskResult(card.Name, rpcURL, raw)
		if err != nil {
			return nil, err
		}
		if result.Completed || isTerminalFailure(result.State) {
			return result, nil
		}
	}
}

func (c *Client) call(ctx context.Context, rpcURL, version, requestID, method string, params any) (json.RawMessage, error) {
	payload, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		ID:      requestID,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("a2a: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("a2a: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("A2A-Version", version)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("a2a: send %s: %w", method, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("a2a: read %s response: %w", method, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("a2a: %s status %d: %s", method, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("a2a: decode %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("a2a: rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func (c *Client) parseSendResult(agentName, rpcURL string, raw json.RawMessage) (*TaskResult, error) {
	var envelope sendResultEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil {
		switch {
		case envelope.Task != nil:
			return taskToResult(agentName, rpcURL, envelope.Task), nil
		case envelope.Message != nil:
			return messageToResult(agentName, rpcURL, envelope.Message), nil
		}
	}

	var t task
	if err := json.Unmarshal(raw, &t); err == nil && (t.ID != "" || t.Status.State != "" || len(t.Artifacts) > 0) {
		return taskToResult(agentName, rpcURL, &t), nil
	}

	var msg message
	if err := json.Unmarshal(raw, &msg); err == nil && len(msg.Parts) > 0 {
		return messageToResult(agentName, rpcURL, &msg), nil
	}

	return nil, fmt.Errorf("a2a: unrecognized send result")
}

func (c *Client) parseTaskResult(agentName, rpcURL string, raw json.RawMessage) (*TaskResult, error) {
	var envelope sendResultEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Task != nil {
		return taskToResult(agentName, rpcURL, envelope.Task), nil
	}

	var t task
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("a2a: decode task result: %w", err)
	}
	return taskToResult(agentName, rpcURL, &t), nil
}

func endpointForCard(baseURL string, card *AgentCard) (string, string, error) {
	for _, iface := range card.SupportedInterfaces {
		if strings.EqualFold(iface.ProtocolBinding, "JSONRPC") || strings.EqualFold(iface.ProtocolBinding, "JSON-RPC") {
			version := iface.ProtocolVersion
			if version == "" {
				version = defaultProtocolVersion
			}
			return iface.URL, version, nil
		}
	}

	if card.URL != "" {
		return card.URL, defaultProtocolVersion, nil
	}
	if baseURL == "" {
		return "", "", fmt.Errorf("a2a: missing base URL")
	}
	return baseURL, defaultProtocolVersion, nil
}

func resolveURL(baseURL, ref string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	target, err := url.Parse(ref)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(target).String(), nil
}

func taskToResult(agentName, rpcURL string, t *task) *TaskResult {
	_, completed := classifyState(t.Status.State)
	return &TaskResult{
		AgentName: agentName,
		RPCURL:    rpcURL,
		TaskID:    t.ID,
		ContextID: t.ContextID,
		State:     t.Status.State,
		Output:    collectTaskOutput(t),
		Completed: completed,
	}
}

func messageToResult(agentName, rpcURL string, msg *message) *TaskResult {
	return &TaskResult{
		AgentName: agentName,
		RPCURL:    rpcURL,
		State:     "completed",
		Output:    collectParts(msg.Parts),
		Completed: true,
	}
}

func collectTaskOutput(t *task) string {
	var chunks []string
	for _, artifact := range t.Artifacts {
		text := collectParts(artifact.Parts)
		if text != "" {
			chunks = append(chunks, text)
		}
	}
	if len(chunks) > 0 {
		return strings.Join(chunks, "\n")
	}
	if t.Status.Message != nil {
		return collectParts(t.Status.Message.Parts)
	}
	return ""
}

func collectParts(parts []part) string {
	var chunks []string
	for _, p := range parts {
		if p.Text != "" {
			chunks = append(chunks, p.Text)
		}
	}
	return strings.Join(chunks, "\n")
}

func classifyState(state string) (terminal bool, completed bool) {
	switch strings.ToUpper(state) {
	case "COMPLETED", "TASK_STATE_COMPLETED":
		return true, true
	case "FAILED", "TASK_STATE_FAILED", "REJECTED", "TASK_STATE_REJECTED", "CANCELED", "TASK_STATE_CANCELED", "CANCELLED", "TASK_STATE_CANCELLED":
		return true, false
	default:
		return false, false
	}
}

func isTerminalFailure(state string) bool {
	terminal, completed := classifyState(state)
	return terminal && !completed
}
