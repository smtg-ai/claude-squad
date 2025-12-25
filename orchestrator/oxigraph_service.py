#!/usr/bin/env python3
"""
Oxigraph-based knowledge graph service for Claude Squad agent orchestration.
This service provides semantic task tracking, dependency management, and analytics.
"""

import json
import logging
from datetime import datetime
from typing import Dict, List, Optional, Any
from dataclasses import dataclass, asdict
from flask import Flask, request, jsonify
from pyoxigraph import Store, NamedNode, Literal, Triple
import threading
import uuid

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Initialize Oxigraph store
store = Store()
store_lock = threading.Lock()

# Namespaces
CS = "http://claude-squad.ai/ontology#"
RDF = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
RDFS = "http://www.w3.org/2000/01/rdf-schema#"
XSD = "http://www.w3.org/2001/XMLSchema#"


@dataclass
class AgentTask:
    """Represents a task for an AI agent."""
    id: str
    agent_id: str
    description: str
    status: str  # pending, running, completed, failed
    priority: int
    dependencies: List[str]
    created_at: str
    started_at: Optional[str] = None
    completed_at: Optional[str] = None
    result: Optional[str] = None
    metadata: Optional[Dict[str, Any]] = None


class OxigraphOrchestrator:
    """Advanced orchestrator using Oxigraph for semantic task management."""

    def __init__(self):
        self.max_concurrent_agents = 10
        self._initialize_ontology()

    def _initialize_ontology(self):
        """Initialize the knowledge graph ontology."""
        with store_lock:
            # Define classes
            triples = [
                # Classes
                Triple(NamedNode(f"{CS}Task"), NamedNode(f"{RDF}type"), NamedNode(f"{RDFS}Class")),
                Triple(NamedNode(f"{CS}Agent"), NamedNode(f"{RDF}type"), NamedNode(f"{RDFS}Class")),
                Triple(NamedNode(f"{CS}Dependency"), NamedNode(f"{RDF}type"), NamedNode(f"{RDFS}Class")),
                Triple(NamedNode(f"{CS}Result"), NamedNode(f"{RDF}type"), NamedNode(f"{RDFS}Class")),

                # Properties
                Triple(NamedNode(f"{CS}hasDescription"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}hasStatus"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}hasPriority"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}dependsOn"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}assignedTo"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}hasResult"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}createdAt"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}startedAt"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
                Triple(NamedNode(f"{CS}completedAt"), NamedNode(f"{RDF}type"), NamedNode(f"{RDF}Property")),
            ]

            for triple in triples:
                store.add(triple)

    def create_task(self, task: AgentTask) -> str:
        """Create a new task in the knowledge graph."""
        task_uri = f"{CS}task/{task.id}"

        with store_lock:
            triples = [
                Triple(NamedNode(task_uri), NamedNode(f"{RDF}type"), NamedNode(f"{CS}Task")),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}hasDescription"),
                       Literal(task.description)),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}hasStatus"),
                       Literal(task.status)),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}hasPriority"),
                       Literal(str(task.priority), datatype=NamedNode(f"{XSD}integer"))),
                Triple(NamedNode(task_uri), NamedNode(f"{CS}createdAt"),
                       Literal(task.created_at, datatype=NamedNode(f"{XSD}dateTime"))),
            ]

            # Add agent assignment
            if task.agent_id:
                agent_uri = f"{CS}agent/{task.agent_id}"
                triples.append(
                    Triple(NamedNode(task_uri), NamedNode(f"{CS}assignedTo"),
                           NamedNode(agent_uri))
                )

            # Add dependencies
            for dep_id in task.dependencies:
                dep_uri = f"{CS}task/{dep_id}"
                triples.append(
                    Triple(NamedNode(task_uri), NamedNode(f"{CS}dependsOn"),
                           NamedNode(dep_uri))
                )

            # Add metadata as JSON literal
            if task.metadata:
                triples.append(
                    Triple(NamedNode(task_uri), NamedNode(f"{CS}hasMetadata"),
                           Literal(json.dumps(task.metadata)))
                )

            for triple in triples:
                store.add(triple)

        logger.info(f"Created task {task.id} with status {task.status}")
        return task_uri

    def update_task_status(self, task_id: str, status: str,
                          result: Optional[str] = None) -> bool:
        """Update task status and optionally add result."""
        task_uri = f"{CS}task/{task_id}"

        with store_lock:
            # Remove old status
            for triple in store.quads_for_pattern(
                NamedNode(task_uri), NamedNode(f"{CS}hasStatus"), None, None
            ):
                store.remove(triple)

            # Add new status
            store.add(Triple(NamedNode(task_uri), NamedNode(f"{CS}hasStatus"),
                           Literal(status)))

            # Add timestamp based on status
            now = datetime.utcnow().isoformat()
            if status == "running":
                store.add(Triple(NamedNode(task_uri), NamedNode(f"{CS}startedAt"),
                               Literal(now, datatype=NamedNode(f"{XSD}dateTime"))))
            elif status in ["completed", "failed"]:
                store.add(Triple(NamedNode(task_uri), NamedNode(f"{CS}completedAt"),
                               Literal(now, datatype=NamedNode(f"{XSD}dateTime"))))

            # Add result if provided
            if result:
                store.add(Triple(NamedNode(task_uri), NamedNode(f"{CS}hasResult"),
                               Literal(result)))

        logger.info(f"Updated task {task_id} status to {status}")
        return True

    def get_ready_tasks(self, limit: int = 10) -> List[Dict]:
        """Get tasks that are ready to run (all dependencies completed)."""
        query = f"""
        PREFIX cs: <{CS}>
        PREFIX rdf: <{RDF}>

        SELECT ?task ?description ?priority
        WHERE {{
            ?task rdf:type cs:Task ;
                  cs:hasStatus "pending" ;
                  cs:hasDescription ?description ;
                  cs:hasPriority ?priority .

            # Filter out tasks with incomplete dependencies
            FILTER NOT EXISTS {{
                ?task cs:dependsOn ?dep .
                ?dep cs:hasStatus ?depStatus .
                FILTER(?depStatus != "completed")
            }}
        }}
        ORDER BY DESC(?priority)
        LIMIT {limit}
        """

        with store_lock:
            results = []
            for solution in store.query(query):
                task_uri = str(solution['task'])
                task_id = task_uri.split('/')[-1]
                results.append({
                    'id': task_id,
                    'uri': task_uri,
                    'description': str(solution['description']),
                    'priority': int(str(solution['priority']))
                })

        logger.info(f"Found {len(results)} ready tasks")
        return results

    def get_running_tasks(self) -> List[str]:
        """Get IDs of currently running tasks."""
        query = f"""
        PREFIX cs: <{CS}>
        PREFIX rdf: <{RDF}>

        SELECT ?task
        WHERE {{
            ?task rdf:type cs:Task ;
                  cs:hasStatus "running" .
        }}
        """

        with store_lock:
            results = []
            for solution in store.query(query):
                task_uri = str(solution['task'])
                task_id = task_uri.split('/')[-1]
                results.append(task_id)

        return results

    def get_task_analytics(self) -> Dict[str, Any]:
        """Get comprehensive task analytics."""
        query = f"""
        PREFIX cs: <{CS}>
        PREFIX rdf: <{RDF}>

        SELECT ?status (COUNT(?task) as ?count)
        WHERE {{
            ?task rdf:type cs:Task ;
                  cs:hasStatus ?status .
        }}
        GROUP BY ?status
        """

        with store_lock:
            analytics = {
                'status_counts': {},
                'total_tasks': 0,
                'running_count': 0,
                'max_concurrent': self.max_concurrent_agents
            }

            for solution in store.query(query):
                status = str(solution['status'])
                count = int(str(solution['count']))
                analytics['status_counts'][status] = count
                analytics['total_tasks'] += count
                if status == 'running':
                    analytics['running_count'] = count

            analytics['available_slots'] = max(
                0,
                self.max_concurrent_agents - analytics['running_count']
            )

        return analytics

    def get_task_chain(self, task_id: str) -> List[Dict]:
        """Get the entire dependency chain for a task."""
        query = f"""
        PREFIX cs: <{CS}>

        SELECT ?dep ?description ?status
        WHERE {{
            <{CS}task/{task_id}> cs:dependsOn* ?dep .
            ?dep cs:hasDescription ?description ;
                 cs:hasStatus ?status .
        }}
        """

        with store_lock:
            results = []
            for solution in store.query(query):
                dep_uri = str(solution['dep'])
                dep_id = dep_uri.split('/')[-1]
                results.append({
                    'id': dep_id,
                    'description': str(solution['description']),
                    'status': str(solution['status'])
                })

        return results

    def optimize_task_distribution(self) -> List[str]:
        """Intelligently select tasks for optimal parallel execution."""
        analytics = self.get_task_analytics()
        available_slots = analytics['available_slots']

        if available_slots <= 0:
            return []

        ready_tasks = self.get_ready_tasks(limit=available_slots * 2)

        # Advanced selection: prioritize diverse tasks to maximize parallelism
        # This is a simple heuristic - in production, use more sophisticated algorithms
        selected = []
        task_descriptions = set()

        for task in ready_tasks:
            if len(selected) >= available_slots:
                break

            # Simple diversity check - avoid duplicate descriptions
            desc_key = task['description'][:50].lower()
            if desc_key not in task_descriptions:
                selected.append(task['id'])
                task_descriptions.add(desc_key)

        # Fill remaining slots with highest priority tasks
        for task in ready_tasks:
            if len(selected) >= available_slots:
                break
            if task['id'] not in selected:
                selected.append(task['id'])

        return selected


# Global orchestrator instance
orchestrator = OxigraphOrchestrator()


# REST API Endpoints

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({'status': 'healthy', 'service': 'oxigraph-orchestrator'})


@app.route('/tasks', methods=['POST'])
def create_task():
    """Create a new task."""
    data = request.json

    task = AgentTask(
        id=data.get('id', str(uuid.uuid4())),
        agent_id=data.get('agent_id', ''),
        description=data['description'],
        status=data.get('status', 'pending'),
        priority=data.get('priority', 0),
        dependencies=data.get('dependencies', []),
        created_at=datetime.utcnow().isoformat(),
        metadata=data.get('metadata')
    )

    task_uri = orchestrator.create_task(task)
    return jsonify({'task_id': task.id, 'task_uri': task_uri}), 201


@app.route('/tasks/<task_id>/status', methods=['PUT'])
def update_task_status_endpoint(task_id):
    """Update task status."""
    data = request.json
    status = data['status']
    result = data.get('result')

    success = orchestrator.update_task_status(task_id, status, result)
    return jsonify({'success': success})


@app.route('/tasks/ready', methods=['GET'])
def get_ready_tasks():
    """Get tasks ready for execution."""
    limit = request.args.get('limit', 10, type=int)
    tasks = orchestrator.get_ready_tasks(limit)
    return jsonify({'tasks': tasks, 'count': len(tasks)})


@app.route('/tasks/running', methods=['GET'])
def get_running_tasks():
    """Get currently running tasks."""
    tasks = orchestrator.get_running_tasks()
    return jsonify({'tasks': tasks, 'count': len(tasks)})


@app.route('/analytics', methods=['GET'])
def get_analytics():
    """Get task analytics."""
    analytics = orchestrator.get_task_analytics()
    return jsonify(analytics)


@app.route('/tasks/<task_id>/chain', methods=['GET'])
def get_task_chain(task_id):
    """Get task dependency chain."""
    chain = orchestrator.get_task_chain(task_id)
    return jsonify({'chain': chain, 'count': len(chain)})


@app.route('/optimize', methods=['GET'])
def optimize_distribution():
    """Get optimized task distribution."""
    tasks = orchestrator.optimize_task_distribution()
    return jsonify({'tasks': tasks, 'count': len(tasks)})


if __name__ == '__main__':
    logger.info("Starting Oxigraph Orchestrator Service")
    logger.info(f"Max concurrent agents: {orchestrator.max_concurrent_agents}")
    app.run(host='0.0.0.0', port=5000, threaded=True)
