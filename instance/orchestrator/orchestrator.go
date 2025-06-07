package orchestrator

import (
	"regexp"
	"strings"
	"sync"

	"claude-squad/instance/task"
	"claude-squad/keys"
)

type status int

const (
	// DefaultStatus is the default status.
	DefaultStatus status = iota
	// Formulating is the status when the orchestrator is coming up with the plan.
	Formulating
	// Planned is the status when the orchestrator has finished planning.
	Planned
	// Executing is the status when the orchestrator is executing the plan.
	Executing
	// Done is the status when the orchestrator has finished executing the plan.
	Done
)

// TODO: Should be able to pick planning/executing models/config
// e.g. Claude 3.5 Haiku for executing, Claude Opus/Sonnet for planning

// Orchestrator manages the orchestration of multiple worker instances to achieve a goal.
type Orchestrator struct {
	// Prompt is the user prompt submitted by the user.
	Prompt string
	// Tasks is the list of tasks devised for the plan.
	Tasks []Task
	// Path is the path the orchestrator is operating in.
	Path string
	// Leader is the instance tied to the orchestrator itself.
	Leader *task.Task
	// Workers is a map of managed workers by this orchestrator.
	Workers   map[string]*task.Task
	Completed map[string]bool
	Program   string

	// Status is the status of the orchestrator.
	Status status

	mu sync.Mutex
}

// Task represents a subdivided work item for a worker.
type Task struct {
	Name   string
	Prompt string
}

// NewOrchestrator creates a new orchestrator with the given prompt and autoyes mode.
func NewOrchestrator(program, prompt string) *Orchestrator {
	return &Orchestrator{
		Prompt:    prompt,
		Path:      ".",
		Workers:   make(map[string]*task.Task),
		Completed: make(map[string]bool),
		Program:   program,
	}
}

// ForumulatePlan devises a plan given a plan.
func (o *Orchestrator) ForumulatePlan() error {
	plannerPrompt := `You are a project orchestrator. Your goal is to implement: \n` + o.Prompt + `\n

    Break this goal down into manageable tasks that can be assigned to worker instances. I'll help you develop a plan, then you can create and manage worker instances to implement specific tasks.

    You have these additional capabilities:
    1. You can create worker instances to implement specific tasks.
    2. You will be notified when a worker instance needs help or completes a task.

    Break this goal down into 2-5 separate distinct tasks that would be appropriate to delegate to different workers. 
    Each task should be independent enough that it can be worked on separately.

    For each task, provide:
    1. A short, descriptive task name (e.g. "Create Login API")
    2. A detailed prompt for the worker that will implement this task

    Respond exactly in the following format, with each task on its own line:
    <TASK-i>
    Task Name | Detailed instructions for the worker to complete this specific task...
    </TASK-i>
    `

	task, err := task.NewTask(task.TaskOptions{
		Title:    "Planning",
		Path:     o.Path,
		Program:  o.Program,
		AutoYes:  true,
		IsWorker: true,
	})
	if err != nil {
		return err
	}

	err = task.SendPrompt(plannerPrompt)
	if err != nil {
		return err
	}

	// Wait for the session to finish before reading plan
	err = task.WaitForCompletion()
	if err != nil {
		return err
	}

	output, err := task.FullOutput()
	if err != nil {
		return err
	}

	tasks := parsePlanOutput(output)

	if len(tasks) == 0 {
		// If no tasks were parsed, fallback to a single task
		o.Tasks = []Task{{Name: "main-task", Prompt: o.Prompt}}
	} else {
		o.Tasks = tasks
	}

	return nil
}

func (o *Orchestrator) StatusText() string {
	switch o.Status {
	case Formulating:
		return "Formulating plan..."
	case Planned:
		return "Plan formulated"
	case Executing:
		return "Executing plan..."
	case Done:
		return "Plan executed"
	}
	panic("unhandled status")
}

func (o *Orchestrator) MenuItems() []keys.KeyName {
	return []keys.KeyName{keys.KeyEnter}
}

// parsePlanOutput parses the output from the planner to extract tasks
func parsePlanOutput(output string) []Task {
	var tasks []Task

	// Regex to find all <TASK-x>...</TASK-x> blocks
	taskBlockRegex := regexp.MustCompile(`(?s)<TASK-\d+>(.*?)</TASK-\d+>`) // (?s) enables dot to match newlines
	matches := taskBlockRegex.FindAllStringSubmatch(output, -1)

	for _, match := range matches {
		content := strings.TrimSpace(match[1])
		// Expect content in the form: taskname | description
		parts := strings.SplitN(content, "|", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			prompt := strings.TrimSpace(parts[1])
			tasks = append(tasks, Task{
				Name:   name,
				Prompt: prompt,
			})
		}
	}

	return tasks
}
