#!/usr/bin/env python3
"""
Main entry point for Claude Squad MCP Server
"""
import asyncio
import sys
from claude_squad_mcp.mcp_server import main

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nMCP Server stopped")
        sys.exit(0)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)