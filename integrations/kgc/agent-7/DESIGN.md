# Agent 7 DESIGN: Documentation Scaffolding (Diataxis)

## Mission

Create comprehensive Diataxis documentation for the KGC knowledge substrate, providing tutorials, how-to guides, reference documentation, and explanations that enable users to understand and use the system effectively.

---

## Formal Specification

### O (Observable Inputs)

The observable inputs this agent assumes:

```
O = {
    contracts :: {
        10_AGENT_SWARM_CHARTER.md,
        SUBSTRATE_INTERFACES.md
    },
    agent_contexts :: {
        agent-0..agent-9 :: DirectoryStructure
    },
    documentation_requirements :: {
        diataxis_framework :: {
            tutorial :: "learning-oriented",
            how_to :: "task-oriented",
            reference :: "information-oriented",
            explanation :: "understanding-oriented"
        }
    }
}
```

**Typing:**

- `contracts` are read-only markdown files
- `agent_contexts` are directory structures (may be empty initially)
- `diataxis_framework` defines documentation taxonomy

---

### A = μ(O) (Transformation)

The transformation this agent performs:

```
A: O → O' where O' = {
    docs/ :: {
        index.md :: EntryPoint,
        tutorial/ :: {
            getting_started.md :: Tutorial
        },
        how_to/ :: {
            create_knowledge_store.md :: HowToGuide,
            verify_receipts.md :: HowToGuide,
            run_multi_agent_demo.md :: HowToGuide
        },
        reference/ :: {
            substrate_interfaces.md :: ReferenceDoc,
            api.md :: ReferenceDoc,
            cli.md :: ReferenceDoc
        },
        explanation/ :: {
            why_determinism.md :: ExplanationDoc,
            receipt_chaining.md :: ExplanationDoc,
            composition_laws.md :: ExplanationDoc
        }
    },
    build_docs.sh :: ValidationScript,
    DESIGN.md :: DesignDoc,
    RECEIPT.json :: ExecutionProof
}
```

**Transformation Rules:**

1. **Index Creation:** `μ_index(contracts) → index.md`
   - Extract core concepts from charter
   - Generate navigation links to all sections
   - Include quick start guide

2. **Tutorial Generation:** `μ_tutorial(SUBSTRATE_INTERFACES.md) → tutorial/*.md`
   - Create step-by-step walkthrough
   - Include runnable code examples
   - Show incremental progression

3. **How-To Generation:** `μ_howto(agent_contexts) → how_to/*.md`
   - Task-oriented guides for common operations
   - Practical examples with full code
   - Troubleshooting sections

4. **Reference Generation:** `μ_reference(SUBSTRATE_INTERFACES.md) → reference/*.md`
   - Complete API specifications
   - Interface definitions
   - CLI command reference

5. **Explanation Generation:** `μ_explain(composition_laws) → explanation/*.md`
   - Deep dives into architecture
   - Mathematical foundations
   - Design rationale

---

### H (Forbidden States / Guards)

Constraints that must never be violated:

```
H = {
    h1: ∀ file ∈ docs/. file must be valid markdown,
    h2: ∀ link ∈ internal_links. target(link) exists,
    h3: ∀ section ∈ diataxis. |files(section)| ≥ 1,
    h4: ∀ code_example. is_runnable(code_example) ∨ is_clearly_marked_as_pseudo_code(code_example),
    h5: index.md must link to all major sections,
    h6: no circular link dependencies,
    h7: no edits outside agent-7/ directory
}
```

**Guard Enforcement:**

- `h1, h2`: Enforced by `build_docs.sh` validation
- `h3`: Enforced by directory structure requirements
- `h4`: Manual verification during creation
- `h5`: Enforced by index generation logic
- `h6`: Enforced by link validation (DAG check)
- `h7`: Enforced by tranche ownership rules

---

### Π (Proof Targets)

What this agent proves:

```
Π = {
    π1: "All markdown files are well-formed",
    π2: "All internal links resolve correctly",
    π3: "Diataxis structure is complete",
    π4: "build_docs.sh passes without errors"
}
```

**Proof Methods:**

**π1: Markdown Well-Formedness**

```bash
# Proof: All files parse correctly
for file in docs/**/*.md; do
    # Check file is non-empty
    [ -s "$file" ] || exit 1

    # Check starts with heading
    head -n 1 "$file" | grep -q '^#' || exit 1
done
```

**π2: Link Resolution**

```bash
# Proof: All links resolve
grep -r '\[.*\](.*\.md)' docs/ | while read link; do
    target=$(echo "$link" | sed 's/.*(\(.*\))/\1/')
    [ -f "$target" ] || exit 1
done
```

**π3: Diataxis Completeness**

```bash
# Proof: All four categories exist and have content
[ -f docs/tutorial/getting_started.md ] || exit 1
[ $(find docs/how_to -name "*.md" | wc -l) -ge 3 ] || exit 1
[ $(find docs/reference -name "*.md" | wc -l) -ge 3 ] || exit 1
[ $(find docs/explanation -name "*.md" | wc -l) -ge 3 ] || exit 1
```

**π4: Build Script Success**

```bash
# Proof: Validation passes
./build_docs.sh && echo "π4: PASS"
```

---

### Σ (Type Assumptions)

Type specifications for all data structures:

```
type MarkdownFile = {
    path: FilePath,
    content: String,
    links: [Link],
    headings: [Heading]
}

type Link = {
    text: String,
    target: FilePath | URL,
    type: Internal | External
}

type Heading = {
    level: 1..6,
    text: String,
    anchor: String
}

type DiataxisSection = Tutorial | HowTo | Reference | Explanation

type Tutorial = {
    title: String,
    steps: [Step],
    code_examples: [CodeBlock],
    prerequisites: [String]
}

type HowToGuide = {
    title: String,
    problem: String,
    solution: String,
    steps: [Step],
    troubleshooting: [Issue]
}

type ReferenceDoc = {
    title: String,
    interfaces: [InterfaceSpec],
    functions: [FunctionSpec],
    examples: [CodeBlock]
}

type ExplanationDoc = {
    title: String,
    concepts: [Concept],
    rationale: String,
    examples: [Example]
}

type ValidationResult = {
    total_files: Nat,
    total_links: Nat,
    broken_links: Nat,
    errors: Nat,
    passed: Bool
}
```

---

### Λ (Priority Order)

Operations are executed in this strict order:

```
Λ = [
    λ1: Create directory structure,
    λ2: Generate index.md (navigation hub),
    λ3: Generate tutorial documentation,
    λ4: Generate how-to guides,
    λ5: Generate reference documentation,
    λ6: Generate explanation documentation,
    λ7: Create build_docs.sh validation script,
    λ8: Run build_docs.sh to validate,
    λ9: Create DESIGN.md (this file),
    λ10: Create RECEIPT.json with execution proof
]
```

**Rationale:**

- `λ1` before all: Directory structure needed for file creation
- `λ2` early: Index provides overview and guides link structure
- `λ3-λ6` middle: Core documentation content
- `λ7` before `λ8`: Script must exist before running
- `λ8` before `λ9`: Validate content before claiming success
- `λ10` last: Receipt proves all work completed

---

### Q (Invariants Preserved)

Invariants that hold before, during, and after this agent's execution:

```
Q = {
    q1: ∀ file ∈ agent-7/. file ∈ tranche(agent-7),
    q2: ∀ link ∈ docs/. is_internal(link) ⟹ target(link) ∈ agent-7/ ∨ target(link) ∈ contracts/,
    q3: index.md ⟶* all_docs (reachability),
    q4: ∀ code_example. is_deterministic(code_example),
    q5: |diataxis_sections| = 4,
    q6: validation_script is idempotent
}
```

**Invariant Proofs:**

**q1: Tranche Isolation**

```
Proof: All file creations use path prefix /integrations/kgc/agent-7/
Therefore: ∀ file created. file ∈ agent-7/ ✓
```

**q2: Link Locality**

```
Proof: All internal links point to either:
  - docs/ (within agent-7/)
  - contracts/ (shared, read-only)
No links to other agent tranches ✓
```

**q3: Reachability**

```
Proof: Index contains links to all four sections:
  - tutorial/
  - how_to/
  - reference/
  - explanation/
Each section file links to related files
Therefore: index.md ⟶* all_docs ✓
```

**q4: Deterministic Examples**

```
Proof: All code examples either:
  - Use deterministic inputs (fixed seeds, no timestamps)
  - Are clearly marked as conceptual/pseudo-code
Therefore: is_deterministic(code_example) ∨ is_marked_conceptual ✓
```

**q5: Diataxis Completeness**

```
Proof: Directory structure enforces:
  1. tutorial/
  2. how_to/
  3. reference/
  4. explanation/
Therefore: |diataxis_sections| = 4 ✓
```

**q6: Validation Idempotence**

```
Proof: build_docs.sh only reads files, never modifies
Therefore: ∀ n. validate^n() = validate() ✓
```

---

## Implementation Strategy

### Phase 1: Structure (λ1)

```bash
mkdir -p docs/{tutorial,how_to,reference,explanation}
```

### Phase 2: Content Generation (λ2-λ6)

For each document:

1. **Extract Concepts:** Parse charter and interfaces
2. **Apply Template:** Use Diataxis template for category
3. **Generate Examples:** Create runnable code snippets
4. **Cross-Link:** Add navigation to related docs

### Phase 3: Validation (λ7-λ8)

```bash
# Create validation script
cat > build_docs.sh <<'EOF'
#!/bin/bash
# Validation logic...
EOF

# Run validation
./build_docs.sh || exit 1
```

### Phase 4: Proofs (λ9-λ10)

```bash
# Generate DESIGN.md (this file)
# Generate RECEIPT.json with:
#   - InputHash: SHA256(charter + interfaces)
#   - OutputHash: SHA256(all docs/ files)
#   - ReplayScript: recreate_docs.sh
```

---

## Composition Semantics

### CompositionOp

```
"append"
```

Agent 7 produces documentation that **appends** to the overall KGC knowledge base. Documentation does not conflict with code or other artifacts.

### ConflictPolicy

```
"fail_fast"
```

If any validation fails (broken links, missing files), fail immediately and loudly.

### Composition Proof

```
agent-7 ⊕ agent-{0..6,8,9}:
  - Disjoint file sets? ✓ (docs/ vs code/)
  - No circular dependencies? ✓
  - All links to other agents reference contracts/ only? ✓

Therefore: composition succeeds ✓
```

---

## Verification Commands

### Local Verification

```bash
cd /home/user/claude-squad/integrations/kgc/agent-7
./build_docs.sh
```

**Expected Output:**

```
✓ All validations passed!
Documentation is ready for use.
```

### Integration Verification

```bash
# Verify no files outside tranche
find . -type f | grep -v '^./agent-7/' && exit 1 || echo "✓ Tranche isolation"

# Verify all links resolve
./build_docs.sh | grep "Broken links: 0" || exit 1
```

---

## Edge Cases & Error Handling

### Case 1: Contract Files Change

**Problem:** Charter or interfaces updated by other agents

**Solution:** Documentation references via stable paths, not inline content

**Mitigation:** Links to `../../contracts/` remain valid

### Case 2: Circular Link Dependencies

**Problem:** A → B → C → A

**Detection:** Build script detects cycles during link validation

**Resolution:** Fail build, require manual fix

### Case 3: Non-Deterministic Code Examples

**Problem:** Example includes `time.Now()` or `rand.Random()`

**Prevention:** All examples reviewed for determinism

**Marking:** Non-deterministic examples clearly labeled

---

## Success Criteria

Agent 7 succeeds if and only if:

```
✓ All markdown files exist and are well-formed (π1)
✓ All internal links resolve (π2)
✓ Diataxis structure is complete (π3)
✓ build_docs.sh exits with code 0 (π4)
✓ DESIGN.md documents formal specification
✓ RECEIPT.json proves execution
```

---

## Receipt Specification

The final RECEIPT.json will contain:

```json
{
  "execution_id": "<uuid>",
  "agent_id": "agent-7",
  "timestamp": "<unix_nanoseconds>",
  "toolchain_ver": "bash 5.x, markdown, grep, sed",
  "input_hash": "sha256:<hash(charter + interfaces)>",
  "output_hash": "sha256:<hash(all docs/ files + build_docs.sh)>",
  "replay_script": "#!/bin/bash\n# Reproduce agent-7 documentation\n...",
  "composition_op": "append",
  "conflict_policy": "fail_fast",
  "proof_artifacts": {
    "validation_log": "build_docs.sh output",
    "file_count": "11",
    "link_count": "<total_links>",
    "broken_links": "0"
  }
}
```

---

## Conclusion

Agent 7 implements a deterministic, validated Diataxis documentation structure for the KGC substrate. All operations are:

- **Deterministic:** Same inputs → same outputs
- **Validated:** build_docs.sh proves correctness
- **Composable:** Disjoint from other agent tranches
- **Auditable:** RECEIPT.json provides proof

**Status:** Ready for execution

---

## Metadata

- **Agent:** 7
- **Domain:** Documentation / Diataxis
- **Tranche:** `/integrations/kgc/agent-7/`
- **Composition:** `append` with `fail_fast` conflict policy
- **Proof Command:** `cd /home/user/claude-squad/integrations/kgc/agent-7 && ./build_docs.sh`
