"""
Example workflows for Claude Squad MCP Server.
These demonstrate how LLMs can interact with Claude Squad through the MCP interface.
"""
import asyncio
import json
from typing import Dict, List, Any
from .mcp_server import ClaudeSquadMCPServer

class WorkflowExamples:
    """Collection of example workflows for Claude Squad automation"""
    
    def __init__(self, mcp_server: ClaudeSquadMCPServer):
        self.server = mcp_server
        self.automator = mcp_server.automator
    
    async def workflow_create_and_monitor_instance(self, task_name: str, prompt: str) -> Dict[str, Any]:
        """
        Example Workflow: Create instance and monitor its progress
        
        This simulates what an LLM would do:
        1. Create a new instance with a task
        2. Monitor its progress 
        3. Review the results
        """
        results = {
            "workflow": "create_and_monitor",
            "steps": [],
            "status": "success"
        }
        
        try:
            # Step 1: Create instance
            results["steps"].append({
                "step": "create_instance",
                "action": f"Creating instance '{task_name}' with prompt",
                "timestamp": "now"
            })
            
            # Use MCP tools to create instance
            await self.server.handle_call_tool("create_instance", {
                "name": task_name,
                "with_prompt": True,
                "prompt": prompt
            })
            
            # Step 2: Wait and monitor
            results["steps"].append({
                "step": "monitor_start", 
                "action": "Starting to monitor instance progress"
            })
            
            for i in range(10):  # Monitor for up to 10 cycles
                await asyncio.sleep(2)
                
                # Get current state
                screen_state = self.automator.get_screen_content()
                
                # Check if our instance is ready
                target_instance = None
                for inst in screen_state.instances:
                    if inst.name.strip() == task_name.strip():
                        target_instance = inst
                        break
                
                if target_instance:
                    results["steps"].append({
                        "step": f"monitor_cycle_{i}",
                        "instance_status": target_instance.status,
                        "git_stats": target_instance.git_stats
                    })
                    
                    if target_instance.status == "Ready":
                        break
                else:
                    results["steps"].append({
                        "step": f"monitor_cycle_{i}",
                        "error": "Instance not found"
                    })
            
            # Step 3: Review results
            results["steps"].append({
                "step": "review_results",
                "action": "Switching to diff tab to review changes"
            })
            
            await self.server.handle_call_tool("switch_tab", {"tab": "diff"})
            
            # Get diff content
            diff_content = await self.server.handle_read_resource("claude-squad://content/diff")
            
            results["final_state"] = {
                "instance_found": target_instance is not None,
                "instance_status": target_instance.status if target_instance else None,
                "diff_preview": diff_content[:500] if diff_content else "No changes"
            }
            
        except Exception as e:
            results["status"] = "error"
            results["error"] = str(e)
        
        return results
    
    async def workflow_review_all_instances(self) -> Dict[str, Any]:
        """
        Example Workflow: Review all active instances
        
        This demonstrates systematic instance review:
        1. Get list of all instances
        2. Navigate through each one
        3. Collect status and changes
        4. Generate summary report
        """
        results = {
            "workflow": "review_all_instances",
            "instances_reviewed": [],
            "summary": {}
        }
        
        try:
            # Get current state
            screen_state = self.automator.get_screen_content()
            
            for instance in screen_state.instances:
                review_data = {
                    "index": instance.index,
                    "name": instance.name,
                    "status": instance.status,
                    "git_stats": instance.git_stats
                }
                
                # Navigate to this instance
                await self.server.handle_call_tool("navigate_to_instance", {
                    "index": instance.index
                })
                
                # Check each tab for content
                for tab in ["preview", "diff", "console"]:
                    await self.server.handle_call_tool("switch_tab", {"tab": tab})
                    content = await self.server.handle_read_resource(f"claude-squad://content/{tab}")
                    
                    review_data[f"{tab}_content_length"] = len(content)
                    review_data[f"{tab}_has_content"] = len(content.strip()) > 0
                    
                    if tab == "diff" and content.strip():
                        # Basic analysis of diff
                        lines = content.split('\n')
                        review_data["diff_analysis"] = {
                            "total_lines": len(lines),
                            "added_lines": len([l for l in lines if l.startswith('+')]),
                            "removed_lines": len([l for l in lines if l.startswith('-')])
                        }
                
                results["instances_reviewed"].append(review_data)
            
            # Generate summary
            total_instances = len(results["instances_reviewed"])
            running_instances = len([i for i in results["instances_reviewed"] if i["status"] == "Running"])
            ready_instances = len([i for i in results["instances_reviewed"] if i["status"] == "Ready"])
            paused_instances = len([i for i in results["instances_reviewed"] if i["status"] == "Paused"])
            
            total_git_changes = sum(
                inst["git_stats"].get("+", 0) + inst["git_stats"].get("-", 0) 
                for inst in results["instances_reviewed"]
            )
            
            results["summary"] = {
                "total_instances": total_instances,
                "running": running_instances,
                "ready": ready_instances,
                "paused": paused_instances,
                "total_git_changes": total_git_changes,
                "instances_with_diffs": len([
                    i for i in results["instances_reviewed"] 
                    if i.get("diff_has_content", False)
                ])
            }
            
        except Exception as e:
            results["error"] = str(e)
        
        return results
    
    async def workflow_automated_code_review(self, instance_name: str) -> Dict[str, Any]:
        """
        Example Workflow: Automated code review for specific instance
        
        This demonstrates how an LLM could:
        1. Navigate to specific instance
        2. Examine the diff
        3. Provide feedback via prompt
        4. Monitor response
        """
        results = {
            "workflow": "automated_code_review",
            "instance_name": instance_name,
            "review_steps": []
        }
        
        try:
            # Find the target instance
            screen_state = self.automator.get_screen_content()
            target_instance = None
            
            for inst in screen_state.instances:
                if instance_name.lower() in inst.name.lower():
                    target_instance = inst
                    break
            
            if not target_instance:
                results["error"] = f"Instance '{instance_name}' not found"
                return results
            
            # Navigate to the instance
            await self.server.handle_call_tool("navigate_to_instance", {
                "index": target_instance.index
            })
            
            results["review_steps"].append({
                "step": "navigate_to_instance",
                "target_index": target_instance.index,
                "instance_status": target_instance.status
            })
            
            # Switch to diff tab
            await self.server.handle_call_tool("switch_tab", {"tab": "diff"})
            
            # Get diff content
            diff_content = await self.server.handle_read_resource("claude-squad://content/diff")
            
            results["review_steps"].append({
                "step": "analyze_diff",
                "diff_length": len(diff_content),
                "has_changes": len(diff_content.strip()) > 0
            })
            
            if diff_content.strip():
                # Simulate LLM analysis of the diff
                diff_lines = diff_content.split('\n')
                analysis = {
                    "total_lines": len(diff_lines),
                    "files_changed": len(set(
                        line.split()[1] for line in diff_lines 
                        if line.startswith('+++') or line.startswith('---')
                    )),
                    "additions": len([l for l in diff_lines if l.startswith('+') and not l.startswith('+++')]),
                    "deletions": len([l for l in diff_lines if l.startswith('-') and not l.startswith('---')])
                }
                
                # Generate review feedback
                feedback_prompt = f"""
Code Review for {instance_name}:

Changes Summary:
- {analysis['files_changed']} files changed
- {analysis['additions']} lines added
- {analysis['deletions']} lines deleted

The changes look good. Please add unit tests for any new functionality and ensure proper error handling.
                """.strip()
                
                results["review_steps"].append({
                    "step": "generate_feedback",
                    "analysis": analysis,
                    "feedback_prompt": feedback_prompt
                })
                
                # Send feedback to the instance
                await self.server.handle_call_tool("send_prompt", {
                    "prompt": feedback_prompt
                })
                
                results["review_steps"].append({
                    "step": "send_feedback",
                    "action": "Feedback sent to instance"
                })
            
            else:
                results["review_steps"].append({
                    "step": "no_changes",
                    "message": "No changes found to review"
                })
            
        except Exception as e:
            results["error"] = str(e)
        
        return results
    
    async def workflow_batch_operation(self, operation: str) -> Dict[str, Any]:
        """
        Example Workflow: Batch operations on multiple instances
        
        Operations: 'checkout_all', 'review_all', 'push_ready'
        """
        results = {
            "workflow": f"batch_{operation}",
            "operations": [],
            "summary": {}
        }
        
        try:
            screen_state = self.automator.get_screen_content()
            
            for instance in screen_state.instances:
                operation_result = {
                    "instance_index": instance.index,
                    "instance_name": instance.name,
                    "initial_status": instance.status
                }
                
                # Navigate to instance
                await self.server.handle_call_tool("navigate_to_instance", {
                    "index": instance.index
                })
                
                if operation == "checkout_all":
                    if instance.status in ["Running", "Ready"]:
                        await self.server.handle_call_tool("checkout_instance", {})
                        operation_result["action"] = "checked_out"
                    else:
                        operation_result["action"] = "skipped_not_running"
                
                elif operation == "push_ready":
                    if instance.status == "Ready" and instance.git_stats:
                        await self.server.handle_call_tool("push_changes", {})
                        operation_result["action"] = "pushed_changes"
                    else:
                        operation_result["action"] = "skipped_no_changes"
                
                elif operation == "review_all":
                    # Quick review - just check for changes
                    await self.server.handle_call_tool("switch_tab", {"tab": "diff"})
                    diff_content = await self.server.handle_read_resource("claude-squad://content/diff")
                    operation_result["action"] = "reviewed"
                    operation_result["has_changes"] = len(diff_content.strip()) > 0
                    operation_result["change_count"] = len(diff_content.split('\n'))
                
                results["operations"].append(operation_result)
                
                # Small delay between operations
                await asyncio.sleep(0.5)
            
            # Generate summary
            total_ops = len(results["operations"])
            successful_ops = len([op for op in results["operations"] if "action" in op])
            
            results["summary"] = {
                "total_instances": total_ops,
                "successful_operations": successful_ops,
                "operation_type": operation
            }
            
        except Exception as e:
            results["error"] = str(e)
        
        return results

# Example usage functions that an LLM would call
async def example_usage():
    """Example of how an LLM would use these workflows"""
    
    # Initialize the MCP server
    mcp_server = ClaudeSquadMCPServer()
    
    # Start Claude Squad
    await mcp_server.automator.start_claude_squad()
    
    # Create workflow handler
    workflows = WorkflowExamples(mcp_server)
    
    # Example 1: Create and monitor a new task
    result1 = await workflows.workflow_create_and_monitor_instance(
        "fix-auth-bug", 
        "Fix the authentication bug in user login. Add proper error handling and tests."
    )
    print("Workflow 1 Result:", json.dumps(result1, indent=2))
    
    # Example 2: Review all instances
    result2 = await workflows.workflow_review_all_instances()
    print("Workflow 2 Result:", json.dumps(result2, indent=2))
    
    # Example 3: Automated code review
    result3 = await workflows.workflow_automated_code_review("fix-auth-bug")
    print("Workflow 3 Result:", json.dumps(result3, indent=2))
    
    # Example 4: Batch checkout all instances
    result4 = await workflows.workflow_batch_operation("checkout_all")
    print("Workflow 4 Result:", json.dumps(result4, indent=2))
    
    # Cleanup
    await mcp_server.automator.stop()

if __name__ == "__main__":
    # Run example workflows
    asyncio.run(example_usage())