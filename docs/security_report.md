# Claude Squad Security Analysis Report

Date: 2025-06-10

## 1. Summary

This report details the findings of a security analysis conducted on the Claude Squad repository. The analysis focused on identifying potential vulnerabilities related to data routing, command execution, file system interaction, data storage, and the installation process.

Overall, the application's use of Go's `os/exec` package with distinct arguments for command construction significantly mitigates common command injection risks. The primary concerns lie with the installation script's lack of integrity checks, the handling of user-configurable program names and paths in stored configuration, and the behavior of the daemon's `AutoYes` feature. The web UI was confirmed to be a static informational site and does not pose a direct vulnerability to the Go application.

## 2. Findings and Recommendations

### 2.1. Installation Script (`install.sh`)

*   **Finding 1.1: Missing Cryptographic Integrity Check for Downloaded Binaries**
    *   **Severity**: High
    *   **Description**: The `install.sh` script downloads release binaries from GitHub but does not verify their integrity using cryptographic checksums (e.g., SHA256).
    *   **Impact**: An attacker who compromises the GitHub releases page or intercepts the download could replace the binary with a malicious version, leading to arbitrary code execution on the user's machine during installation.
    *   **Recommendation**:
        1.  Generate SHA256 checksums for all release assets during the build process.
        2.  Upload these checksums alongside the assets to the GitHub release.
        3.  Modify `install.sh` to download the appropriate checksum file and verify the integrity of the downloaded binary archive before extraction and installation. Exit if validation fails.

*   **Finding 1.2: Dependency Installation with `sudo`**
    *   **Severity**: Medium
    *   **Description**: The script uses `sudo` with package managers (`apt-get`, `dnf`, etc.) to install dependencies like `tmux` and `gh`. Some steps involve `curl | sudo dd` or `echo | sudo tee` for adding the `gh` repository.
    *   **Impact**: If the sources for these commands (e.g., `cli.github.com` for `gh` GPG key and repo info) are compromised or if the user's system/network is subject to MITM attacks (less likely with HTTPS but possible with sophisticated attacks), malicious commands could be executed with root privileges.
    *   **Recommendation**:
        1.  This is a common pattern, but users should be aware of the trust placed in these external sources when running the script.
        2.  Consider advising users to install dependencies manually if they prefer, or to carefully review the script sections involving `sudo`.
        3.  Ensure all `curl` commands for GPG keys and repository information use HTTPS and securely fetch resources.

*   **Finding 1.3: General Risk of `curl | bash`**
    *   **Severity**: Informational (User Awareness)
    *   **Description**: The recommended installation method `curl ... | bash` is inherently risky as it executes a script from the internet directly.
    *   **Impact**: If the URL is mistyped, or in certain compromise scenarios (e.g., DNS hijacking, TLS stripping if HTTPS were not enforced by `curl -fsSL`), a malicious script could be executed.
    *   **Recommendation**: Provide an alternative, safer installation instruction: download the script, inspect it, then execute it locally.

### 2.2. Command Construction and Execution

*   **Finding 2.1: Shell Command Execution in `config.GetClaudeCommand()`**
    *   **Severity**: Low
    *   **Description**: The `GetClaudeCommand()` function in `config/config.go` uses `exec.Command(shell, "-c", "source ...; which claude")` to find the `claude` executable by sourcing shell profiles.
    *   **Impact**: If the `SHELL` environment variable is maliciously crafted to point to a fake shell or includes complex arguments, it could potentially lead to unexpected behavior or (in extreme, unlikely cases for typical `os/exec` usage) command manipulation. More practically, it relies on `which` and shell sourcing, which can be less robust than `exec.LookPath`.
    *   **Recommendation**: Prioritize using `exec.LookPath("claude")` first. If that fails, then consider falling back to the shell-based discovery, ensuring the `shell` variable is treated with caution or restricted to known safe shells.

*   **Finding 2.2: User-Configurable Program in Tmux Sessions**
    *   **Severity**: Medium (Context-Dependent, see also 2.3)
    *   **Description**: The `program` argument in `tmux new-session ... program` (in `session/tmux/tmux.go`) can be derived from `config.DefaultProgram` or the `--program` flag, and also from the `Program` field in stored instance data (`state.json`).
    *   **Impact**: If an attacker can modify `config.json` or `state.json` (see Section 2.4), they could set this `program` to a malicious script or command. While `os/exec` passes it as a single argument (preventing direct shell injection via this variable alone), if the program is set to something like `/bin/sh` and subsequent interactions (e.g., via daemon `AutoYes`) send controllable input, it could lead to arbitrary command execution.
    *   **Recommendation**:
        1.  When loading `Program` from config, validate it to ensure it doesn't contain unexpected shell metacharacters or arguments if it's not simply a path.
        2.  Consider if `program` should always be a full path to an executable or come from a predefined list of known safe programs.

### 2.3. File System Interactions

*   **Finding 2.3.1: Worktree Path Construction**
    *   **Severity**: Low
    *   **Description**: Worktree paths are constructed using `filepath.Join(worktreeDir, sanitizedName) + timestamp`. `sanitizedName` comes from `sessionName` via `sanitizeBranchName`. `sanitizeBranchName` allows `/` and `.` characters.
    *   **Impact**: While `filepath.Join` cleans paths and should prevent traversal outside `worktreeDir` (e.g., `../`), the allowance of `/` means users can create nested directory structures within `~/.claude-squad/worktrees/` using session names like `my/nested/session`. This is not a direct vulnerability but could lead to unexpectedly deep paths or minor "directory traversal" *within* the designated worktree storage. The timestamp suffix largely prevents direct overwrites.
    *   **Recommendation**: This is likely acceptable. If stricter control over path structure within `worktrees` is desired, `sanitizeBranchName` could be made to disallow `/`, or paths could be further flattened.

### 2.4. Data Serialization and Storage (`config.json`, `state.json`)

*   **Finding 2.4.1: Trust in Stored Instance Paths and Programs**
    *   **Severity**: Medium
    *   **Description**: The application loads instance configurations from `state.json`, including `Path` (repository path) and `Program` (program to run in tmux). `config.json` stores `DefaultProgram`.
    *   **Impact**: If an attacker gains write access to `~/.claude-squad/config.json` or `~/.claude-squad/state.json`, they can:
        *   Change `Program` or `DefaultProgram` to a malicious executable or script. When the instance/daemon starts, this malicious program will be executed.
        *   Change `Path` or `Worktree.RepoPath` to point to different Git repositories, potentially causing the application to operate on unintended data or execute hooks from a malicious repository if such hooks are triggered by the application's git operations.
    *   **Recommendation**:
        1.  The application must operate under the assumption that these configuration files can be user-modified (maliciously or accidentally).
        2.  For `Program` / `DefaultProgram`: Critical. Consider if these should be validated against a list of known programs or if their execution should be sandboxed/restricted if they are arbitrary. At a minimum, clearly document that these are executable paths defined by the user's config.
        3.  For `Path` / `RepoPath`: The check `findGitRepoRoot` provides some safety by ensuring it's a git repo. Further validation (e.g., against a list of user-approved repo locations) is likely overkill for a local tool but could be considered for higher-security contexts.

### 2.5. Daemon Behavior (`daemon/daemon.go`)

*   **Finding 2.5.1: Daemon Forces `AutoYes` on All Instances**
    *   **Severity**: Medium (User Experience / Potential for Unintended Action)
    *   **Description**: The daemon, when active, unconditionally sets `instance.AutoYes = true` for all loaded instances, overriding any per-instance `AutoYes` setting from `state.json`.
    *   **Impact**: Users might have specific sessions they do not want to auto-progress. The daemon's current behavior bypasses this, potentially leading to the daemon automatically sending "Enter" to prompts in sessions the user intended to manage manually.
    *   **Recommendation**: Modify the daemon to respect the `AutoYes` flag stored for each instance. If a global "daemon autoyes" is desired, make this a separate explicit global configuration, and ensure users are aware.

*   **Finding 2.5.2: Fragile Prompt Detection for `AutoYes`**
    *   **Severity**: Low to Medium
    *   **Description**: The daemon's `AutoYes` feature (via `TmuxSession.HasUpdated` and `TapEnter`) relies on detecting specific substrings in the tmux pane output (e.g., "No, and tell Claude what to do differently") to identify prompts.
    *   **Impact**:
        *   If these prompt strings change in future versions of Claude/Aider, the `AutoYes` feature will break silently.
        *   If these substrings accidentally appear in other, non-prompt contexts (e.g., in code generated by the AI, or in error messages), the daemon could mistakenly send an "Enter", potentially causing unintended actions. The current strings are fairly specific, reducing this risk.
    *   **Recommendation**:
        1.  This is an inherent challenge when automating CLI tools via screen scraping.
        2.  Explore if Claude/Aider offer more robust ways to signal that they are at a user prompt (e.g., specific exit codes for a "check prompt" command, IPC, or more unique/non-printable markers in the TUI).
        3.  Regularly test compatibility with new versions of the supported AI tools.

## 3. Conclusion

Claude Squad is a powerful tool for managing AI agent sessions. The primary security enhancements should focus on:
1.  Securing the installation process by adding binary integrity checks.
2.  Carefully managing how user-defined program paths in configuration files are handled and executed.
3.  Refining daemon behavior to be more predictable and respectful of user settings.

By addressing these areas, the overall security posture of Claude Squad can be significantly improved.
