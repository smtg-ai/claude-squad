#!/usr/bin/env python3
"""
Comprehensive test suite for RDF/Turtle best practices implementation.
Tests W3C compliance, SPARQL queries, provenance, and validation.
"""

import unittest
import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from pyoxigraph import Store, NamedNode, Literal, Triple
import json
from datetime import datetime


class TestRDFBestPractices(unittest.TestCase):
    """Test W3C RDF best practices implementation."""

    def setUp(self):
        """Set up test store."""
        self.store = Store()

        # Namespaces
        self.cs = "http://claude-squad.ai/ontology/v1.0.0#"
        self.rdf = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
        self.rdfs = "http://www.w3.org/2000/01/rdf-schema#"
        self.owl = "http://www.w3.org/2002/07/owl#"
        self.xsd = "http://www.w3.org/2001/XMLSchema#"
        self.prov = "http://www.w3.org/ns/prov#"

    def test_namespace_versioning(self):
        """Test that ontology is properly versioned."""
        # Best Practice #2: Version your ontology
        self.assertIn("v1.0.0", self.cs)
        self.assertTrue(self.cs.startswith("http://claude-squad.ai/ontology/"))

    def test_use_standard_vocabularies(self):
        """Test that standard W3C vocabularies are used."""
        # Best Practice #1: Use standard vocabularies
        standard_vocabs = [self.rdf, self.rdfs, self.owl, self.xsd, self.prov]

        for vocab in standard_vocabs:
            self.assertTrue(vocab.startswith("http://www.w3.org/") or
                          vocab.startswith("http://purl.org/"))

    def test_triple_creation(self):
        """Test basic triple creation."""
        task_uri = f"{self.cs}task/test-001"

        triple = Triple(
            NamedNode(task_uri),
            NamedNode(f"{self.rdf}type"),
            NamedNode(f"{self.cs}Task")
        )

        self.store.add(triple)

        # Verify triple was added
        results = list(self.store.quads_for_pattern(
            NamedNode(task_uri),
            NamedNode(f"{self.rdf}type"),
            None,
            None
        ))

        self.assertEqual(len(results), 1)

    def test_batch_operations(self):
        """Test batch triple addition."""
        # Best Practice #5: Use batch operations
        triples = []

        for i in range(100):
            task_uri = f"{self.cs}task/batch-{i:03d}"
            triples.extend([
                Triple(NamedNode(task_uri), NamedNode(f"{self.rdf}type"),
                      NamedNode(f"{self.cs}Task")),
                Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasPriority"),
                      Literal(str(i % 10), datatype=NamedNode(f"{self.xsd}integer"))),
            ])

        # Add all at once
        for triple in triples:
            self.store.add(triple)

        # Verify all were added
        query = f"""
        PREFIX cs: <{self.cs}>
        PREFIX rdf: <{self.rdf}>
        SELECT (COUNT(?task) as ?count)
        WHERE {{ ?task rdf:type cs:Task }}
        """

        result = list(self.store.query(query))
        count = int(str(result[0]['count']))

        self.assertEqual(count, 100)

    def test_sparql_with_prefixes(self):
        """Test SPARQL queries use proper prefixes."""
        # Best Practice #3: Define proper prefixes
        task_uri = f"{self.cs}task/test-002"

        self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.rdf}type"),
                             NamedNode(f"{self.cs}Task")))
        self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasStatus"),
                             Literal("pending")))

        query = f"""
        PREFIX cs: <{self.cs}>
        PREFIX rdf: <{self.rdf}>

        SELECT ?task WHERE {{
            ?task rdf:type cs:Task ;
                  cs:hasStatus "pending" .
        }}
        """

        results = list(self.store.query(query))
        self.assertEqual(len(results), 1)

    def test_property_domain_range(self):
        """Test that properties have proper domain and range."""
        # Best Practice #8: Define domain and range
        depends_on = f"{self.cs}dependsOn"

        # Add property definition
        self.store.add(Triple(NamedNode(depends_on), NamedNode(f"{self.rdf}type"),
                             NamedNode(f"{self.owl}ObjectProperty")))
        self.store.add(Triple(NamedNode(depends_on), NamedNode(f"{self.rdfs}domain"),
                             NamedNode(f"{self.cs}Task")))
        self.store.add(Triple(NamedNode(depends_on), NamedNode(f"{self.rdfs}range"),
                             NamedNode(f"{self.cs}Task")))

        # Verify
        query = f"""
        PREFIX rdfs: <{self.rdfs}>
        SELECT ?domain ?range WHERE {{
            <{depends_on}> rdfs:domain ?domain ;
                          rdfs:range ?range .
        }}
        """

        results = list(self.store.query(query))
        self.assertEqual(len(results), 1)
        self.assertEqual(str(results[0]['domain']), f"{self.cs}Task")
        self.assertEqual(str(results[0]['range']), f"{self.cs}Task")

    def test_transitive_property(self):
        """Test transitive property (dependsOn)."""
        # Best Practice: Use OWL property characteristics
        task1 = f"{self.cs}task/001"
        task2 = f"{self.cs}task/002"
        task3 = f"{self.cs}task/003"

        # Create dependency chain: task3 -> task2 -> task1
        self.store.add(Triple(NamedNode(task3), NamedNode(f"{self.cs}dependsOn"),
                             NamedNode(task2)))
        self.store.add(Triple(NamedNode(task2), NamedNode(f"{self.cs}dependsOn"),
                             NamedNode(task1)))

        # Query with property path (transitive closure)
        query = f"""
        PREFIX cs: <{self.cs}>
        SELECT ?dep WHERE {{
            <{task3}> cs:dependsOn+ ?dep .
        }}
        """

        results = list(self.store.query(query))
        deps = [str(r['dep']) for r in results]

        # Should find both task2 and task1
        self.assertIn(task2, deps)
        self.assertIn(task1, deps)
        self.assertEqual(len(results), 2)

    def test_provenance_tracking(self):
        """Test PROV-O provenance tracking."""
        # Best Practice #9: Use PROV-O for provenance
        task_uri = f"{self.cs}task/prov-test"
        activity_uri = f"{self.cs}execution/exec-001"
        agent_uri = f"{self.cs}agent/claude-001"

        # Create provenance triples
        triples = [
            # Task as entity
            Triple(NamedNode(task_uri), NamedNode(f"{self.rdf}type"),
                  NamedNode(f"{self.prov}Entity")),

            # Execution as activity
            Triple(NamedNode(activity_uri), NamedNode(f"{self.rdf}type"),
                  NamedNode(f"{self.prov}Activity")),
            Triple(NamedNode(activity_uri), NamedNode(f"{self.prov}used"),
                  NamedNode(task_uri)),

            # Agent
            Triple(NamedNode(agent_uri), NamedNode(f"{self.rdf}type"),
                  NamedNode(f"{self.prov}Agent")),
            Triple(NamedNode(activity_uri), NamedNode(f"{self.prov}wasAssociatedWith"),
                  NamedNode(agent_uri)),
        ]

        for triple in triples:
            self.store.add(triple)

        # Query provenance
        query = f"""
        PREFIX prov: <{self.prov}>
        SELECT ?activity ?agent WHERE {{
            ?activity prov:used <{task_uri}> ;
                     prov:wasAssociatedWith ?agent .
        }}
        """

        results = list(self.store.query(query))
        self.assertEqual(len(results), 1)
        self.assertEqual(str(results[0]['activity']), activity_uri)
        self.assertEqual(str(results[0]['agent']), agent_uri)

    def test_optional_properties(self):
        """Test SPARQL OPTIONAL pattern."""
        # Best Practice #11: Use OPTIONAL for optional properties
        task1 = f"{self.cs}task/opt-001"
        task2 = f"{self.cs}task/opt-002"
        workflow = f"{self.cs}workflow/wf-001"

        # task1 has workflow, task2 doesn't
        self.store.add(Triple(NamedNode(task1), NamedNode(f"{self.rdf}type"),
                             NamedNode(f"{self.cs}Task")))
        self.store.add(Triple(NamedNode(task1), NamedNode(f"{self.cs}partOfWorkflow"),
                             NamedNode(workflow)))

        self.store.add(Triple(NamedNode(task2), NamedNode(f"{self.rdf}type"),
                             NamedNode(f"{self.cs}Task")))

        query = f"""
        PREFIX cs: <{self.cs}>
        PREFIX rdf: <{self.rdf}>
        SELECT ?task ?workflow WHERE {{
            ?task rdf:type cs:Task .
            OPTIONAL {{ ?task cs:partOfWorkflow ?workflow }}
        }}
        """

        results = list(self.store.query(query))
        self.assertEqual(len(results), 2)

        # One should have workflow, one shouldn't
        workflows = [r.get('workflow') for r in results]
        self.assertIn(None, workflows)
        self.assertTrue(any(w is not None for w in workflows))

    def test_aggregate_functions(self):
        """Test SPARQL aggregate functions."""
        # Best Practice #16: Use aggregate functions
        statuses = ["pending", "running", "completed", "failed"]

        for i, status in enumerate(statuses * 5):
            task_uri = f"{self.cs}task/agg-{i:03d}"
            self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasStatus"),
                                 Literal(status)))

        query = f"""
        PREFIX cs: <{self.cs}>
        SELECT ?status (COUNT(?task) as ?count)
        WHERE {{
            ?task cs:hasStatus ?status .
        }}
        GROUP BY ?status
        """

        results = list(self.store.query(query))
        self.assertEqual(len(results), 4)

        for result in results:
            self.assertEqual(int(str(result['count'])), 5)

    def test_union_pattern(self):
        """Test SPARQL UNION pattern."""
        # Best Practice #13: Use UNION for alternatives
        task = f"{self.cs}task/union-test"
        activity1 = f"{self.cs}execution/exec-001"
        activity2 = f"{self.cs}execution/exec-002"

        # Two different provenance patterns
        self.store.add(Triple(NamedNode(activity1), NamedNode(f"{self.prov}used"),
                             NamedNode(task)))
        self.store.add(Triple(NamedNode(task), NamedNode(f"{self.prov}wasGeneratedBy"),
                             NamedNode(activity2)))

        query = f"""
        PREFIX prov: <{self.prov}>
        SELECT ?activity WHERE {{
            {{
                ?activity prov:used <{task}> .
            }} UNION {{
                <{task}> prov:wasGeneratedBy ?activity .
            }}
        }}
        """

        results = list(self.store.query(query))
        activities = [str(r['activity']) for r in results]

        self.assertEqual(len(results), 2)
        self.assertIn(activity1, activities)
        self.assertIn(activity2, activities)

    def test_turtle_serialization(self):
        """Test Turtle export."""
        # Best Practice #6: Support standard serializations
        task_uri = f"{self.cs}task/turtle-test"

        self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.rdf}type"),
                             NamedNode(f"{self.cs}Task")))
        self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasDescription"),
                             Literal("Test task for Turtle export")))

        # Export to Turtle
        turtle_output = self.store.dump(format="text/turtle")

        # Verify Turtle contains our data
        turtle_str = turtle_output.decode('utf-8')
        self.assertIn("Task", turtle_str)
        self.assertIn("hasDescription", turtle_str)

    def test_datatype_validation(self):
        """Test XSD datatype usage."""
        task_uri = f"{self.cs}task/datatype-test"

        # Add typed literals
        self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasPriority"),
                             Literal("10", datatype=NamedNode(f"{self.xsd}integer"))))
        self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}createdAt"),
                             Literal("2025-01-01T00:00:00Z",
                                   datatype=NamedNode(f"{self.xsd}dateTime"))))

        # Query with type checking
        query = f"""
        PREFIX cs: <{self.cs}>
        PREFIX xsd: <{self.xsd}>
        SELECT ?priority ?created WHERE {{
            <{task_uri}> cs:hasPriority ?priority ;
                        cs:createdAt ?created .
        }}
        """

        results = list(self.store.query(query))
        self.assertEqual(len(results), 1)

    def test_filter_optimization(self):
        """Test SPARQL FILTER optimization."""
        # Add tasks with different priorities
        for i in range(20):
            task_uri = f"{self.cs}task/filter-{i:03d}"
            self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasPriority"),
                                 Literal(str(i % 10), datatype=NamedNode(f"{self.xsd}integer"))))

        # Filter for high priority tasks
        query = f"""
        PREFIX cs: <{self.cs}>
        SELECT ?task ?priority WHERE {{
            ?task cs:hasPriority ?priority .
            FILTER(?priority >= 8)
        }}
        """

        results = list(self.store.query(query))

        # Should find tasks with priority 8 and 9
        for result in results:
            priority = int(str(result['priority']))
            self.assertGreaterEqual(priority, 8)

    def test_order_by_optimization(self):
        """Test SPARQL ORDER BY optimization."""
        # Add tasks with different priorities
        priorities = [5, 10, 3, 8, 1]
        for i, priority in enumerate(priorities):
            task_uri = f"{self.cs}task/order-{i:03d}"
            self.store.add(Triple(NamedNode(task_uri), NamedNode(f"{self.cs}hasPriority"),
                                 Literal(str(priority), datatype=NamedNode(f"{self.xsd}integer"))))

        query = f"""
        PREFIX cs: <{self.cs}>
        SELECT ?task ?priority WHERE {{
            ?task cs:hasPriority ?priority .
        }}
        ORDER BY DESC(?priority)
        """

        results = list(self.store.query(query))

        # Verify descending order
        prev_priority = 11  # Higher than max
        for result in results:
            priority = int(str(result['priority']))
            self.assertLessEqual(priority, prev_priority)
            prev_priority = priority


class TestSPARQLQueryLibrary(unittest.TestCase):
    """Test SPARQL query library patterns."""

    def setUp(self):
        """Set up test environment."""
        self.store = Store()
        self.cs = "http://claude-squad.ai/ontology/v1.0.0#"

    def test_ready_tasks_query(self):
        """Test optimized ready tasks query."""
        # Create test data
        task1 = f"{self.cs}task/ready-001"
        task2 = f"{self.cs}task/ready-002"
        task3 = f"{self.cs}task/ready-003"  # Has incomplete dependency

        for task in [task1, task2, task3]:
            self.store.add(Triple(NamedNode(task), NamedNode(f"{self.cs}hasStatus"),
                                 Literal("pending")))

        # task3 depends on task1 which is still pending
        self.store.add(Triple(NamedNode(task3), NamedNode(f"{self.cs}dependsOn"),
                             NamedNode(task1)))

        # Query should exclude task3
        query = f"""
        PREFIX cs: <{self.cs}>
        SELECT ?task WHERE {{
            ?task cs:hasStatus "pending" .
            FILTER NOT EXISTS {{
                ?task cs:dependsOn ?dep .
                ?dep cs:hasStatus ?depStatus .
                FILTER(?depStatus != "completed")
            }}
        }}
        """

        results = list(self.store.query(query))
        task_uris = [str(r['task']) for r in results]

        # Should find task1 and task2, but not task3
        self.assertIn(task1, task_uris)
        self.assertIn(task2, task_uris)
        self.assertNotIn(task3, task_uris)


def run_tests():
    """Run all tests."""
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()

    suite.addTests(loader.loadTestsFromTestCase(TestRDFBestPractices))
    suite.addTests(loader.loadTestsFromTestCase(TestSPARQLQueryLibrary))

    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    return result.wasSuccessful()


if __name__ == '__main__':
    success = run_tests()
    sys.exit(0 if success else 1)
