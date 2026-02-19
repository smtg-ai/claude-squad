"use client";

import { useState } from "react";
import styles from "./InstallTabs.module.css";

const installMethods = [
  {
    label: "Homebrew",
    command: "brew install ByteMirror/tap/hivemind",
  },
  {
    label: "Scoop",
    command: "scoop bucket add bytemirror https://github.com/ByteMirror/scoop-bucket\nscoop install hivemind",
  },
  {
    label: "Go Install",
    command: "go install github.com/ByteMirror/hivemind@latest",
  },
  {
    label: "Shell Script",
    command: "curl -fsSL https://raw.githubusercontent.com/ByteMirror/hivemind/main/install.sh | bash",
  },
];

export default function InstallTabs() {
  const [activeTab, setActiveTab] = useState(0);
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(installMethods[activeTab].command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className={styles.terminal}>
      <div className={styles.titleBar}>
        <span className={`${styles.dot} ${styles.dotRed}`} />
        <span className={`${styles.dot} ${styles.dotYellow}`} />
        <span className={`${styles.dot} ${styles.dotGreen}`} />
      </div>
      <div className={styles.tabs}>
        {installMethods.map((method, i) => (
          <button
            key={method.label}
            className={`${styles.tab} ${i === activeTab ? styles.tabActive : ""}`}
            onClick={() => { setActiveTab(i); setCopied(false); }}
          >
            {method.label}
          </button>
        ))}
      </div>
      <div className={styles.content}>
        <pre className={styles.command}>
          {installMethods[activeTab].command.split("\n").map((line, i) => (
            <span key={i}>
              <span className={styles.prompt}>$ </span>{line}
              {i < installMethods[activeTab].command.split("\n").length - 1 && "\n"}
            </span>
          ))}
        </pre>
      </div>
      <div className={styles.copyRow}>
        <button
          className={`${styles.copyBtn} ${copied ? styles.copied : ""}`}
          onClick={handleCopy}
        >
          {copied ? (
            <>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
              Copied
            </>
          ) : (
            <>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
              Copy
            </>
          )}
        </button>
      </div>
    </div>
  );
}
