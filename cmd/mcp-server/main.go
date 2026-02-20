package main

import (
	"fmt"
	"os"
	"path/filepath"

	hivemindmcp "github.com/ByteMirror/hivemind/mcp"
)

func main() {
	hivemindDir := os.Getenv("HIVEMIND_DIR")
	if hivemindDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "hivemind-mcp: failed to get home directory: %v\n", err)
			os.Exit(1)
		}
		hivemindDir = filepath.Join(homeDir, ".hivemind")
	}

	instanceID := os.Getenv("HIVEMIND_INSTANCE_ID")

	srv := hivemindmcp.NewHivemindMCPServer(hivemindDir, instanceID, 1)
	if err := srv.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "hivemind-mcp: %v\n", err)
		os.Exit(1)
	}
}
