# Key Customization

This document explains how to customize key bindings in claude-squad.

## Overview

Claude-squad now supports user-configurable key mappings. Instead of being limited to hardcoded key bindings, you can now customize which keys trigger which actions according to your preferences.

## Configuration

Key mappings are configured in your `~/.claude-squad/config.json` file. You can add a `key_mappings` section to customize only the key bindings you want to change. The system will merge your settings with the defaults.

### Full Configuration Example
```json
{
  "default_program": "claude",
  "auto_yes": false,
  "daemon_poll_interval": 1000,
  "branch_prefix": "user/",
  "key_mappings": {
    "up": ["up", "w"],
    "down": ["down", "s"],
    "enter": ["enter", "space"],
    "new": ["n", "a"],
    "kill": ["D", "x"],
    "quit": ["q", "esc"],
    "tab": ["tab", "t"],
    "checkout": ["c", "C"],
    "resume": ["r", "R"],
    "submit": ["p", "P", "shift+p"],
    "prompt": ["N", "P"],
    "help": ["?", "h"]
  }
}
```

### Partial Configuration (Recommended)
You only need to specify the keys you want to customize:
```json
{
  "key_mappings": {
    "checkout": ["c", "C"],
    "resume": ["r", "R"],
    "submit": ["p", "P", "shift+p"]
  }
}
```
All other keys will use their default values.

## Available Actions

The following actions can be customized:

- **up**: Move cursor up in the list
- **down**: Move cursor down in the list
- **enter**: Open/attach to selected instance
- **new**: Create a new instance
- **kill**: Delete selected instance
- **quit**: Exit the application
- **tab**: Switch between preview and diff tabs
- **checkout**: Checkout the branch for selected instance
- **resume**: Resume a paused instance
- **submit**: Push changes from selected instance
- **prompt**: Create new instance with prompt
- **help**: Show help screen
- **shift+up**: Scroll up in preview/diff pane
- **shift+down**: Scroll down in preview/diff pane

## Multiple Key Bindings

Each action can have multiple key bindings. For example:
```json
"up": ["up", "k", "w"]
```
This allows any of `↑`, `k`, or `w` to move the cursor up.

## Default Key Bindings

If you don't specify `key_mappings` in your config, the following defaults are used:

- **up**: `up`, `k`
- **down**: `down`, `j`
- **enter**: `enter`, `o`
- **new**: `n`
- **kill**: `D`
- **quit**: `q`
- **tab**: `tab`
- **checkout**: `c`
- **resume**: `r`
- **submit**: `p`
- **prompt**: `N`
- **help**: `?`
- **shift+up**: `shift+up`
- **shift+down**: `shift+down`

## Examples

### Add Alternative Key Combinations (Partial Config)
Only customize specific actions with alternative keys:
```json
{
  "key_mappings": {
    "checkout": ["c", "C"],
    "resume": ["r", "R"],
    "submit": ["p", "P", "shift+p"]
  }
}
```

### Vim-style Navigation
```json
{
  "key_mappings": {
    "up": ["k"],
    "down": ["j"],
    "enter": ["l"],
    "quit": ["q"]
  }
}
```

### WASD Navigation
```json
{
  "key_mappings": {
    "up": ["w"],
    "down": ["s"],
    "enter": ["d"],
    "new": ["a"]
  }
}
```

### Gaming-style Controls
```json
{
  "key_mappings": {
    "up": ["w"],
    "down": ["s"],
    "enter": ["space"],
    "new": ["e"],
    "kill": ["x"],
    "quit": ["esc"]
  }
}
```

### Mixed Custom and Default Keys
Combine original keys with new alternatives:
```json
{
  "key_mappings": {
    "up": ["up", "k", "w"],
    "down": ["down", "j", "s"],
    "checkout": ["c", "C", "o"],
    "submit": ["p", "P", "shift+p"]
  }
}
```

### Uppercase and Shift-focused Workflow
Use uppercase and shift combinations for main actions:
```json
{
  "key_mappings": {
    "new": ["n", "N"],
    "kill": ["D", "shift+d"],
    "checkout": ["c", "C"],
    "resume": ["r", "R"],
    "submit": ["p", "P", "shift+p"],
    "quit": ["q", "Q"]
  }
}
```

## Notes

- Key mappings are case-sensitive
- Special keys like `shift+up`, `shift+p` are supported
- **Avoid terminal conflicts**: Don't use `ctrl+c` (SIGINT) or `ctrl+d` (EOF) as they conflict with terminal signal handling
- **Recommended alternatives**: Use uppercase letters (`C`, `R`, `P`) or shift combinations (`shift+p`) instead of problematic ctrl keys
- If a key is mapped to multiple actions, only the first match will be used
- Invalid or unknown actions in the config will be ignored
- The application will fall back to default mappings if the config is invalid