#!/usr/bin/env python3
"""
Enhanced Oxigraph service with W3C RDF/Turtle best practices.
Implements proper ontology design, provenance tracking, and advanced SPARQL patterns.
"""

import json
import logging
from datetime import datetime
from typing import Dict, List, Optional, Any, Set
from dataclasses import dataclass
from pathlib import Path
from flask import Flask, request, jsonify, Response
from pyoxigraph import Store, NamedNode, Literal, Triple, Quad, BlankNode
import threading
import uuid
import hashlib

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# W3C Standard Namespaces (Best Practice #1: Use standard vocabularies)
RDF = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
RDFS = "http://www.w3.org/2000/01/rdf-schema#"
OWL = "http://www.w3.org/2002/07/owl#"
XSD = "http://www.w3.org/2001/XMLSchema#"
DCTERMS = "http://purl.org/dc/terms/"
PROV = "http://www.w3.org/ns/prov#"
FOAF = "http://xmlns.com/foaf/0.1/"

# Custom namespace with proper versioning (Best Practice #2: Version your ontology)
CS_BASE = "http://claude-squad.ai/ontology/"
CS_VERSION = "1.0.0"
CS = f"{CS_BASE}v{CS_VERSION}#"
CS_TASK = f"{CS_BASE}task/"
CS_AGENT = f"{CS_BASE}agent/"
CS_EXECUTION = f"{CS_BASE}execution/"

# Namespace prefix mapping for SPARQL and Turtle (Best Practice #3: Define prefixes)
PREFIXES = {
    "rdf": RDF,
    "rdfs": RDFS,
    "owl": OWL,
    "xsd": XSD,
    "dcterms": DCTERMS,
    "prov": PROV,
    "foaf": FOAF,
    "cs": CS,
}

def get_prefix_declarations() -> str:
    """Generate SPARQL prefix declarations."""
    return "\n".join([f"PREFIX {prefix}: <{uri}>" for prefix, uri in PREFIXES.items()])


class EnhancedOxigraphStore:
    """
    Enhanced Oxigraph store with best practices:
    - Persistent storage
    - Transaction support
    - Turtle export/import
    - Validation
    """

    def __init__(self, storage_path: Optional[str] = None):
        """Initialize store with optional persistent storage."""
        if storage_path:
            # Best Practice #4: Use persistent storage
            storage_dir = Path(storage_path)
            storage_dir.mkdir(parents=True, exist_ok=True)
            self.store = Store(path=str(storage_dir))
            logger.info(f"Initialized persistent store at {storage_path}")
        else:
            self.store = Store()
            logger.info("Initialized in-memory store")

        self.lock = threading.RLock()  # Reentrant lock for nested operations

    def add_triple(self, triple: Triple) -> None:
        """Add a triple with proper locking."""
        with self.lock:
            self.store.add(triple)

    def add_triples(self, triples: List[Triple]) -> None:
        """Batch add triples (Best Practice #5: Use batch operations)."""
        with self.lock:
            for triple in triples:
                self.store.add(triple)

    def remove_triples_by_pattern(self, subject=None, predicate=None, object=None) -> int:
        """Remove triples matching pattern."""
        count = 0
        with self.lock:
            for quad in self.store.quads_for_pattern(subject, predicate, object, None):
                self.store.remove(quad)
                count += 1
        return count

    def query(self, query: str) -> Any:
        """Execute SPARQL query with proper locking."""
        with self.lock:
            return self.store.query(query)

    def export_turtle(self, output_path: str) -> None:
        """Export graph to Turtle format (Best Practice #6: Support standard serializations)."""
        with self.lock:
            with open(output_path, 'wb') as f:
                self.store.dump(f, "text/turtle")
        logger.info(f"Exported graph to {output_path}")

    def import_turtle(self, input_path: str) -> None:
        """Import Turtle file into graph."""
        with self.lock:
            with open(input_path, 'rb') as f:
                self.store.load(f, "text/turtle")
        logger.info(f"Imported graph from {input_path}")

    def get_stats(self) -> Dict[str, int]:
        """Get store statistics."""
        with self.lock:
            # Count triples by querying
            count_query = f"""
            {get_prefix_declarations()}
            SELECT (COUNT(*) as ?count)
            WHERE {{ ?s ?p ?o }}
            """
            result = list(self.store.query(count_query))
            count = int(str(result[0]['count'])) if result else 0

            return {
                'triple_count': count,
                'storage_type': 'persistent' if hasattr(self.store, 'path') else 'in-memory'
            }


# Initialize enhanced store with persistent storage
STORAGE_PATH = "/app/data/oxigraph" if Path("/app/data").exists() else "./data/oxigraph"
store = EnhancedOxigraphStore(STORAGE_PATH)


class OntologyManager:
    """
    Manages the Claude Squad ontology with W3C best practices.
    Best Practice #7: Separate ontology management from data operations.
    """

    @staticmethod
    def initialize_ontology():
        """
        Initialize complete ontology with RDFS and OWL constructs.
        Best Practice #8: Define complete ontology with domain, range, and constraints.
        """
        triples = []

        # Ontology metadata
        ontology_uri = f"{CS_BASE}v{CS_VERSION}"
        triples.extend([
            Triple(NamedNode(ontology_uri), NamedNode(f"{RDF}type"), NamedNode(f"{OWL}Ontology")),
            Triple(NamedNode(ontology_uri), NamedNode(f"{OWL}versionInfo"), Literal(CS_VERSION)),
            Triple(NamedNode(ontology_uri), NamedNode(f"{DCTERMS}title"),
                   Literal("Claude Squad Task Orchestration Ontology")),
            Triple(NamedNode(ontology_uri), NamedNode(f"{DCTERMS}description"),
                   Literal("Ontology for managing AI agent task orchestration with dependencies and provenance")),
            Triple(NamedNode(ontology_uri), NamedNode(f"{DCTERMS}created"),
                   Literal(datetime.utcnow().isoformat(), datatype=NamedNode(f"{XSD}dateTime"))),
        ])

        # Class definitions with RDFS labels and comments
        classes = {
            f"{CS}Task": ("Task", "A unit of work to be executed by an AI agent"),
            f"{CS}Agent": ("Agent", "An AI agent capable of executing tasks"),
            f"{CS}Execution": ("Execution", "A specific execution instance of a task"),
            f"{CS}Result": ("Result", "The result of a task execution"),
            f"{CS}Workflow": ("Workflow", "A collection of related tasks"),
        }

        for class_uri, (label, comment) in classes.items():
            triples.extend([
                Triple(NamedNode(class_uri), NamedNode(f"{RDF}type"), NamedNode(f"{OWL}Class")),
                Triple(NamedNode(class_uri), NamedNode(f"{RDFS}label"), Literal(label, language="en")),
                Triple(NamedNode(class_uri), NamedNode(f"{RDFS}comment"), Literal(comment, language="en")),
            ])

        # Property definitions with domain and range (Best Practice #9)
        properties = {
            # Object properties
            f"{CS}dependsOn": {
                "type": f"{OWL}ObjectProperty",
                "label": "depends on",
                "comment": "Indicates that a task depends on another task",
                "domain": f"{CS}Task",
                "range": f"{CS}Task",
                "characteristics": [f"{OWL}TransitiveProperty"]
            },
            f"{CS}assignedTo": {
                "type": f"{OWL}ObjectProperty",
                "label": "assigned to",
                "comment": "Links a task to the agent assigned to execute it",
                "domain": f"{CS}Task",
                "range": f"{CS}Agent",
            },
            f"{CS}hasExecution": {
                "type": f"{OWL}ObjectProperty",
                "label": "has execution",
                "comment": "Links a task to its execution instance",
                "domain": f"{CS}Task",
                "range": f"{CS}Execution",
            },
            f"{CS}hasResult": {
                "type": f"{OWL}ObjectProperty",
                "label": "has result",
                "comment": "Links an execution to its result",
                "domain": f"{CS}Execution",
                "range": f"{CS}Result",
            },
            f"{CS}partOfWorkflow": {
                "type": f"{OWL}ObjectProperty",
                "label": "part of workflow",
                "comment": "Indicates a task is part of a workflow",
                "domain": f"{CS}Task",
                "range": f"{CS}Workflow",
            },

            # Datatype properties
            f"{CS}hasDescription": {
                "type": f"{OWL}DatatypeProperty",
                "label": "has description",
                "comment": "Textual description of a task",
                "domain": f"{CS}Task",
                "range": f"{XSD}string",
            },
            f"{CS}hasStatus": {
                "type": f"{OWL}DatatypeProperty",
                "label": "has status",
                "comment": "Current status of a task (pending, running, completed, failed)",
                "domain": f"{CS}Task",
                "range": f"{XSD}string",
            },
            f"{CS}hasPriority": {
                "type": f"{OWL}DatatypeProperty",
                "label": "has priority",
                "comment": "Priority level of a task (0-10)",
                "domain": f"{CS}Task",
                "range": f"{XSD}integer",
            },
            f"{CS}createdAt": {
                "type": f"{OWL}DatatypeProperty",
                "label": "created at",
                "comment": "Timestamp when the task was created",
                "domain": f"{CS}Task",
                "range": f"{XSD}dateTime",
            },
            f"{CS}startedAt": {
                "type": f"{OWL}DatatypeProperty",
                "label": "started at",
                "comment": "Timestamp when the task execution started",
                "domain": f"{CS}Execution",
                "range": f"{XSD}dateTime",
            },
            f"{CS}completedAt": {
                "type": f"{OWL}DatatypeProperty",
                "label": "completed at",
                "comment": "Timestamp when the task execution completed",
                "domain": f"{CS}Execution",
                "range": f"{XSD}dateTime",
            },
            f"{CS}hasDuration": {
                "type": f"{OWL}DatatypeProperty",
                "label": "has duration",
                "comment": "Duration of task execution in seconds",
                "domain": f"{CS}Execution",
                "range": f"{XSD}decimal",
            },
        }

        for prop_uri, prop_def in properties.items():
            triples.append(Triple(NamedNode(prop_uri), NamedNode(f"{RDF}type"),
                                NamedNode(prop_def["type"])))
            triples.append(Triple(NamedNode(prop_uri), NamedNode(f"{RDFS}label"),
                                Literal(prop_def["label"], language="en")))
            triples.append(Triple(NamedNode(prop_uri), NamedNode(f"{RDFS}comment"),
                                Literal(prop_def["comment"], language="en")))

            if "domain" in prop_def:
                triples.append(Triple(NamedNode(prop_uri), NamedNode(f"{RDFS}domain"),
                                    NamedNode(prop_def["domain"])))
            if "range" in prop_def:
                triples.append(Triple(NamedNode(prop_uri), NamedNode(f"{RDFS}range"),
                                    NamedNode(prop_def["range"])))

            # Add OWL characteristics (Best Practice #10: Use OWL properties)
            if "characteristics" in prop_def:
                for char in prop_def["characteristics"]:
                    triples.append(Triple(NamedNode(prop_uri), NamedNode(f"{RDF}type"),
                                        NamedNode(char)))

        # Add status individuals (Best Practice #11: Define enumerations)
        statuses = ["pending", "running", "completed", "failed", "cancelled"]
        for status in statuses:
            status_uri = f"{CS}Status{status.capitalize()}"
            triples.extend([
                Triple(NamedNode(status_uri), NamedNode(f"{RDF}type"), NamedNode(f"{CS}TaskStatus")),
                Triple(NamedNode(status_uri), NamedNode(f"{RDFS}label"), Literal(status, language="en")),
            ])

        # Batch add all ontology triples
        store.add_triples(triples)
        logger.info(f"Initialized ontology with {len(triples)} triples")


class ProvenanceTracker:
    """
    Tracks provenance using W3C PROV-O ontology.
    Best Practice #12: Use PROV-O for provenance tracking.
    """

    @staticmethod
    def record_activity(activity_id: str, activity_type: str,
                       agent_id: Optional[str] = None,
                       used_entities: Optional[List[str]] = None,
                       generated_entities: Optional[List[str]] = None,
                       started_at: Optional[str] = None,
                       ended_at: Optional[str] = None) -> List[Triple]:
        """Record a provenance activity."""
        activity_uri = f"{CS_EXECUTION}{activity_id}"
        triples = [
            Triple(NamedNode(activity_uri), NamedNode(f"{RDF}type"), NamedNode(f"{PROV}Activity")),
            Triple(NamedNode(activity_uri), NamedNode(f"{RDF}type"), NamedNode(activity_type)),
        ]

        if agent_id:
            agent_uri = f"{CS_AGENT}{agent_id}"
            triples.extend([
                Triple(NamedNode(agent_uri), NamedNode(f"{RDF}type"), NamedNode(f"{PROV}Agent")),
                Triple(NamedNode(activity_uri), NamedNode(f"{PROV}wasAssociatedWith"),
                       NamedNode(agent_uri)),
            ])

        if used_entities:
            for entity_id in used_entities:
                entity_uri = f"{CS_TASK}{entity_id}"
                triples.append(Triple(NamedNode(activity_uri), NamedNode(f"{PROV}used"),
                                    NamedNode(entity_uri)))

        if generated_entities:
            for entity_id in generated_entities:
                entity_uri = f"{CS_TASK}{entity_id}"
                triples.extend([
                    Triple(NamedNode(entity_uri), NamedNode(f"{PROV}wasGeneratedBy"),
                           NamedNode(activity_uri)),
                ])

        if started_at:
            triples.append(Triple(NamedNode(activity_uri), NamedNode(f"{PROV}startedAtTime"),
                                Literal(started_at, datatype=NamedNode(f"{XSD}dateTime"))))

        if ended_at:
            triples.append(Triple(NamedNode(activity_uri), NamedNode(f"{PROV}endedAtTime"),
                                Literal(ended_at, datatype=NamedNode(f"{XSD}dateTime"))))

        return triples


class EnhancedOxigraphOrchestrator:
    """
    Enhanced orchestrator with W3C best practices.
    """

    def __init__(self):
        self.max_concurrent_agents = 10
        OntologyManager.initialize_ontology()

    def create_task(self, task_id: str, description: str, priority: int = 5,
                   agent_id: Optional[str] = None,
                   dependencies: Optional[List[str]] = None,
                   metadata: Optional[Dict] = None,
                   workflow_id: Optional[str] = None) -> str:
        """
        Create a task with full provenance tracking.
        Best Practice #13: Comprehensive metadata and provenance.
        """
        task_uri = f"{CS_TASK}{task_id}"
        created_at = datetime.utcnow().isoformat()

        triples = [
            # Basic task information
            Triple(NamedNode(task_uri), NamedNode(f"{RDF}type"), NamedNode(f"{CS}Task")),
            Triple(NamedNode(task_uri), NamedNode(f"{CS}hasDescription"), Literal(description)),
            Triple(NamedNode(task_uri), NamedNode(f"{CS}hasStatus"), Literal("pending")),
            Triple(NamedNode(task_uri), NamedNode(f"{CS}hasPriority"),
                   Literal(str(priority), datatype=NamedNode(f"{XSD}integer"))),
            Triple(NamedNode(task_uri), NamedNode(f"{CS}createdAt"),
                   Literal(created_at, datatype=NamedNode(f"{XSD}dateTime"))),

            # Dublin Core metadata
            Triple(NamedNode(task_uri), NamedNode(f"{DCTERMS}identifier"), Literal(task_id)),
            Triple(NamedNode(task_uri), NamedNode(f"{DCTERMS}created"),
                   Literal(created_at, datatype=NamedNode(f"{XSD}dateTime"))),
        ]

        # Agent assignment
        if agent_id:
            agent_uri = f"{CS_AGENT}{agent_id}"
            triples.extend([
                Triple(NamedNode(agent_uri), NamedNode(f"{RDF}type"), NamedNode(f"{CS}Agent")),
                Triple(NamedNode(agent_uri), NamedNode(f"{RDF}type"), NamedNode(f"{PROV}Agent")),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}assignedTo"), NamedNode(agent_uri)),
            ])

        # Dependencies
        if dependencies:
            for dep_id in dependencies:
                dep_uri = f"{CS_TASK}{dep_id}"
                triples.append(Triple(NamedNode(task_uri), NamedNode(f"{CS}dependsOn"),
                                    NamedNode(dep_uri)))

        # Workflow membership
        if workflow_id:
            workflow_uri = f"{CS_BASE}workflow/{workflow_id}"
            triples.extend([
                Triple(NamedNode(workflow_uri), NamedNode(f"{RDF}type"), NamedNode(f"{CS}Workflow")),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}partOfWorkflow"), NamedNode(workflow_uri)),
            ])

        # Metadata as structured data (Best Practice #14: Avoid JSON literals when possible)
        if metadata:
            for key, value in metadata.items():
                # Create a proper URI for the metadata predicate
                meta_pred = f"{CS}meta_{key}"
                triples.append(Triple(NamedNode(task_uri), NamedNode(meta_pred), Literal(str(value))))

        # Provenance: Record task creation
        creation_activity_id = f"create_{task_id}_{int(datetime.utcnow().timestamp())}"
        prov_triples = ProvenanceTracker.record_activity(
            activity_id=creation_activity_id,
            activity_type=f"{CS}TaskCreation",
            agent_id="system",
            generated_entities=[task_id],
            ended_at=created_at
        )
        triples.extend(prov_triples)

        store.add_triples(triples)
        logger.info(f"Created task {task_id} with {len(triples)} triples")
        return task_uri

    def update_task_status(self, task_id: str, status: str,
                          result: Optional[str] = None,
                          agent_id: Optional[str] = None) -> bool:
        """
        Update task status with provenance tracking.
        Best Practice #15: Track all changes with provenance.
        """
        task_uri = f"{CS_TASK}{task_id}"
        now = datetime.utcnow().isoformat()

        # Remove old status
        store.remove_triples_by_pattern(NamedNode(task_uri), NamedNode(f"{CS}hasStatus"), None)

        triples = [
            # New status
            Triple(NamedNode(task_uri), NamedNode(f"{CS}hasStatus"), Literal(status)),
            Triple(NamedNode(task_uri), NamedNode(f"{DCTERMS}modified"),
                   Literal(now, datatype=NamedNode(f"{XSD}dateTime"))),
        ]

        # Create execution instance for running/completed states
        if status in ["running", "completed", "failed"]:
            execution_id = f"{task_id}_exec_{int(datetime.utcnow().timestamp())}"
            execution_uri = f"{CS_EXECUTION}{execution_id}"

            exec_triples = [
                Triple(NamedNode(execution_uri), NamedNode(f"{RDF}type"), NamedNode(f"{CS}Execution")),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}hasExecution"), NamedNode(execution_uri)),
            ]

            if status == "running":
                exec_triples.append(Triple(NamedNode(execution_uri), NamedNode(f"{CS}startedAt"),
                                         Literal(now, datatype=NamedNode(f"{XSD}dateTime"))))
            elif status in ["completed", "failed"]:
                exec_triples.append(Triple(NamedNode(execution_uri), NamedNode(f"{CS}completedAt"),
                                         Literal(now, datatype=NamedNode(f"{XSD}dateTime"))))

            # Add result
            if result:
                exec_triples.append(Triple(NamedNode(execution_uri), NamedNode(f"{CS}hasResult"),
                                         Literal(result)))

            # Provenance
            prov_triples = ProvenanceTracker.record_activity(
                activity_id=execution_id,
                activity_type=f"{CS}TaskExecution",
                agent_id=agent_id or "system",
                used_entities=[task_id],
                started_at=now if status == "running" else None,
                ended_at=now if status in ["completed", "failed"] else None
            )

            triples.extend(exec_triples)
            triples.extend(prov_triples)

        store.add_triples(triples)
        logger.info(f"Updated task {task_id} to status {status}")
        return True

    def get_ready_tasks(self, limit: int = 10) -> List[Dict]:
        """
        Get ready tasks with optimized SPARQL query.
        Best Practice #16: Use OPTIONAL for optional properties.
        """
        query = f"""
        {get_prefix_declarations()}

        SELECT ?task ?description ?priority ?createdAt ?workflow
        WHERE {{
            ?task rdf:type cs:Task ;
                  cs:hasStatus "pending" ;
                  cs:hasDescription ?description ;
                  cs:hasPriority ?priority ;
                  cs:createdAt ?createdAt .

            # Optional workflow membership
            OPTIONAL {{ ?task cs:partOfWorkflow ?workflow }}

            # Filter: No incomplete dependencies
            FILTER NOT EXISTS {{
                ?task cs:dependsOn ?dep .
                ?dep cs:hasStatus ?depStatus .
                FILTER(?depStatus != "completed")
            }}
        }}
        ORDER BY DESC(?priority) ?createdAt
        LIMIT {limit}
        """

        results = []
        for solution in store.query(query):
            task_uri = str(solution['task'])
            task_id = task_uri.split('/')[-1]
            results.append({
                'id': task_id,
                'uri': task_uri,
                'description': str(solution['description']),
                'priority': int(str(solution['priority'])),
                'created_at': str(solution['createdAt']),
                'workflow': str(solution['workflow']) if solution.get('workflow') else None,
            })

        logger.info(f"Found {len(results)} ready tasks")
        return results

    def get_task_analytics(self) -> Dict[str, Any]:
        """
        Enhanced analytics with duration and performance metrics.
        Best Practice #17: Provide comprehensive analytics.
        """
        query = f"""
        {get_prefix_declarations()}

        SELECT ?status (COUNT(?task) as ?count)
               (AVG(?duration) as ?avgDuration)
        WHERE {{
            ?task rdf:type cs:Task ;
                  cs:hasStatus ?status .

            OPTIONAL {{
                ?task cs:hasExecution ?exec .
                ?exec cs:hasDuration ?duration .
            }}
        }}
        GROUP BY ?status
        """

        analytics = {
            'status_counts': {},
            'total_tasks': 0,
            'running_count': 0,
            'max_concurrent': self.max_concurrent_agents,
            'avg_duration_by_status': {},
        }

        for solution in store.query(query):
            status = str(solution['status'])
            count = int(str(solution['count']))
            analytics['status_counts'][status] = count
            analytics['total_tasks'] += count

            if status == 'running':
                analytics['running_count'] = count

            if solution.get('avgDuration'):
                analytics['avg_duration_by_status'][status] = float(str(solution['avgDuration']))

        analytics['available_slots'] = max(0, self.max_concurrent_agents - analytics['running_count'])
        analytics['utilization'] = (analytics['running_count'] / self.max_concurrent_agents * 100)

        return analytics

    def export_to_turtle(self, output_path: str) -> None:
        """Export entire graph to Turtle format."""
        store.export_turtle(output_path)

    def get_task_provenance(self, task_id: str) -> List[Dict]:
        """
        Get complete provenance chain for a task.
        Best Practice #18: Provide provenance queries.
        """
        query = f"""
        {get_prefix_declarations()}

        SELECT ?activity ?activityType ?agent ?startTime ?endTime
        WHERE {{
            {{
                ?activity prov:used cs-task:{task_id} .
            }} UNION {{
                cs-task:{task_id} prov:wasGeneratedBy ?activity .
            }}

            ?activity rdf:type ?activityType .

            OPTIONAL {{ ?activity prov:wasAssociatedWith ?agent }}
            OPTIONAL {{ ?activity prov:startedAtTime ?startTime }}
            OPTIONAL {{ ?activity prov:endedAtTime ?endTime }}
        }}
        ORDER BY ?startTime
        """

        results = []
        for solution in store.query(query):
            results.append({
                'activity': str(solution['activity']),
                'type': str(solution['activityType']),
                'agent': str(solution['agent']) if solution.get('agent') else None,
                'started': str(solution['startTime']) if solution.get('startTime') else None,
                'ended': str(solution['endTime']) if solution.get('endTime') else None,
            })

        return results


# Global orchestrator instance
orchestrator = EnhancedOxigraphOrchestrator()


# Enhanced REST API Endpoints

@app.route('/health', methods=['GET'])
def health():
    """Health check with store statistics."""
    stats = store.get_stats()
    return jsonify({
        'status': 'healthy',
        'service': 'oxigraph-orchestrator-enhanced',
        'version': CS_VERSION,
        'store_stats': stats,
    })


@app.route('/tasks', methods=['POST'])
def create_task():
    """Create a new task with full metadata."""
    data = request.json

    task_id = data.get('id', str(uuid.uuid4()))
    task_uri = orchestrator.create_task(
        task_id=task_id,
        description=data['description'],
        priority=data.get('priority', 5),
        agent_id=data.get('agent_id'),
        dependencies=data.get('dependencies', []),
        metadata=data.get('metadata'),
        workflow_id=data.get('workflow_id'),
    )

    return jsonify({'task_id': task_id, 'task_uri': task_uri}), 201


@app.route('/tasks/<task_id>/status', methods=['PUT'])
def update_task_status_endpoint(task_id):
    """Update task status with provenance."""
    data = request.json
    success = orchestrator.update_task_status(
        task_id=task_id,
        status=data['status'],
        result=data.get('result'),
        agent_id=data.get('agent_id')
    )
    return jsonify({'success': success})


@app.route('/tasks/ready', methods=['GET'])
def get_ready_tasks():
    """Get tasks ready for execution."""
    limit = request.args.get('limit', 10, type=int)
    tasks = orchestrator.get_ready_tasks(limit)
    return jsonify({'tasks': tasks, 'count': len(tasks)})


@app.route('/analytics', methods=['GET'])
def get_analytics():
    """Get comprehensive analytics."""
    analytics = orchestrator.get_task_analytics()
    return jsonify(analytics)


@app.route('/tasks/<task_id>/provenance', methods=['GET'])
def get_task_provenance(task_id):
    """Get task provenance chain."""
    provenance = orchestrator.get_task_provenance(task_id)
    return jsonify({'provenance': provenance, 'count': len(provenance)})


@app.route('/export/turtle', methods=['GET'])
def export_turtle():
    """Export graph to Turtle format."""
    output_path = "/tmp/oxigraph_export.ttl"
    orchestrator.export_to_turtle(output_path)

    with open(output_path, 'r') as f:
        turtle_content = f.read()

    return Response(turtle_content, mimetype='text/turtle',
                   headers={'Content-Disposition': 'attachment; filename=graph.ttl'})


@app.route('/ontology', methods=['GET'])
def get_ontology():
    """Get ontology documentation."""
    return jsonify({
        'ontology_uri': f"{CS_BASE}v{CS_VERSION}",
        'version': CS_VERSION,
        'namespaces': PREFIXES,
        'description': 'Claude Squad Task Orchestration Ontology',
    })


if __name__ == '__main__':
    logger.info("Starting Enhanced Oxigraph Orchestrator Service")
    logger.info(f"Version: {CS_VERSION}")
    logger.info(f"Max concurrent agents: {orchestrator.max_concurrent_agents}")
    logger.info(f"Storage: {STORAGE_PATH}")
    app.run(host='0.0.0.0', port=5000, threaded=True)
