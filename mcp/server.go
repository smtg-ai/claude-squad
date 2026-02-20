package mcp

import (
	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

const serverInstructions = "You are running inside Hivemind, a multi-agent orchestration system. " +
	"You may be one of several agents working in parallel on the same codebase. " +
	"Be a good teammate: check what others are working on, avoid file conflicts, " +
	"share useful discoveries, and keep your work focused."

// HivemindMCPServer wraps an MCP server with Hivemind-specific state.
type HivemindMCPServer struct {
	server      *mcpserver.MCPServer
	stateReader *StateReader
	hivemindDir string
	instanceID  string // used by Tier 2 introspection tools (Phase 3)
	tier        int    // gates tool registration: 1=read, 2=+introspect, 3=+write (Phase 4)
}

// NewHivemindMCPServer creates a new MCP server for a Hivemind agent.
func NewHivemindMCPServer(hivemindDir, instanceID string, tier int) *HivemindMCPServer {
	s := mcpserver.NewMCPServer(
		"hivemind",
		"0.1.0",
		mcpserver.WithInstructions(serverInstructions),
	)

	h := &HivemindMCPServer{
		server:      s,
		stateReader: NewStateReader(hivemindDir),
		instanceID:  instanceID,
		tier:        tier,
		hivemindDir: hivemindDir,
	}

	h.registerTier1Tools()

	return h
}

// registerTier1Tools registers read-only Tier 1 tools.
func (h *HivemindMCPServer) registerTier1Tools() {
	listInstances := gomcp.NewTool("list_instances",
		gomcp.WithDescription(
			"See all Hivemind instances, their status, current activity, and branch. "+
				"Use this to understand what the swarm is working on before starting work.",
		),
		gomcp.WithReadOnlyHintAnnotation(true),
	)
	h.server.AddTool(listInstances, handleListInstances(h.stateReader))

	checkFileActivity := gomcp.NewTool("check_file_activity",
		gomcp.WithDescription(
			"Check if other agents are currently modifying specific files. "+
				"Use this before editing shared files to avoid merge conflicts.",
		),
		gomcp.WithString("files",
			gomcp.Required(),
			gomcp.Description("Comma-separated file paths to check for activity by other agents."),
		),
		gomcp.WithReadOnlyHintAnnotation(true),
	)
	h.server.AddTool(checkFileActivity, handleCheckFileActivity())

	getSharedContext := gomcp.NewTool("get_shared_context",
		gomcp.WithDescription(
			"Read discoveries and decisions published by other agents. "+
				"Check this early to learn patterns and conventions others have found.",
		),
		gomcp.WithReadOnlyHintAnnotation(true),
	)
	h.server.AddTool(getSharedContext, handleGetSharedContext(h.hivemindDir))
}

// Serve starts the MCP server using stdio transport.
func (h *HivemindMCPServer) Serve() error {
	return mcpserver.ServeStdio(h.server)
}
