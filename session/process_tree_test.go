package session

import (
	"testing"
)

func TestParseProcessTree_BasicTree(t *testing.T) {
	// Simulate: zsh(1) → claude(2) → node(3) → node(4)
	psOutput := `    1     0 /bin/zsh          0.1  1024
    2     1 claude            5.0  2048
    3     2 /usr/bin/node    10.0  4096
    4     3 node              2.0  1024
`

	tree, err := parseProcessTree(psOutput)
	if err != nil {
		t.Fatal(err)
	}

	if len(tree.procs) != 4 {
		t.Fatalf("expected 4 processes, got %d", len(tree.procs))
	}

	// Verify comm is basename-only
	if tree.procs[3].comm != "node" {
		t.Errorf("expected comm 'node', got %q", tree.procs[3].comm)
	}
}

func TestParseProcessTree_Descendants(t *testing.T) {
	// zsh(1) → claude(2) → node(3) → node(4)
	//                    → node(5)
	psOutput := `    1     0 zsh               0.1  1024
    2     1 claude            5.0  2048
    3     2 node             10.0  4096
    4     3 node              2.0  1024
    5     2 node              3.0  2048
`

	tree, err := parseProcessTree(psOutput)
	if err != nil {
		t.Fatal(err)
	}

	// Descendants of zsh (PID 1) should be [2,3,4,5]
	desc := tree.descendants(1)
	if len(desc) != 4 {
		t.Fatalf("expected 4 descendants of PID 1, got %d", len(desc))
	}

	// Descendants of claude (PID 2) should be [3,4,5]
	desc = tree.descendants(2)
	if len(desc) != 3 {
		t.Fatalf("expected 3 descendants of PID 2, got %d", len(desc))
	}

	// Descendants of node (PID 4) should be empty
	desc = tree.descendants(4)
	if len(desc) != 0 {
		t.Fatalf("expected 0 descendants of PID 4, got %d", len(desc))
	}
}

func TestParseProcessTree_AggregateCPUMem(t *testing.T) {
	psOutput := `    1     0 zsh               0.1  1024
    2     1 claude            5.0  2048
    3     2 node             10.0  4096
`

	tree, err := parseProcessTree(psOutput)
	if err != nil {
		t.Fatal(err)
	}

	desc := tree.descendants(1)
	var totalCPU, totalRSS float64
	for _, p := range desc {
		totalCPU += p.cpu
		totalRSS += p.rss
	}

	if totalCPU != 15.0 {
		t.Errorf("expected total CPU 15.0, got %.1f", totalCPU)
	}
	if totalRSS != 6144.0 {
		t.Errorf("expected total RSS 6144.0, got %.1f", totalRSS)
	}
}

func TestParseProcessTree_EmptyOutput(t *testing.T) {
	tree, err := parseProcessTree("")
	if err != nil {
		t.Fatal(err)
	}

	if len(tree.procs) != 0 {
		t.Fatalf("expected 0 processes, got %d", len(tree.procs))
	}

	desc := tree.descendants(1)
	if len(desc) != 0 {
		t.Fatalf("expected 0 descendants, got %d", len(desc))
	}
}

func TestParseProcessTree_MalformedLines(t *testing.T) {
	psOutput := `    1     0 zsh               0.1  1024
bad line
    2     1 claude            5.0  2048
incomplete
`

	tree, err := parseProcessTree(psOutput)
	if err != nil {
		t.Fatal(err)
	}

	// Should parse the 2 valid lines and skip malformed ones
	if len(tree.procs) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(tree.procs))
	}
}

func TestParseProcessTree_PathBasename(t *testing.T) {
	psOutput := `    1     0 /usr/local/bin/zsh     0.1  1024
    2     1 /home/user/.local/bin/claude  5.0  2048
`

	tree, err := parseProcessTree(psOutput)
	if err != nil {
		t.Fatal(err)
	}

	if tree.procs[1].comm != "zsh" {
		t.Errorf("expected 'zsh', got %q", tree.procs[1].comm)
	}
	if tree.procs[2].comm != "claude" {
		t.Errorf("expected 'claude', got %q", tree.procs[2].comm)
	}
}

func TestParseProcessTree_ToolProcessDetection(t *testing.T) {
	// Verify that toolProcessNames correctly identifies tool processes
	// within a sub-agent's process tree (used for activity detection).
	psOutput := `    1     0 zsh               0.1  1024
    2     1 claude            5.0  2048
    3     2 git               0.5   256
    4     2 rg                1.0   512
    5     2 node              3.0  1024
`

	tree, err := parseProcessTree(psOutput)
	if err != nil {
		t.Fatal(err)
	}

	desc := tree.descendants(2)
	var toolCount int
	for _, p := range desc {
		if toolProcessNames[p.comm] {
			toolCount++
		}
	}

	if toolCount != 3 {
		t.Fatalf("expected 3 tool processes (git, rg, node), got %d", toolCount)
	}
}
