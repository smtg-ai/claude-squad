"""
Terminal automation core for Claude Squad TUI interaction.
"""
import asyncio
import pexpect
import pyte
import time
import json
import logging
from typing import Dict, List, Optional, Any
from dataclasses import dataclass

logger = logging.getLogger(__name__)

@dataclass
class InstanceInfo:
    index: int
    name: str
    status: str  # Running ●, Ready ●, Paused ⏸
    project: Optional[str]
    branch: str
    git_stats: Dict[str, int]  # {'+': 5, '-': 3}

@dataclass
class ScreenState:
    instances: List[InstanceInfo]
    selected_instance: int
    current_tab: str  # preview, diff, console
    tab_content: str
    menu_items: List[str]
    error_message: Optional[str]

class ClaudeSquadAutomator:
    """Core automation engine for Claude Squad TUI"""
    
    def __init__(self, terminal_width: int = 120, terminal_height: int = 30):
        self.process: Optional[pexpect.spawn] = None
        self.screen = pyte.Screen(terminal_width, terminal_height)
        self.stream = pyte.ByteStream(self.screen)
        self.width = terminal_width
        self.height = terminal_height
        self._last_screen_hash = None
        self._running = False
        
        # Key mappings for Claude Squad
        self.key_mappings = {
            'up': '\x1b[A',
            'down': '\x1b[B',
            'left': '\x1b[D', 
            'right': '\x1b[C',
            'tab': '\t',
            'enter': '\r',
            'escape': '\x1b',
            'shift_up': '\x1b[1;2A',
            'shift_down': '\x1b[1;2B',
            'ctrl_c': '\x03',
            # Claude Squad specific keys
            'n': 'n',           # New instance
            'N': 'N',           # New instance with prompt
            'D': 'D',           # Kill instance
            'c': 'c',           # Checkout
            'r': 'r',           # Resume
            'p': 'p',           # Push/Submit
            'P': 'P',           # Add project
            'q': 'q',           # Quit
            '?': '?',           # Help
            'o': 'o',           # Open/Attach
        }
    
    async def start_claude_squad(self, program: str = "cs") -> bool:
        """Start Claude Squad application"""
        try:
            self.process = pexpect.spawn(
                program,
                encoding='utf-8',
                dimensions=(self.height, self.width)
            )
            self.process.logfile_read = self._capture_output
            self._running = True
            
            # Wait for initial UI to load
            await asyncio.sleep(2)
            await self._wait_for_ui_stable()
            
            logger.info("Claude Squad started successfully")
            return True
            
        except Exception as e:
            logger.error(f"Failed to start Claude Squad: {e}")
            return False
    
    def _capture_output(self, data: str):
        """Capture and process terminal output"""
        if data:
            self.stream.feed(data.encode('utf-8'))
    
    async def send_key(self, key: str, wait_for_update: bool = True) -> bool:
        """Send key to Claude Squad and optionally wait for UI update"""
        if not self.process or not self._running:
            logger.error("Claude Squad not running")
            return False
            
        try:
            key_sequence = self.key_mappings.get(key, key)
            self.process.send(key_sequence)
            
            if wait_for_update:
                return await self._wait_for_ui_update()
            return True
            
        except Exception as e:
            logger.error(f"Failed to send key '{key}': {e}")
            return False
    
    async def send_text(self, text: str, wait_for_update: bool = True) -> bool:
        """Send text input to Claude Squad"""
        if not self.process or not self._running:
            return False
            
        try:
            self.process.send(text)
            if wait_for_update:
                return await self._wait_for_ui_update()
            return True
            
        except Exception as e:
            logger.error(f"Failed to send text: {e}")
            return False
    
    def get_screen_content(self) -> ScreenState:
        """Extract structured content from current screen"""
        lines = []
        for row in self.screen.buffer:
            line = ''.join(char.data for char in row).rstrip()
            lines.append(line)
        
        # Parse the screen into structured data
        instances = self._parse_instance_list(lines)
        selected_instance = self._detect_selected_instance(lines)
        current_tab = self._detect_current_tab(lines)
        tab_content = self._extract_tab_content(lines, current_tab)
        menu_items = self._parse_menu_items(lines)
        error_message = self._extract_error_message(lines)
        
        return ScreenState(
            instances=instances,
            selected_instance=selected_instance,
            current_tab=current_tab,
            tab_content=tab_content,
            menu_items=menu_items,
            error_message=error_message
        )
    
    def _parse_instance_list(self, lines: List[str]) -> List[InstanceInfo]:
        """Parse the left panel instance list"""
        instances = []
        
        # Look for instance lines in the left panel (first ~30% of width)
        left_panel_width = self.width // 3
        
        for i, line in enumerate(lines[1:-3]):  # Skip header and footer
            if len(line) < 3:
                continue
                
            # Extract left panel portion
            left_portion = line[:left_panel_width].strip()
            
            # Check if this looks like an instance line
            if self._is_instance_line(left_portion):
                instance = self._extract_instance_info(left_portion, len(instances))
                if instance:
                    instances.append(instance)
        
        return instances
    
    def _is_instance_line(self, line: str) -> bool:
        """Check if line contains instance information"""
        # Look for status indicators
        status_indicators = ['●', '⏸', '○']
        return any(indicator in line for indicator in status_indicators)
    
    def _extract_instance_info(self, line: str, index: int) -> Optional[InstanceInfo]:
        """Extract instance info from a line"""
        try:
            # Parse status
            status = "Unknown"
            if '●' in line:
                # Check color or context to determine if Running or Ready
                status = "Running" if "Running" in line else "Ready"
            elif '⏸' in line:
                status = "Paused"
            elif '○' in line:
                status = "Stopped"
            
            # Extract name (usually the main text of the line)
            name_part = line.strip()
            # Remove status indicators and git stats
            for indicator in ['●', '⏸', '○', '+', '-']:
                name_part = name_part.replace(indicator, '').strip()
            
            # Extract git stats
            git_stats = {}
            import re
            git_match = re.search(r'\+(\d+)\s*-(\d+)', line)
            if git_match:
                git_stats = {'+': int(git_match.group(1)), '-': int(git_match.group(2))}
            
            # Extract project and branch (simplified)
            project = None
            branch = "main"  # default
            
            # Look for project info in parentheses
            project_match = re.search(r'\(([^)]+)\)', line)
            if project_match:
                project = project_match.group(1)
            
            return InstanceInfo(
                index=index,
                name=name_part[:20],  # Truncate long names
                status=status,
                project=project,
                branch=branch,
                git_stats=git_stats
            )
            
        except Exception as e:
            logger.warning(f"Failed to parse instance line '{line}': {e}")
            return None
    
    def _detect_selected_instance(self, lines: List[str]) -> int:
        """Detect which instance is currently selected"""
        # Look for highlighting or selection indicators
        for i, line in enumerate(lines):
            # Common selection indicators in TUIs
            if line.strip().startswith('>') or '█' in line:
                # Try to map back to instance index
                return self._line_to_instance_index(i, lines)
        return 0
    
    def _line_to_instance_index(self, line_num: int, lines: List[str]) -> int:
        """Convert screen line number to instance index"""
        # Count instance lines before this line
        instance_count = 0
        for i in range(min(line_num, len(lines))):
            if i > 0 and self._is_instance_line(lines[i][:self.width//3]):
                if i == line_num:
                    return instance_count
                instance_count += 1
        return max(0, instance_count - 1)
    
    def _detect_current_tab(self, lines: List[str]) -> str:
        """Detect which tab is currently active"""
        # Look for tab indicators in the top portion
        for line in lines[:5]:
            if '█' in line:  # Active tab indicator
                if 'Preview' in line:
                    return 'preview'
                elif 'Diff' in line:
                    return 'diff'
                elif 'Console' in line:
                    return 'console'
        return 'preview'  # default
    
    def _extract_tab_content(self, lines: List[str], current_tab: str) -> str:
        """Extract content from the current tab"""
        # Find the content area (right panel, below tabs)
        left_panel_width = self.width // 3
        content_lines = []
        
        # Skip header lines and extract right panel content
        for line in lines[3:-3]:  # Skip tabs and menu
            if len(line) > left_panel_width:
                content_lines.append(line[left_panel_width:].rstrip())
        
        return '\n'.join(content_lines)
    
    def _parse_menu_items(self, lines: List[str]) -> List[str]:
        """Parse menu items from bottom of screen"""
        menu_items = []
        # Look at bottom few lines for menu
        for line in lines[-3:]:
            if '│' in line or '|' in line:  # Menu separators
                # Extract menu items
                items = line.split('│')
                for item in items:
                    clean_item = item.strip()
                    if clean_item and len(clean_item) < 20:
                        menu_items.append(clean_item)
        return menu_items
    
    def _extract_error_message(self, lines: List[str]) -> Optional[str]:
        """Extract error message if present"""
        for line in lines[-5:]:  # Check bottom lines for errors
            if 'error' in line.lower() or 'failed' in line.lower():
                return line.strip()
        return None
    
    async def _wait_for_ui_update(self, timeout: float = 2.0) -> bool:
        """Wait for UI to update after an action"""
        start_hash = self._get_screen_hash()
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            await asyncio.sleep(0.1)
            current_hash = self._get_screen_hash()
            if current_hash != start_hash:
                # Additional small delay to ensure update is complete
                await asyncio.sleep(0.2)
                return True
        
        logger.warning(f"UI did not update within {timeout}s")
        return False
    
    async def _wait_for_ui_stable(self, timeout: float = 3.0) -> bool:
        """Wait for UI to become stable (no changes for a period)"""
        stable_period = 0.5  # seconds of stability required
        last_hash = None
        stable_start = None
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            current_hash = self._get_screen_hash()
            
            if current_hash == last_hash:
                if stable_start is None:
                    stable_start = time.time()
                elif time.time() - stable_start >= stable_period:
                    return True
            else:
                stable_start = None
                last_hash = current_hash
            
            await asyncio.sleep(0.1)
        
        return False
    
    def _get_screen_hash(self) -> str:
        """Get hash of current screen state for change detection"""
        screen_text = '\n'.join(''.join(char.data for char in row) for row in self.screen.buffer)
        return str(hash(screen_text))
    
    async def stop(self):
        """Stop Claude Squad and cleanup"""
        self._running = False
        if self.process and self.process.isalive():
            self.process.send('q')  # Quit command
            await asyncio.sleep(1)
            if self.process.isalive():
                self.process.terminate()
                await asyncio.sleep(1)
                if self.process.isalive():
                    self.process.kill()
        logger.info("Claude Squad stopped")