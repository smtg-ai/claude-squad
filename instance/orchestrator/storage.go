package orchestrator

import (
	"claude-squad/config"
	"claude-squad/instance/task"
	"encoding/json"
	"fmt"
)

// OrchestratorData is the serializable form of Orchestrator
// Only serializable fields and TaskData references are included
// Leader and Workers are stored as TaskData
// Tasks is a slice of lightweight Task
// Completed is a map[string]bool
// Status is stored as int

// Serialize returns the JSON encoding of the orchestrator's serializable state
func (o *Orchestrator) Serialize() []byte {
	data, err := json.Marshal(o.ToOrchestratorData())
	if err != nil {
		return nil
	}
	return data
}

// Deserialize populates the orchestrator from JSON
func (o *Orchestrator) Deserialize(data []byte) error {
	var od OrchestratorData
	if err := json.Unmarshal(data, &od); err != nil {
		return err
	}
	// Copy fields
	leader := (*task.Task)(nil)
	if od.Leader != nil {
		var err error
		leader, err = task.FromInstanceData(*od.Leader)
		if err != nil {
			return err
		}
	}
	workers := make(map[string]*task.Task)
	for k, v := range od.Workers {
		w, err := task.FromInstanceData(v)
		if err != nil {
			return err
		}
		workers[k] = w
	}
	// Assign all fields
	o.Prompt = od.Prompt
	o.Tasks = od.Tasks
	o.Path = od.Path
	o.Leader = leader
	o.Workers = workers
	o.Completed = od.Completed
	o.Program = od.Program
	o.Status = status(od.Status)
	return nil
}

type OrchestratorData struct {
	Prompt    string                   `json:"prompt"`
	Tasks     []Task                   `json:"tasks"`
	Path      string                   `json:"path"`
	Leader    *task.TaskData           `json:"leader"`
	Workers   map[string]task.TaskData `json:"workers"`
	Completed map[string]bool          `json:"completed"`
	Program   string                   `json:"program"`
	Status    int                      `json:"status"`
}

// ToOrchestratorData converts an Orchestrator to its serializable form
func (o *Orchestrator) ToOrchestratorData() OrchestratorData {
	var leaderData *task.TaskData
	if o.Leader != nil {
		ld := o.Leader.ToInstanceData()
		leaderData = &ld
	}
	workersData := make(map[string]task.TaskData)
	for k, v := range o.Workers {
		if v != nil {
			workersData[k] = v.ToInstanceData()
		}
	}
	return OrchestratorData{
		Prompt:    o.Prompt,
		Tasks:     o.Tasks,
		Path:      o.Path,
		Leader:    leaderData,
		Workers:   workersData,
		Completed: o.Completed,
		Program:   o.Program,
		Status:    int(o.Status),
	}
}

// FromOrchestratorData creates an Orchestrator from OrchestratorData
func FromOrchestratorData(data OrchestratorData) (*Orchestrator, error) {
	var leader *task.Task
	if data.Leader != nil {
		var err error
		leader, err = task.FromInstanceData(*data.Leader)
		if err != nil {
			return nil, err
		}
	}
	workers := make(map[string]*task.Task)
	for k, v := range data.Workers {
		w, err := task.FromInstanceData(v)
		if err != nil {
			return nil, err
		}
		workers[k] = w
	}
	return &Orchestrator{
		Prompt:    data.Prompt,
		Tasks:     data.Tasks,
		Path:      data.Path,
		Leader:    leader,
		Workers:   workers,
		Completed: data.Completed,
		Program:   data.Program,
		Status:    status(data.Status),
	}, nil
}

// Helper: marshal a single Orchestrator
func orchestratorToData(o *Orchestrator) ([]byte, error) {
	return json.Marshal(o.ToOrchestratorData())
}

// Helper: unmarshal a single Orchestrator
func orchestratorFromData(data []byte) (*Orchestrator, error) {
	var od OrchestratorData
	if err := json.Unmarshal(data, &od); err != nil {
		return nil, err
	}
	return FromOrchestratorData(od)
}

// Helper: marshal a slice of Orchestrators
func orchestratorsToBytes(orchestrators []*Orchestrator) ([]byte, error) {
	allData := make([]OrchestratorData, 0, len(orchestrators))
	for _, o := range orchestrators {
		allData = append(allData, o.ToOrchestratorData())
	}
	return json.Marshal(allData)
}

// Helper: unmarshal a slice of Orchestrators
func orchestratorsFromBytes(data []byte) ([]*Orchestrator, error) {
	var allData []OrchestratorData
	if err := json.Unmarshal(data, &allData); err != nil {
		return nil, err
	}
	result := make([]*Orchestrator, 0, len(allData))
	for _, od := range allData {
		o, err := FromOrchestratorData(od)
		if err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, nil
}

// OrchestratorStorage provides storage functionality for Orchestrator instances
// This is a standalone implementation to avoid import cycles with instance.Storage
type OrchestratorStorage struct {
	state config.InstanceStorage
}

// NewOrchestratorStorage creates a new storage for orchestrators
func NewOrchestratorStorage(state config.InstanceStorage) *OrchestratorStorage {
	return &OrchestratorStorage{
		state: state,
	}
}

// SaveOrchestrator saves an orchestrator to disk
// This handles the single orchestrator case by retrieving all instances,
// updating the relevant one, and saving them all back
func (s *OrchestratorStorage) SaveOrchestrator(orchestrator *Orchestrator) error {
	data, err := orchestratorToData(orchestrator)
	if err != nil {
		return fmt.Errorf("failed to marshal orchestrator: %w", err)
	}

	// For a single orchestrator, we just save it as a JSON array with one element
	wrappedData := []byte(fmt.Sprintf("[%s]", data))
	return s.state.SaveInstances(wrappedData)
}

// LoadOrchestrator loads an orchestrator from disk by its prompt (used as ID)
func (s *OrchestratorStorage) LoadOrchestrator(prompt string) (*Orchestrator, error) {
	allData := s.state.GetInstances()
	if len(allData) == 0 {
		return nil, fmt.Errorf("no orchestrators found")
	}

	// Decode the JSON array
	var rawItems []json.RawMessage
	if err := json.Unmarshal(allData, &rawItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instances: %w", err)
	}

	// Find the orchestrator with the matching prompt
	for _, item := range rawItems {
		orch, err := orchestratorFromData(item)
		if err != nil {
			continue // Skip invalid entries
		}
		if orch.Prompt == prompt {
			return orch, nil
		}
	}

	return nil, fmt.Errorf("orchestrator not found: %s", prompt)
}
