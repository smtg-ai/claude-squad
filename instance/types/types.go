package types

import "encoding/json"

// TaggedInstance is used for type-tagged serialization
type TaggedInstance struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

const (
	TypeTask         = "task"
	TypeOrchestrator = "orchestrator"
)
