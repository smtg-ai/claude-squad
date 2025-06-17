# UX Design: Multi-Project Support for Claude Squad

## Current State vs Final Design

### Current Interface
```
┌─ Instances ─────────────┐┌─────────── Preview ──────────────┐┌──────── Console ──────┐
│ 1. hola               ● ││                                   ││                       │
│    J-2-gabadi/hola      ││   Welcome to Claude Code!         ││   > █                 │
│                         ││                                   ││                       │
│                         ││   /help for help, /status for...  ││                       │
│                         ││                                   ││                       │
└─────────────────────────┘└───────────────────────────────────┘└───────────────────────┘
```

### Final Multi-Project Interface
```
┌─ Projects & Instances ──────────┐┌────────── Preview ──────────┐┌──────── Console ─────┐
│ 📁 claude-squad (current)       ││                              ││                      │
│   └─ 1. feature/console       ● ││   Welcome to Claude Code!    ││   > █                │
│   └─ 2. bugfix/scroll           ││                              ││                      │
│                                 ││   Current project:           ││                      │
│ 📁 my-app                       ││   claude-squad               ││                      │
│   └─ 3. feat/auth             ⏸ ││                              ││                      │
│   └─ 4. refactor/api            ││   Working directory:         ││                      │
│                                 ││   /Users/.../claude-squad    ││                      │
│ 📁 docs-site                    ││                              ││                      │
│   └─ 5. content/update          ││                              ││                      │
│                                 ││                              ││                      │
│ + Add Project                   ││                              ││                      │
└─────────────────────────────────┘└──────────────────────────────┘└──────────────────────┘
```

## Add Project Flow (Trigger: `p`)

### Smart Input Interface
```
┌─ Add Project ─────────────────────────────────┐
│                                               │
│ Path or name:                                 │
│ > █                                           │
│                                               │
│ Examples:                                     │
│ • docs        → searches sibling directories  │
│ • ../my-app   → relative path                 │
│ • /full/path  → absolute path                 │
│ • .           → current directory             │
│                                               │
│ [Enter] Add  [Tab] Autocomplete  [Esc] Cancel │
└───────────────────────────────────────────────┘
```

### Smart Input Logic
1. **Project name only** (e.g., `docs`):
   - Searches sibling directories: `../docs`, `../claude-squad-docs`, etc.
   - Auto-completes with closest match

2. **Relative path** (e.g., `../my-app`):
   - Resolves relative to current project directory

3. **Absolute path** (e.g., `/Users/dev/project`):
   - Uses exact path provided

4. **Current directory** (`.`):
   - Adds the directory where `cs` was executed from

### Real-time Feedback Example
```
┌─ Add Project ─────────────────────────────────┐
│                                               │
│ Path or name:                                 │
│ > docs█                                       │
│                                               │
│ 💡 Found: /Users/dev/claude-squad-docs        │
│                                               │
│ [Enter] Add  [Tab] Autocomplete  [Esc] Cancel │
└───────────────────────────────────────────────┘
```

## Project Context Switcher (Trigger: `ctrl+p`)
```
┌─ Switch Project Context ─────────────────────┐
│                                              │
│ 🎯 claude-squad (current)           3 inst   │
│   my-app                            2 inst   │
│   docs-site                         1 inst   │
│                                              │
│ Type to filter...                            │
│ > █                                          │
│                                              │
│ [↑↓] Navigate [Enter] Switch [Esc] Cancel    │
└──────────────────────────────────────────────┘
```

## Keyboard Shortcuts

```
Multi-Project:
  p          Add project (smart input)
  ctrl+p     Quick project switcher
  
Existing shortcuts remain unchanged:
  n          Create new instance (in current project context)
  N          Create new instance with prompt
  D          Kill selected instance
  ↑/j, ↓/k   Navigate instances/projects
  ↵/o        Attach to selected instance
  ctrl-q     Detach from session
  s          Commit and push branch
  c          Checkout/commit changes
  r          Resume paused session
  tab        Switch between Preview/Diff/Console
  q          Quit application
  ?          Show help (updated to include project commands)
```

## Implementation Strategy

### Core Features (MVP)
- [ ] Hierarchical project/instance view in left sidebar
- [ ] Smart project input (`p` command) with sibling directory scanning
- [ ] Project context switcher (`ctrl+p`)
- [ ] Project configuration storage (`~/.config/claude-squad/projects.yaml`)
- [ ] Update help menu (`?`) to include new project commands

### Enhanced Features (Future)
- [ ] Project collapse/expand functionality
- [ ] Project-specific settings
- [ ] Cross-project instance movement
- [ ] Project health indicators

## Technical Considerations

### Data Structure
```yaml
projects:
  - name: "claude-squad"
    path: "/Users/dev/claude-squad"
    last_accessed: "2024-01-15T10:30:00Z"
    instances: [...]
    
  - name: "my-app"
    path: "/Users/dev/my-app"
    last_accessed: "2024-01-14T15:45:00Z"
    instances: [...]
```

### Configuration Storage
- Global config: `~/.config/claude-squad/projects.yaml`
- Instance state: `~/.config/claude-squad/instances.json`

---

*Generated by Sally, UX Expert 🎨 - Focused on user-centered, accessible multi-project workflows*