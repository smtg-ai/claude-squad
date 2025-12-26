# Valid Diataxis Tutorial Example

This document describes what constitutes a **valid Tutorial document** according to the Diataxis framework validation rules implemented in the `docs` package.

## Tutorial Document Structure

A valid Tutorial document must include:

### 1. Required Fields

```go
doc := &Document{
    ID:          "unique-tutorial-id",     // Required: Unique identifier
    Type:        Tutorial,                  // Required: Must be "tutorial"
    Title:       "Tutorial Title",         // Required: Non-empty title
    Content:     "...",                    // Required: Must be 100-50000 characters
    Description: "Tutorial description",   // Recommended
    Version:     "1.0.0",                  // Recommended
}
```

### 2. Content Requirements

The Content field must satisfy Tutorial-specific validation rules:

#### **Learning Objectives Section (REQUIRED)**

Must include one of:
- `## Learning Objectives`
- `By the end of this tutorial, you will...`
- `You will learn...`

Example:
```markdown
## Learning Objectives

By the end of this tutorial, you will be able to:
- Understand the basics of X
- Create your first Y
- Deploy Z to production
```

#### **Numbered Steps (REQUIRED)**

Must include step-by-step instructions using:
- Numbered lists: `1.`, `2.`, `3.`
- Step headings: `## Step 1`, `## Step 2`

Example:
```markdown
## Step 1: Installation

First, install the package:

`‌`‌`bash
go get github.com/example/package
`‌`‌`

## Step 2: Configuration

Create a configuration file:

`‌`‌`yaml
config:
  option: value
`‌`‌`
```

#### **Code Examples (RECOMMENDED)**

Should include code blocks with language specification:

```markdown
`‌`‌`go
package main

func main() {
    fmt.Println("Hello World")
}
`‌`‌`
```

#### **Practical Focus (RECOMMENDED)**

Should use action-oriented, learning-focused language:
- "step", "learn", "practice", "objective", "goal"

### 3. Complete Valid Tutorial Example

```go
package main

import (
    "fmt"
    "github.com/seanchatmangpt/claude-squad/docs"
)

func main() {
    tutorial := &docs.Document{
        ID:          "getting-started-tutorial",
        Type:        docs.Tutorial,
        Title:       "Getting Started with Claude Squad",
        Description: "Learn how to set up and use Claude Squad for multi-agent orchestration",
        Content: `# Getting Started with Claude Squad

## Learning Objectives

By the end of this tutorial, you will be able to:
- Install and configure Claude Squad
- Create your first agent
- Execute concurrent tasks
- Monitor agent performance

## Prerequisites

Before starting, ensure you have:
- Go 1.21 or later installed
- Basic understanding of concurrency
- A text editor or IDE

## Step 1: Installation

Install Claude Squad using Go modules:

`‌`‌`bash
# Initialize your project
go mod init myproject

# Install Claude Squad
go get github.com/seanchatmangpt/claude-squad
`‌`‌`

## Step 2: Create Your First Agent

Create a file named `main.go`:

`‌`‌`go
package main

import (
    "fmt"
    "github.com/seanchatmangpt/claude-squad/squad"
)

func main() {
    // Create a new agent
    agent := squad.NewAgent("worker-1")
    fmt.Printf("Agent %s is ready\n", agent.Name())
}
`‌`‌`

## Step 3: Run Your Agent

Execute your program:

`‌`‌`bash
go run main.go
`‌`‌`

You should see output confirming the agent is ready.

## Step 4: Add Task Execution

Extend your agent to execute tasks:

`‌`‌`go
func main() {
    agent := squad.NewAgent("worker-1")

    // Define a task
    task := squad.Task{
        ID:     "task-1",
        Action: "process",
    }

    // Execute the task
    result := agent.Execute(task)
    fmt.Printf("Task completed: %v\n", result)
}
`‌`‌`

## What You've Learned

Congratulations! You now know how to:
- Install Claude Squad in your Go project
- Create and initialize agents
- Define and execute tasks
- Verify agent operations

## Next Steps

Continue your learning journey:
- [How to Deploy Agents](howto-deploy) - Deploy agents to production
- [API Reference](api-reference) - Complete API documentation
- [Understanding Agent Architecture](explain-architecture) - Deep dive into design
`,
        Version: "1.0.0",
        Tags:    []string{"tutorial", "getting-started", "beginner"},
        Metadata: map[string]interface{}{
            "difficulty":     "beginner",
            "estimated_time": "15 minutes",
        },
    }

    // Validate the tutorial
    issues := tutorial.Validate()

    if len(issues) == 0 {
        fmt.Println("✓ Tutorial is valid!")
        fmt.Printf("Status: %s\n", tutorial.ValidationStatus)
    } else {
        fmt.Printf("✗ Found %d validation issues:\n", len(issues))
        for _, issue := range issues {
            fmt.Printf("  [%s] %s: %s\n", issue.Severity, issue.Location, issue.Message)
        }
    }
}
```

## Validation Rules Summary

### Error-Level (Must Fix)

1. **ID must not be empty**
2. **Title must not be empty**
3. **Type must not be empty**
4. **Content must not be empty**
5. **Must have Learning Objectives section** - Include "## Learning Objectives" or "By the end of this tutorial, you will..."
6. **Must have numbered steps** - Use "1.", "2.", "3." or "## Step 1", "## Step 2"
7. **Code blocks must be closed** - All ‌`‌`‌` must have matching closing ‌`‌`‌`

### Warning-Level (Should Fix)

1. **Content length** - Should be 100-50,000 characters
2. **Code blocks should specify language** - Use ‌`‌`‌`go, ‌`‌`‌`bash, etc.
3. **Should include code examples** - Tutorials benefit from practical code
4. **Should use tutorial keywords** - Include words like "step", "learn", "objective"

### Info-Level (Nice to Have)

1. **Description recommended** - Helps with discoverability
2. **Tags recommended** - Improves searchability
3. **Version recommended** - Helps with change tracking

## Testing Your Tutorial

```go
// Test validation
issues := tutorial.Validate()

if tutorial.ValidationStatus == docs.ValidationPassed {
    // Tutorial is valid
} else if tutorial.ValidationStatus == docs.ValidationWarnings {
    // Tutorial has warnings but will work
} else {
    // Tutorial has errors and needs fixes
}

// Calculate quality score
calculator := docs.NewQualityCalculator()
score := calculator.Calculate(tutorial)
fmt.Printf("Quality Score: %.0f/100\n", score)
```

## Common Validation Errors

### Missing Learning Objectives
```
[error] Tutorial must include learning objectives. Add a section like:
'## Learning Objectives' or 'By the end of this tutorial, you will...'
```

**Fix:** Add a Learning Objectives section at the beginning.

### Missing Numbered Steps
```
[error] Tutorial must have numbered steps. Use '1.', '2.', '3.' or
'## Step 1', '## Step 2', etc.
```

**Fix:** Structure your tutorial with clear, numbered steps.

### No Code Examples
```
[warning] Tutorial should include practical code examples in code blocks
(‌`‌`‌`language ... ‌`‌`‌`)
```

**Fix:** Add code blocks with language specification.

## See Also

- `/home/user/claude-squad/docs/example_test.go` - Runnable examples
- `/home/user/claude-squad/docs/diataxis.go` - Core Document types
- `/home/user/claude-squad/docs/validator.go` - Validation rules implementation

## File Locations

- **Document types:** `/home/user/claude-squad/docs/diataxis.go:14-18`
- **Validate() method:** `/home/user/claude-squad/docs/diataxis.go:186-229`
- **Tutorial validation:** `/home/user/claude-squad/docs/validator.go:440-485`
- **Example tutorials:** `/home/user/claude-squad/docs/example_test.go:222-310`
