package registry

import (
	"claude-squad/instance/interfaces"
	orchPkg "claude-squad/instance/orchestrator"
	taskPkg "claude-squad/instance/task"
	typesPkg "claude-squad/instance/types"
	"encoding/json"
	"fmt"
)

// MarshalInstanceWithType serializes an instance with a type tag
func MarshalInstanceWithType(inst interfaces.Instance) ([]byte, error) {
	var typeTag string
	switch inst.(type) {
	case *taskPkg.Task:
		typeTag = typesPkg.TypeTask
	case *orchPkg.Orchestrator:
		typeTag = typesPkg.TypeOrchestrator
	default:
		return nil, fmt.Errorf("unknown instance type for serialization")
	}
	data := inst.Serialize()
	return json.Marshal(typesPkg.TaggedInstance{
		Type: typeTag,
		Data: data,
	})
}

// UnmarshalInstanceWithType deserializes a type-tagged instance
func UnmarshalInstanceWithType(tagged typesPkg.TaggedInstance) (interfaces.Instance, error) {
	switch tagged.Type {
	case typesPkg.TypeTask:
		var t taskPkg.Task
		if err := t.Deserialize(tagged.Data); err != nil {
			return nil, err
		}
		return &t, nil
	case typesPkg.TypeOrchestrator:
		var o orchPkg.Orchestrator
		if err := o.Deserialize(tagged.Data); err != nil {
			return nil, err
		}
		return &o, nil
	default:
		return nil, fmt.Errorf("unknown instance type tag: %s", tagged.Type)
	}
}

// MarshalInstanceSlice serializes a slice of Instance as a JSON array of type-tagged objects
func MarshalInstanceSlice(instances []interfaces.Instance) ([]byte, error) {
	var tagged []typesPkg.TaggedInstance
	for _, inst := range instances {
		b, err := MarshalInstanceWithType(inst)
		if err != nil {
			return nil, err
		}
		var tag typesPkg.TaggedInstance
		if err := json.Unmarshal(b, &tag); err != nil {
			return nil, err
		}
		tagged = append(tagged, tag)
	}
	return json.Marshal(tagged)
}

// UnmarshalInstanceSlice deserializes a JSON array of type-tagged objects into a slice of Instance
func UnmarshalInstanceSlice(data []byte) ([]interfaces.Instance, error) {
	var tagged []typesPkg.TaggedInstance
	if err := json.Unmarshal(data, &tagged); err != nil {
		return nil, err
	}
	var result []interfaces.Instance
	for _, tag := range tagged {
		inst, err := UnmarshalInstanceWithType(tag)
		if err != nil {
			return nil, err
		}
		result = append(result, inst)
	}
	return result, nil
}
