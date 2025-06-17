# Template Format Conventions

Templates in the BMAD method use standardized markup for AI processing. These conventions ensure consistent document generation.

## Template Markup Elements

- **{{placeholders}}**: Variables to be replaced with actual content
- **[[LLM: instructions]]**: Internal processing instructions for AI agents (never shown to users)
- **REPEAT** sections: Content blocks that may be repeated as needed
- **^^CONDITION^^** blocks: Conditional content included only if criteria are met
- **@{examples}**: Example content for guidance (never output to users)

## Processing Rules

- Replace all {{placeholders}} with project-specific content
- Execute all [[LLM: instructions]] internally without showing users
- Process conditional and repeat blocks as specified
- Use examples for guidance but never include them in final output
- Present only clean, formatted content to users

## Critical Guidelines

- **NEVER display template markup, LLM instructions, or examples to users**
- Template elements are for AI processing only
- Focus on faithful template execution and clean output
- All template-specific instructions are embedded within templates
