import styles from "./page.module.css";
import dynamic from "next/dynamic";

// Import ThemeToggle and CopyButton dynamically to prevent hydration issues
const ThemeToggle = dynamic(() => import("./components/ThemeToggle"), {
});

const CopyButton = dynamic(() => import("./components/CopyButton"), {
});

export default function Home() {
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <h1 className={styles.title}>hivemind</h1>
        <div className={styles.headerActions}>
          <a
            className={styles.headerButton}
            href="https://github.com/ByteMirror/hivemind"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub
          </a>
          <a
            href="https://github.com/ByteMirror/hivemind#readme"
            target="_blank"
            rel="noopener noreferrer"
            className={styles.headerButton}
          >
            Docs
          </a>
          <ThemeToggle />
        </div>
      </header>
      <main className={styles.main}>


        <p className={styles.tagline}>
          A TUI-based agent-driven IDE. Manage <span className={styles.highlight}>Claude Code</span>, <span className={styles.highlight}>Codex</span>, <span className={styles.highlight}>Aider</span>, and more. <br/><span className={styles.tenx}>10x</span> your productivity
        </p>

        <div className={styles.demoVideo}>
          <video
            controls
            autoPlay
            muted
            loop
            playsInline
            className={styles.video}
            src="https://github.com/user-attachments/assets/aef18253-e58f-4525-9032-f5a3d66c975a"
          />
        </div>

        <div className={styles.installation}>
          <h2>Installation</h2>
          <h3>Via Homebrew</h3>
          <div className={styles.codeBlockWrapper}>
            <pre className={styles.codeBlock}>
              <code>brew install ByteMirror/tap/hivemind</code>
            </pre>
            <CopyButton textToCopy="brew install ByteMirror/tap/hivemind" />
          </div>
          <br></br>
          <h3>Via Scoop (Windows)</h3>
          <div className={styles.codeBlockWrapper}>
            <pre className={styles.codeBlock}>
              <code>scoop bucket add bytemirror https://github.com/ByteMirror/scoop-bucket{"\n"}scoop install hivemind</code>
            </pre>
            <CopyButton textToCopy="scoop bucket add bytemirror https://github.com/ByteMirror/scoop-bucket && scoop install hivemind" />
          </div>
          <br></br>
          <h3>Via Go Install</h3>
          <div className={styles.codeBlockWrapper}>
            <pre className={styles.codeBlock}>
              <code>go install github.com/ByteMirror/hivemind@latest</code>
            </pre>
            <CopyButton textToCopy="go install github.com/ByteMirror/hivemind@latest" />
          </div>
          <br></br>
          <h3>Via Shell Script</h3>
          <div className={styles.codeBlockWrapper}>
            <pre className={styles.codeBlock}>
              <code>curl -fsSL https://raw.githubusercontent.com/ByteMirror/hivemind/main/install.sh | bash</code>
            </pre>
            <CopyButton textToCopy="curl -fsSL https://raw.githubusercontent.com/ByteMirror/hivemind/main/install.sh | bash" />
          </div>
          <p className={styles.prerequisites}>
            Prerequisites: tmux, gh (GitHub CLI)
          </p>
        </div>

        <div className={styles.features}>
          <h2>Why use Hivemind?</h2>
          <ul>
            <li>Supervise multiple agents in one UI</li>
            <li>Isolate tasks in git workspaces</li>
            <li>Review work before shipping</li>
          </ul>
        </div>
      </main>
      <footer className={styles.footer}>
        <p className={styles.copyright}>
          &copy; {new Date().getFullYear()} Hivemind. Licensed under <a href="https://github.com/ByteMirror/hivemind/blob/main/LICENSE.md" target="_blank" rel="noopener noreferrer">GNU AGPL v3.0</a>
        </p>
      </footer>
    </div>
  );
}
