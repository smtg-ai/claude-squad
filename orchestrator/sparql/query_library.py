"""
SPARQL Query Library - W3C Best Practice Compliant Queries
Best Practice: Maintain reusable, optimized SPARQL query templates
"""

from typing import Dict, Any


class SPARQLQueryLibrary:
    """
    Library of optimized SPARQL queries for common operations.
    Following W3C SPARQL best practices.
    """

    @staticmethod
    def prefixes() -> str:
        """Standard prefix declarations."""
        return """
PREFIX cs: <http://claude-squad.ai/ontology/v1.0.0#>
PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
PREFIX owl: <http://www.w3.org/2002/07/owl#>
PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
PREFIX dcterms: <http://purl.org/dc/terms/>
PREFIX prov: <http://www.w3.org/ns/prov#>
"""

    # =========================================================================
    # Task Queries
    # =========================================================================

    @staticmethod
    def get_ready_tasks_optimized(limit: int = 10) -> str:
        """
        Get tasks ready to execute with no incomplete dependencies.
        Optimized with:
        - OPTIONAL for optional properties
        - Subquery for dependency check
        - Proper ordering
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT DISTINCT ?task ?description ?priority ?createdAt ?workflow ?agent
WHERE {{
    # Main task properties
    ?task a cs:Task ;
          cs:hasStatus "pending" ;
          cs:hasDescription ?description ;
          cs:hasPriority ?priority ;
          cs:createdAt ?createdAt .

    # Optional properties
    OPTIONAL {{ ?task cs:partOfWorkflow ?workflow }}
    OPTIONAL {{ ?task cs:assignedTo ?agent }}

    # Subquery: Check no incomplete dependencies
    FILTER NOT EXISTS {{
        ?task cs:dependsOn ?dep .
        ?dep cs:hasStatus ?depStatus .
        FILTER(?depStatus IN ("pending", "running", "failed"))
    }}
}}
ORDER BY DESC(?priority) ?createdAt
LIMIT {limit}
"""

    @staticmethod
    def get_blocked_tasks() -> str:
        """Get tasks blocked by failed dependencies."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?task ?description ?blocker ?blockerStatus
WHERE {{
    ?task a cs:Task ;
          cs:hasStatus "pending" ;
          cs:hasDescription ?description ;
          cs:dependsOn ?blocker .

    ?blocker cs:hasStatus ?blockerStatus .
    FILTER(?blockerStatus = "failed")
}}
ORDER BY ?task
"""

    @staticmethod
    def get_dependency_tree(task_id: str, max_depth: int = 10) -> str:
        """
        Get complete dependency tree using property paths.
        Best Practice: Use property paths for transitive queries.
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?dep ?description ?status ?depth
WHERE {{
    {{
        SELECT ?dep (COUNT(?mid) as ?depth)
        WHERE {{
            <http://claude-squad.ai/ontology/task/{task_id}> cs:dependsOn* ?mid .
            ?mid cs:dependsOn* ?dep .
        }}
        GROUP BY ?dep
        HAVING (?depth <= {max_depth})
    }}

    ?dep cs:hasDescription ?description ;
         cs:hasStatus ?status .
}}
ORDER BY ?depth ?dep
"""

    @staticmethod
    def get_workflow_tasks(workflow_id: str) -> str:
        """Get all tasks in a workflow."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?task ?description ?status ?priority
WHERE {{
    ?task cs:partOfWorkflow <http://claude-squad.ai/ontology/workflow/{workflow_id}> ;
          cs:hasDescription ?description ;
          cs:hasStatus ?status ;
          cs:hasPriority ?priority .
}}
ORDER BY DESC(?priority) ?description
"""

    # =========================================================================
    # Analytics Queries
    # =========================================================================

    @staticmethod
    def get_comprehensive_analytics() -> str:
        """
        Comprehensive analytics with aggregations.
        Best Practice: Use GROUP BY and aggregate functions.
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?status
       (COUNT(?task) as ?count)
       (AVG(?priority) as ?avgPriority)
       (MIN(?createdAt) as ?oldestTask)
       (MAX(?createdAt) as ?newestTask)
WHERE {{
    ?task a cs:Task ;
          cs:hasStatus ?status ;
          cs:hasPriority ?priority ;
          cs:createdAt ?createdAt .
}}
GROUP BY ?status
ORDER BY ?status
"""

    @staticmethod
    def get_performance_metrics() -> str:
        """
        Get execution performance metrics.
        Best Practice: Calculate derived metrics in SPARQL.
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?status
       (COUNT(?exec) as ?execCount)
       (AVG(?duration) as ?avgDuration)
       (MIN(?duration) as ?minDuration)
       (MAX(?duration) as ?maxDuration)
WHERE {{
    ?task a cs:Task ;
          cs:hasStatus ?status ;
          cs:hasExecution ?exec .

    ?exec cs:hasDuration ?duration .
}}
GROUP BY ?status
"""

    @staticmethod
    def get_agent_workload() -> str:
        """Get workload distribution across agents."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?agent
       (COUNT(?task) as ?totalTasks)
       (SUM(IF(?status = "completed", 1, 0)) as ?completedTasks)
       (SUM(IF(?status = "running", 1, 0)) as ?runningTasks)
       (SUM(IF(?status = "failed", 1, 0)) as ?failedTasks)
WHERE {{
    ?task cs:assignedTo ?agent ;
          cs:hasStatus ?status .
}}
GROUP BY ?agent
ORDER BY DESC(?totalTasks)
"""

    # =========================================================================
    # Provenance Queries
    # =========================================================================

    @staticmethod
    def get_task_provenance(task_id: str) -> str:
        """
        Get complete provenance chain for a task.
        Best Practice: Use UNION for alternative patterns.
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?activity ?activityType ?agent ?startTime ?endTime ?entity
WHERE {{
    {{
        # Activities that used this task
        ?activity prov:used <http://claude-squad.ai/ontology/task/{task_id}> .
        BIND(<http://claude-squad.ai/ontology/task/{task_id}> as ?entity)
    }} UNION {{
        # Activities that generated this task
        <http://claude-squad.ai/ontology/task/{task_id}> prov:wasGeneratedBy ?activity .
        BIND(<http://claude-squad.ai/ontology/task/{task_id}> as ?entity)
    }}

    ?activity a ?activityType .

    OPTIONAL {{ ?activity prov:wasAssociatedWith ?agent }}
    OPTIONAL {{ ?activity prov:startedAtTime ?startTime }}
    OPTIONAL {{ ?activity prov:endedAtTime ?endTime }}
}}
ORDER BY ?startTime
"""

    @staticmethod
    def get_derivation_chain(task_id: str) -> str:
        """Get all tasks derived from a source task."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?derived ?description ?derivationType
WHERE {{
    ?derived prov:wasDerivedFrom+ <http://claude-squad.ai/ontology/task/{task_id}> ;
             cs:hasDescription ?description .

    OPTIONAL {{
        ?derived prov:wasDerivedFrom <http://claude-squad.ai/ontology/task/{task_id}> .
        ?derived a ?derivationType .
    }}
}}
"""

    # =========================================================================
    # Advanced Queries
    # =========================================================================

    @staticmethod
    def find_critical_path() -> str:
        """
        Find critical path (longest dependency chain).
        Best Practice: Complex graph algorithms in SPARQL.
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?task ?description (MAX(?depth) as ?maxDepth)
WHERE {{
    {{
        SELECT ?task (COUNT(?dep) as ?depth)
        WHERE {{
            ?task a cs:Task ;
                  cs:hasStatus "pending" .

            OPTIONAL {{
                ?task cs:dependsOn+ ?dep .
                ?dep cs:hasStatus ?depStatus .
                FILTER(?depStatus != "completed")
            }}
        }}
        GROUP BY ?task
    }}

    ?task cs:hasDescription ?description .
}}
GROUP BY ?task ?description
ORDER BY DESC(?maxDepth)
LIMIT 1
"""

    @staticmethod
    def suggest_next_tasks(available_slots: int) -> str:
        """
        Suggest next tasks to execute using multiple criteria.
        Best Practice: Multi-criteria optimization.
        """
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?task ?description ?priority ?age ?score
WHERE {{
    ?task a cs:Task ;
          cs:hasStatus "pending" ;
          cs:hasDescription ?description ;
          cs:hasPriority ?priority ;
          cs:createdAt ?createdAt .

    # No incomplete dependencies
    FILTER NOT EXISTS {{
        ?task cs:dependsOn ?dep .
        ?dep cs:hasStatus ?depStatus .
        FILTER(?depStatus != "completed")
    }}

    # Calculate age in hours
    BIND(xsd:integer((NOW() - ?createdAt) / xsd:dayTimeDuration("PT1H")) as ?age)

    # Calculate score: priority + age/10 (favor both high priority and old tasks)
    BIND(?priority + (?age / 10.0) as ?score)
}}
ORDER BY DESC(?score)
LIMIT {available_slots}
"""

    @staticmethod
    def detect_anomalies() -> str:
        """Detect anomalous patterns in task execution."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT ?issue ?task ?details
WHERE {{
    {{
        # Long running tasks (>1 hour)
        SELECT ("long_running" as ?issue) ?task (CONCAT("Running for ", STR(?duration), " seconds") as ?details)
        WHERE {{
            ?task cs:hasStatus "running" ;
                  cs:hasExecution ?exec .
            ?exec cs:startedAt ?start .

            BIND((NOW() - ?start) / xsd:dayTimeDuration("PT1S") as ?duration)
            FILTER(?duration > 3600)
        }}
    }} UNION {{
        # Tasks with many failed attempts
        SELECT ("multiple_failures" as ?issue) ?task (CONCAT(STR(?failCount), " failed executions") as ?details)
        WHERE {{
            SELECT ?task (COUNT(?exec) as ?failCount)
            WHERE {{
                ?task cs:hasExecution ?exec .
                ?exec cs:hasStatus "failed" .
            }}
            GROUP BY ?task
            HAVING (?failCount >= 3)
        }}
    }} UNION {{
        # Orphaned tasks (no workflow, no dependencies, old)
        SELECT ("orphaned" as ?issue) ?task ("No workflow or dependencies, pending >24h" as ?details)
        WHERE {{
            ?task cs:hasStatus "pending" ;
                  cs:createdAt ?created .

            FILTER NOT EXISTS {{ ?task cs:partOfWorkflow ?wf }}
            FILTER NOT EXISTS {{ ?task cs:dependsOn ?dep }}
            FILTER NOT EXISTS {{ ?other cs:dependsOn ?task }}

            BIND((NOW() - ?created) / xsd:dayTimeDuration("PT1H") as ?age)
            FILTER(?age > 24)
        }}
    }}
}}
ORDER BY ?issue
"""

    # =========================================================================
    # Validation Queries
    # =========================================================================

    @staticmethod
    def validate_concurrency_limit(max_concurrent: int = 10) -> str:
        """Check if running tasks exceed limit."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT (COUNT(?task) as ?runningCount)
WHERE {{
    ?task cs:hasStatus "running" .
}}
HAVING (COUNT(?task) > {max_concurrent})
"""

    @staticmethod
    def detect_dependency_cycles() -> str:
        """Detect circular dependencies."""
        return f"""
{SPARQLQueryLibrary.prefixes()}

SELECT DISTINCT ?task ?description
WHERE {{
    ?task a cs:Task ;
          cs:hasDescription ?description ;
          cs:dependsOn+ ?task .
}}
"""


# Example usage
if __name__ == "__main__":
    library = SPARQLQueryLibrary()

    print("=" * 80)
    print("Ready Tasks Query:")
    print("=" * 80)
    print(library.get_ready_tasks_optimized())

    print("\n" + "=" * 80)
    print("Critical Path Query:")
    print("=" * 80)
    print(library.find_critical_path())

    print("\n" + "=" * 80)
    print("Anomaly Detection Query:")
    print("=" * 80)
    print(library.detect_anomalies())
