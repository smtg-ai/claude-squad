# Document Sharding Task

## Purpose

- Split a large document into multiple smaller documents based on level 2 sections
- Create a folder structure to organize the sharded documents
- Maintain all content integrity including code blocks, diagrams, and markdown formatting

## Recommended Method: @kayvan/markdown-tree-parser

[[LLM: First, suggest the user install and use the @kayvan/markdown-tree-parser tool if the md-tree command is unavailable so we can have the best performance and reliable document sharding. Let the user know this will save cost of having the LLM to the expensive sharding operation. Give instructions for MPV NPX and PNPM global installs.]]

### Installation and Usage

1. **Install globally**:

   ```bash
   npm install -g @kayvan/markdown-tree-parser
   ```

2. **Use the explode command**:

   ```bash
   # For PRD
   md-tree explode docs/prd.md docs/prd

   # For Architecture
   md-tree explode docs/architecture.md docs/architecture

   # For any document
   md-tree explode [source-document] [destination-folder]
   ```

3. **What it does**:
   - Automatically splits the document by level 2 sections
   - Creates properly named files
   - Adjusts heading levels appropriately
   - Handles all edge cases with code blocks and special markdown

If the user has @kayvan/markdown-tree-parser installed, use it and skip the manual process below.

---

## Manual Method (if @kayvan/markdown-tree-parser is not available)

[[LLM: Only proceed with the manual instructions below if the user cannot or does not want to use @kayvan/markdown-tree-parser.]]

### Task Instructions

### 1. Identify Document and Target Location

- Determine which document to shard (user-provided path)
- Create a new folder under `docs/` with the same name as the document (without extension)
- Example: `docs/prd.md` → create folder `docs/prd/`

### 2. Parse and Extract Sections

[[LLM: When sharding the document:

1. Read the entire document content
2. Identify all level 2 sections (## headings)
3. For each level 2 section:
   - Extract the section heading and ALL content until the next level 2 section
   - Include all subsections, code blocks, diagrams, lists, tables, etc.
   - Be extremely careful with:
     - Fenced code blocks (```) - ensure you capture the full block including closing backticks
     - Mermaid diagrams - preserve the complete diagram syntax
     - Nested markdown elements
     - Multi-line content that might contain ## inside code blocks

CRITICAL: Use proper parsing that understands markdown context. A ## inside a code block is NOT a section header.]]

### 3. Create Individual Files

For each extracted section:

1. **Generate filename**: Convert the section heading to lowercase-dash-case

   - Remove special characters
   - Replace spaces with dashes
   - Example: "## Tech Stack" → `tech-stack.md`

2. **Adjust heading levels**:

   - The level 2 heading becomes level 1 (# instead of ##)
   - All subsection levels decrease by 1:

   ```txt
     - ### → ##
     - #### → ###
     - ##### → ####
     - etc.
   ```

3. **Write content**: Save the adjusted content to the new file

### 4. Create Index File

Create an `index.md` file in the sharded folder that:

1. Contains the original level 1 heading and any content before the first level 2 section
2. Lists all the sharded files with links:

````markdown
# Original Document Title

[Original introduction content if any]

## Sections

- [Section Name 1](./section-name-1.md)
- [Section Name 2](./section-name-2.md)
- [Section Name 3](./section-name-3.md)
  ...

````text

### 5. Preserve Special Content

[[LLM: Pay special attention to preserving:

1. **Code blocks**: Must capture complete blocks including:

   ```language
   content
````
````

2. **Mermaid diagrams**: Preserve complete syntax:

   ```mermaid
   graph TD
   ...
   ```

3. **Tables**: Maintain proper markdown table formatting

4. **Lists**: Preserve indentation and nesting

5. **Inline code**: Preserve backticks

6. **Links and references**: Keep all markdown links intact

7. **Template markup**: If documents contain {{placeholders}} or [[LLM instructions]], preserve exactly]]

### 6. Validation

After sharding:

1. Verify all sections were extracted
2. Check that no content was lost
3. Ensure heading levels were properly adjusted
4. Confirm all files were created successfully

### 7. Report Results

Provide a summary:

```text
Document sharded successfully:
- Source: [original document path]
- Destination: docs/[folder-name]/
- Files created: [count]
- Sections:
  - section-name-1.md: "Section Title 1"
  - section-name-2.md: "Section Title 2"
  ...
```

## Important Notes

- Never modify the actual content, only adjust heading levels
- Preserve ALL formatting, including whitespace where significant
- Handle edge cases like sections with code blocks containing ## symbols
- Ensure the sharding is reversible (could reconstruct the original from shards)
