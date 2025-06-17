# Create Document from Template Task

## Purpose

- Generate documents from any specified template following embedded instructions from the perspective of the selected agent persona

## Instructions

### 1. Identify Template and Context

- Determine which template to use (user-provided or list available for selection to user)

  - Agent-specific templates are listed in the agent's dependencies under `templates`. For each template listed, consider it a document the agent can create. So if an agent has:

    @{example}
    dependencies:
    templates: - prd-tmpl - architecture-tmpl
    @{/example}

    You would offer to create "PRD" and "Architecture" documents when the user asks what you can help with.

- Gather all relevant inputs, or ask for them, or else rely on user providing necessary details to complete the document
- Understand the document purpose and target audience

### 2. Determine Interaction Mode

Confirm with the user their preferred interaction style:

- **Incremental:** Work through chunks of the document.
- **YOLO Mode:** Draft complete document making reasonable assumptions in one shot. (Can be entered also after starting incremental by just typing /yolo)

### 3. Execute Template

- Load specified template from `templates#*` or the /templates directory
- Follow ALL embedded LLM instructions within the template
- Process template markup according to `utils#template-format` conventions

### 4. Template Processing Rules

#### CRITICAL: Never display template markup, LLM instructions, or examples to users

- Replace all {{placeholders}} with actual content
- Execute all [[LLM: instructions]] internally
- Process `<<REPEAT>>` sections as needed
- Evaluate ^^CONDITION^^ blocks and include only if applicable
- Use @{examples} for guidance but never output them

### 5. Content Generation

- **Incremental Mode**: Present each major section for review before proceeding
- **YOLO Mode**: Generate all sections, then review complete document with user
- Apply any elicitation protocols specified in template
- Incorporate user feedback and iterate as needed

### 6. Validation

If template specifies a checklist:

- Run the appropriate checklist against completed document
- Document completion status for each item
- Address any deficiencies found
- Present validation summary to user

### 7. Final Presentation

- Present clean, formatted content only
- Ensure all sections are complete
- DO NOT truncate or summarize content
- Begin directly with document content (no preamble)
- Include any handoff prompts specified in template

## Important Notes

- Template markup is for AI processing only - never expose to users
