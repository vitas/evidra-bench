package agentgateway

import "time"

// Kind identifies the normalized event class.
type Kind string

const (
	KindGatewayCall    Kind = "gateway_call"
	KindPolicyDecision Kind = "policy_decision"
	KindClusterEffect  Kind = "cluster_effect"
)

// Decision identifies a policy outcome.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Event is the normalized AgentGateway-facing event shape.
type Event struct {
	Kind Kind `json:"kind"`

	Timestamp time.Time `json:"timestamp"`
	TraceID   string    `json:"trace_id,omitempty"`
	SessionID string    `json:"session_id,omitempty"`

	Target string `json:"target,omitempty"`
	Method string `json:"method,omitempty"`
	Tool   string `json:"tool,omitempty"`

	Status int    `json:"status,omitempty"`
	Reason string `json:"reason,omitempty"`

	Decision     Decision `json:"decision,omitempty"`
	PolicyID     string   `json:"policy_id,omitempty"`
	PolicyReason string   `json:"policy_reason,omitempty"`

	Namespace    string `json:"namespace,omitempty"`
	Resource     string `json:"resource,omitempty"`
	Verb         string `json:"verb,omitempty"`
	ResourceKind string `json:"resource_kind,omitempty"`
	ResourceName string `json:"resource_name,omitempty"`
	ResultCode   int    `json:"result_code,omitempty"`
}
