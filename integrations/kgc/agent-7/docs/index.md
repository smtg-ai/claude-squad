# KGC Knowledge Substrate Documentation

Welcome to the **KGC (Knowledge Graph Compositional) Substrate** documentation. This system provides a deterministic, receipt-backed foundation for multi-agent code generation and verification.

## What is KGC?

The KGC substrate is a **deterministic knowledge management system** designed for multi-agent coordination. Every operation produces cryptographic receipts proving reproducibility, enabling:

- **Deterministic builds**: Same inputs always produce identical outputs
- **Receipt chaining**: Every change is cryptographically verifiable
- **Multi-agent composition**: 10 agents work in parallel without conflicts
- **Cross-repo integration**: Policy-driven validation with seanchatmangpt/unrdf

## Documentation Structure

This documentation follows the [Diataxis framework](https://diataxis.fr/), organizing content by purpose:

### Tutorials (Learning-Oriented)

Step-by-step lessons to get you started:

- [Getting Started with KGC](tutorial/getting_started.md) - Your first KGC program

### How-To Guides (Task-Oriented)

Practical guides for specific tasks:

- [Create a Knowledge Store](how_to/create_knowledge_store.md)
- [Verify Receipts](how_to/verify_receipts.md)
- [Run Multi-Agent Demo](how_to/run_multi_agent_demo.md)

### Reference (Information-Oriented)

Technical specifications and API documentation:

- [Substrate Interfaces](reference/substrate_interfaces.md)
- [API Reference](reference/api.md)
- [CLI Reference](reference/cli.md)

### Explanation (Understanding-Oriented)

Deep dives into concepts and architecture:

- [Why Determinism Matters](explanation/why_determinism.md)
- [Receipt Chaining](explanation/receipt_chaining.md)
- [Composition Laws](explanation/composition_laws.md)

## Quick Start

```bash
# Clone the repository
git clone https://github.com/seanchatmangpt/claude-squad.git
cd claude-squad/integrations/kgc

# Run the end-to-end demo
cd agent-9
go run demo.go

# Verify all receipts
cd ../agent-0
go test -v
```

## Core Concepts

- **KnowledgeStore**: Immutable append-log with hash-stable snapshots
- **Receipt**: Cryptographic proof of execution (before/after hashes + replay script)
- **Reconciler**: Validates multi-agent patches compose without conflicts
- **Determinism**: Same inputs → same outputs, always

## Proof Targets

The KGC substrate provides four formal proof guarantees:

| Proof | Description | Command |
|-------|-------------|---------|
| **P1** | Deterministic substrate build | `make proof-p1` |
| **P2** | Multi-agent patch integrity | `make proof-p2` |
| **P3** | Receipt-chain correctness | `make proof-p3` |
| **P4** | Cross-repo integration contract | `make proof-p4` |

## Agent Architecture

The KGC substrate is built by a **10-agent concurrent swarm**, each specializing in one domain:

- **Agent 0**: Coordinator & Reconciler
- **Agent 1**: Knowledge Store Core
- **Agent 2**: Receipt Chain & Tamper Detection
- **Agent 3**: Policy Pack Bridge (→ unrdf)
- **Agent 4**: Resource Allocation & Capacity
- **Agent 5**: Agent Workspace Isolation (Poka-Yoke)
- **Agent 6**: Task Graph & Routing
- **Agent 7**: Documentation Scaffolding (Diataxis)
- **Agent 8**: Performance Harness
- **Agent 9**: End-to-End Demo

## Next Steps

- **New to KGC?** Start with the [Getting Started Tutorial](tutorial/getting_started.md)
- **Building something?** Check the [How-To Guides](how_to/)
- **Need API details?** See the [Reference Documentation](reference/)
- **Want to understand why?** Read the [Explanations](explanation/)

## License

Part of the [claude-squad](https://github.com/seanchatmangpt/claude-squad) project.

## Contributing

All contributions must follow the deterministic receipt-driven workflow. See [CHARTER](../../contracts/10_AGENT_SWARM_CHARTER.md) for details.
