"""
MCP Server for Claude Squad TUI automation.
Provides tools and resources for LLMs to interact with Claude Squad.
"""
import asyncio
import json
import logging
from typing import Any, Dict, List, Optional

import mcp.server.stdio
import mcp.types as types
from mcp.server import NotificationOptions, Server
from mcp.server.models import InitializationOptions

from .terminal_automator import ClaudeSquadAutomator, InstanceInfo, ScreenState

# Setup logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("claude-squad-mcp")

class ClaudeSquadMCPServer:
    """MCP Server for Claude Squad automation"""
    
    def __init__(self):
        self.server = Server("claude-squad-tui")
        self.automator = ClaudeSquadAutomator()
        self._setup_handlers()
    
    def _setup_handlers(self):
        """Setup MCP server handlers"""
        
        # === TOOLS ===
        
        @self.server.list_tools()
        async def handle_list_tools() -> List[types.Tool]:
            """List available tools for Claude Squad automation"""
            return [
                types.Tool(
                    name="start_claude_squad",
                    description="Start Claude Squad application",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "program": {
                                "type": "string",
                                "description": "Command to start Claude Squad (default: 'cs')",
                                "default": "cs"
                            }
                        }
                    }
                ),
                types.Tool(
                    name="create_instance",
                    description="Create a new AI agent instance",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "name": {
                                "type": "string",
                                "description": "Name for the new instance"
                            },
                            "with_prompt": {
                                "type": "boolean",
                                "description": "Whether to immediately provide a prompt",
                                "default": False
                            },
                            "prompt": {
                                "type": "string",
                                "description": "Initial prompt for the instance (if with_prompt is True)"
                            }
                        },
                        "required": ["name"]
                    }
                ),
                types.Tool(
                    name="navigate_to_instance",
                    description="Navigate to a specific instance by index",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "index": {
                                "type": "integer",
                                "description": "0-based index of the instance to select"
                            }
                        },
                        "required": ["index"]
                    }
                ),
                types.Tool(
                    name="switch_tab",
                    description="Switch between tabs (Preview, Diff, Console)",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "tab": {
                                "type": "string",
                                "enum": ["preview", "diff", "console"],
                                "description": "Tab to switch to"
                            }
                        },
                        "required": ["tab"]
                    }
                ),
                types.Tool(
                    name="send_prompt",
                    description="Send a prompt to the currently selected instance",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "prompt": {
                                "type": "string",
                                "description": "Prompt to send to the AI agent"
                            }
                        },
                        "required": ["prompt"]
                    }
                ),
                types.Tool(
                    name="scroll_content",
                    description="Scroll content in the current tab",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "direction": {
                                "type": "string",
                                "enum": ["up", "down"],
                                "description": "Direction to scroll"
                            },
                            "amount": {
                                "type": "integer",
                                "description": "Number of lines to scroll (default: 1)",
                                "default": 1
                            }
                        },
                        "required": ["direction"]
                    }
                ),
                types.Tool(
                    name="kill_instance",
                    description="Kill/delete the currently selected instance",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "confirm": {
                                "type": "boolean",
                                "description": "Confirm deletion",
                                "default": False
                            }
                        }
                    }
                ),
                types.Tool(
                    name="checkout_instance",
                    description="Checkout (pause) the currently selected instance",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                ),
                types.Tool(
                    name="resume_instance",
                    description="Resume a paused instance",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                ),
                types.Tool(
                    name="push_changes",
                    description="Push changes from current instance to git",
                    inputSchema={
                        "type": "object",
                        "properties": {}
                    }
                ),
                types.Tool(
                    name="navigate",
                    description="Send navigation keys (up, down, left, right)",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "direction": {
                                "type": "string",
                                "enum": ["up", "down", "left", "right"],
                                "description": "Navigation direction"
                            },
                            "count": {
                                "type": "integer",
                                "description": "Number of times to repeat the navigation",
                                "default": 1
                            }
                        },
                        "required": ["direction"]
                    }
                ),
                types.Tool(
                    name="send_key",
                    description="Send any key to Claude Squad",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "key": {
                                "type": "string",
                                "description": "Key to send (e.g., 'n', 'tab', 'enter', '?')"
                            }
                        },
                        "required": ["key"]
                    }
                )
            ]
        
        @self.server.call_tool()
        async def handle_call_tool(name: str, arguments: Dict[str, Any]) -> List[types.TextContent]:
            """Handle tool calls"""
            try:
                if name == "start_claude_squad":
                    program = arguments.get("program", "cs")
                    success = await self.automator.start_claude_squad(program)
                    return [types.TextContent(
                        type="text", 
                        text=f"Claude Squad {'started successfully' if success else 'failed to start'}"
                    )]
                
                elif name == "create_instance":
                    instance_name = arguments["name"]
                    with_prompt = arguments.get("with_prompt", False)
                    prompt = arguments.get("prompt", "")
                    
                    # Send 'n' for new instance
                    key = 'N' if with_prompt else 'n'
                    await self.automator.send_key(key)
                    await asyncio.sleep(0.5)
                    
                    # Enter instance name
                    await self.automator.send_text(instance_name)
                    await self.automator.send_key('enter')
                    
                    if with_prompt and prompt:
                        await asyncio.sleep(1)
                        await self.automator.send_text(prompt)
                        await self.automator.send_key('enter')
                    
                    return [types.TextContent(
                        type="text",
                        text=f"Created instance '{instance_name}'" + 
                             (f" with prompt: '{prompt}'" if with_prompt else "")
                    )]
                
                elif name == "navigate_to_instance":
                    target_index = arguments["index"]
                    current_state = self.automator.get_screen_content()
                    current_index = current_state.selected_instance
                    
                    # Navigate to target instance
                    if target_index > current_index:
                        for _ in range(target_index - current_index):
                            await self.automator.send_key('down')
                    elif target_index < current_index:
                        for _ in range(current_index - target_index):
                            await self.automator.send_key('up')
                    
                    return [types.TextContent(
                        type="text",
                        text=f"Navigated to instance {target_index}"
                    )]
                
                elif name == "switch_tab":
                    target_tab = arguments["tab"]
                    current_state = self.automator.get_screen_content()
                    
                    tabs = ["preview", "diff", "console"]
                    current_tab_index = tabs.index(current_state.current_tab)
                    target_tab_index = tabs.index(target_tab)
                    
                    # Calculate how many tab presses needed
                    tab_presses = (target_tab_index - current_tab_index) % len(tabs)
                    
                    for _ in range(tab_presses):
                        await self.automator.send_key('tab')
                    
                    return [types.TextContent(
                        type="text",
                        text=f"Switched to {target_tab} tab"
                    )]
                
                elif name == "send_prompt":
                    prompt = arguments["prompt"]
                    
                    # Open/attach to instance and send prompt
                    await self.automator.send_key('o')  # Open instance
                    await asyncio.sleep(1)
                    
                    await self.automator.send_text(prompt)
                    await self.automator.send_key('enter')
                    
                    # Detach from session
                    await self.automator.send_key('ctrl_c')  # Detach
                    
                    return [types.TextContent(
                        type="text",
                        text=f"Sent prompt to instance: '{prompt}'"
                    )]
                
                elif name == "scroll_content":
                    direction = arguments["direction"]
                    amount = arguments.get("amount", 1)
                    
                    key = 'shift_up' if direction == 'up' else 'shift_down'
                    
                    for _ in range(amount):
                        await self.automator.send_key(key)
                    
                    return [types.TextContent(
                        type="text",
                        text=f"Scrolled {direction} {amount} lines"
                    )]
                
                elif name == "kill_instance":
                    confirm = arguments.get("confirm", False)
                    if not confirm:
                        return [types.TextContent(
                            type="text",
                            text="Instance deletion requires confirmation. Set 'confirm': true"
                        )]
                    
                    await self.automator.send_key('D')
                    # Handle confirmation dialog if it appears
                    await asyncio.sleep(0.5)
                    await self.automator.send_key('enter')  # Confirm
                    
                    return [types.TextContent(
                        type="text",
                        text="Instance deleted"
                    )]
                
                elif name == "checkout_instance":
                    await self.automator.send_key('c')
                    return [types.TextContent(
                        type="text",
                        text="Instance checked out (paused)"
                    )]
                
                elif name == "resume_instance":
                    await self.automator.send_key('r')
                    return [types.TextContent(
                        type="text",
                        text="Instance resumed"
                    )]
                
                elif name == "push_changes":
                    await self.automator.send_key('p')
                    return [types.TextContent(
                        type="text",
                        text="Changes pushed to git"
                    )]
                
                elif name == "navigate":
                    direction = arguments["direction"]
                    count = arguments.get("count", 1)
                    
                    for _ in range(count):
                        await self.automator.send_key(direction)
                    
                    return [types.TextContent(
                        type="text",
                        text=f"Navigated {direction} {count} times"
                    )]
                
                elif name == "send_key":
                    key = arguments["key"]
                    await self.automator.send_key(key)
                    return [types.TextContent(
                        type="text",
                        text=f"Sent key: {key}"
                    )]
                
                else:
                    return [types.TextContent(
                        type="text",
                        text=f"Unknown tool: {name}"
                    )]
                    
            except Exception as e:
                logger.error(f"Tool {name} failed: {e}")
                return [types.TextContent(
                    type="text",
                    text=f"Error executing {name}: {str(e)}"
                )]
        
        # === RESOURCES ===
        
        @self.server.list_resources()
        async def handle_list_resources() -> List[types.Resource]:
            """List available resources"""
            return [
                types.Resource(
                    uri="claude-squad://screen/current",
                    name="Current Screen State",
                    description="Current state of Claude Squad UI including instances, tabs, and content",
                    mimeType="application/json"
                ),
                types.Resource(
                    uri="claude-squad://instances/list",
                    name="Instance List",
                    description="List of all AI agent instances with their status",
                    mimeType="application/json"
                ),
                types.Resource(
                    uri="claude-squad://content/preview",
                    name="Preview Content",
                    description="Content from the Preview tab",
                    mimeType="text/plain"
                ),
                types.Resource(
                    uri="claude-squad://content/diff",
                    name="Diff Content", 
                    description="Content from the Diff tab (git changes)",
                    mimeType="text/plain"
                ),
                types.Resource(
                    uri="claude-squad://content/console",
                    name="Console Content",
                    description="Content from the Console tab",
                    mimeType="text/plain"
                )
            ]
        
        @self.server.read_resource()
        async def handle_read_resource(uri: str) -> str:
            """Read resource content"""
            try:
                current_state = self.automator.get_screen_content()
                
                if uri == "claude-squad://screen/current":
                    return json.dumps({
                        "instances": [
                            {
                                "index": inst.index,
                                "name": inst.name,
                                "status": inst.status,
                                "project": inst.project,
                                "branch": inst.branch,
                                "git_stats": inst.git_stats
                            }
                            for inst in current_state.instances
                        ],
                        "selected_instance": current_state.selected_instance,
                        "current_tab": current_state.current_tab,
                        "menu_items": current_state.menu_items,
                        "error_message": current_state.error_message
                    }, indent=2)
                
                elif uri == "claude-squad://instances/list":
                    return json.dumps([
                        {
                            "index": inst.index,
                            "name": inst.name,
                            "status": inst.status,
                            "project": inst.project,
                            "branch": inst.branch,
                            "git_stats": inst.git_stats
                        }
                        for inst in current_state.instances
                    ], indent=2)
                
                elif uri == "claude-squad://content/preview":
                    if current_state.current_tab == "preview":
                        return current_state.tab_content
                    else:
                        # Switch to preview tab and get content
                        await self.automator.send_key('tab')  # This is simplified
                        new_state = self.automator.get_screen_content()
                        return new_state.tab_content
                
                elif uri == "claude-squad://content/diff":
                    # Similar logic for diff tab
                    return current_state.tab_content if current_state.current_tab == "diff" else ""
                
                elif uri == "claude-squad://content/console":
                    # Similar logic for console tab
                    return current_state.tab_content if current_state.current_tab == "console" else ""
                
                else:
                    raise ValueError(f"Unknown resource URI: {uri}")
                    
            except Exception as e:
                logger.error(f"Failed to read resource {uri}: {e}")
                return f"Error reading resource: {str(e)}"

async def main():
    """Main entry point for the MCP server"""
    server_instance = ClaudeSquadMCPServer()
    
    async with mcp.server.stdio.stdio_server() as (read_stream, write_stream):
        await server_instance.server.run(
            read_stream,
            write_stream,
            InitializationOptions(
                server_name="claude-squad-tui",
                server_version="0.1.0",
                capabilities=server_instance.server.get_capabilities(
                    notification_options=NotificationOptions(),
                    experimental_capabilities={}
                )
            )
        )

if __name__ == "__main__":
    asyncio.run(main())