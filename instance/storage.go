package instance

import (
	"claude-squad/config"
	"claude-squad/instance/interfaces"
	"claude-squad/registry"
	"fmt"
)

// InstanceStorage is a generic storage abstraction for any type implementing the Instance interface.
type Storage[T interfaces.Instance] struct {
	state    config.InstanceStorage
	ToData   func(T) ([]byte, error)
	FromData func([]byte) (T, error)
	GetTitle func(T) string
}

func NewStorage[T interfaces.Instance](state config.InstanceStorage, toData func(T) ([]byte, error), fromData func([]byte) (T, error), getTitle func(T) string) *Storage[T] {
	return &Storage[T]{
		state:    state,
		ToData:   toData,
		FromData: fromData,
		GetTitle: getTitle,
	}
}

// SaveInstances saves a list of instances to disk.
func (s *Storage[T]) SaveInstances(instances []T) error {
	// Convert []T to []Instance for unified serialization
	var insts []interfaces.Instance
	for _, i := range instances {
		insts = append(insts, i)
	}
	data, err := registry.MarshalInstanceSlice(insts)
	if err != nil {
		return fmt.Errorf("failed to marshal instances: %w", err)
	}
	return s.state.SaveInstances(data)
}

// LoadInstances loads all instances from disk.
func (s *Storage[T]) LoadInstances() ([]T, error) {
	jsonData := s.state.GetInstances()
	insts, err := registry.UnmarshalInstanceSlice(jsonData)
	if err != nil {
		return nil, err
	}
	var result []T
	for _, inst := range insts {
		if t, ok := inst.(T); ok {
			result = append(result, t)
		}
	}
	return result, nil
}

// DeleteInstance removes an instance by title.
func (s *Storage[T]) DeleteInstance(title string) error {
	instances, err := s.LoadInstances()
	if err != nil {
		return err
	}
	var newInstances []T
	found := false
	for _, inst := range instances {
		if s.GetTitle(inst) != title {
			newInstances = append(newInstances, inst)
		} else {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("instance not found: %s", title)
	}
	return s.SaveInstances(newInstances)
}

// UpdateInstance updates an existing instance by title.
func (s *Storage[T]) UpdateInstance(instance T) error {
	instances, err := s.LoadInstances()
	if err != nil {
		return err
	}
	title := s.GetTitle(instance)
	found := false
	for i, inst := range instances {
		if s.GetTitle(inst) == title {
			instances[i] = instance
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("instance not found: %s", title)
	}
	return s.SaveInstances(instances)
}

// DeleteAllInstances removes all instances.
func (s *Storage[T]) DeleteAllInstances() error {
	return s.state.DeleteAllInstances()
}
