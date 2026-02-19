"use client";

import styles from "../page.module.css";
import StarField from "./StarField";
import GradientText from "./GradientText";
import TypewriterText from "./TypewriterText";
import ScrollReveal from "./ScrollReveal";
import FeatureCard from "./FeatureCard";
import InstallTabs from "./InstallTabs";
import Header from "./Header";
import BeeCompanion from "./BeeCompanion";

const features = [
  {
    icon: "\u{1F916}",
    title: "Multi-Agent Management",
    description:
      "Run Claude Code, Codex, Aider, and Gemini CLI side-by-side. Supervise all your AI agents from a single terminal UI.",
  },
  {
    icon: "\u{1F500}",
    title: "Isolated Git Workspaces",
    description:
      "Each agent works in its own git worktree. No conflicts, no overwrites. Merge when you're ready.",
  },
  {
    icon: "\u{1F50D}",
    title: "Live Preview & Diff",
    description:
      "See real-time diffs of what your agents are changing. Review every line before it hits your codebase.",
  },
  {
    icon: "\u{1F4BE}",
    title: "Session Persistence",
    description:
      "Sessions survive restarts. Pick up where you left off, even after rebooting your machine.",
  },
  {
    icon: "\u{1F680}",
    title: "Auto-commit & PR",
    description:
      "Automatically commit agent work and create pull requests. Ship faster with less manual overhead.",
  },
  {
    icon: "\u{1F310}",
    title: "Universal Agent Support",
    description:
      "Works with any CLI-based AI agent. If it runs in a terminal, Hivemind can manage it.",
  },
];

const typewriterTexts = [
  "Supervise multiple AI agents at once",
  "Ship features 10x faster",
  "Review diffs before merging",
  "Isolated workspaces, zero conflicts",
];

export default function PageContent() {
  return (
    <div className={styles.page}>
      <StarField />
      <BeeCompanion />

      <div className={`${styles.glowOrb} ${styles.glowAmber}`} />
      <div className={`${styles.glowOrb} ${styles.glowTeal}`} />

      <div className={styles.content}>
        <Header />

        {/* Hero */}
        <section className={styles.hero}>
          <GradientText as="h1" className={styles.heroTitle}>
            hivemind
          </GradientText>
          <p className={styles.heroSubtitle}>
            The agent-driven IDE for your terminal
          </p>
          <div className={styles.heroTypewriter}>
            <TypewriterText texts={typewriterTexts} />
          </div>
          <div className={styles.heroCtas}>
            <a href="#install" className={styles.ctaPrimary}>
              Install Now
            </a>
            <a
              href="https://github.com/ByteMirror/hivemind"
              target="_blank"
              rel="noopener noreferrer"
              className={styles.ctaSecondary}
            >
              View on GitHub
            </a>
          </div>
        </section>

        {/* Demo Video */}
        <ScrollReveal className={styles.videoSection}>
          <div className={styles.videoWrapper}>
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
        </ScrollReveal>

        {/* Features */}
        <section className={styles.section}>
          <ScrollReveal>
            <h2 className={styles.sectionTitle}>Why Hivemind?</h2>
            <p className={styles.sectionSubtitle}>
              Everything you need to manage multiple AI coding agents from one
              place.
            </p>
          </ScrollReveal>
          <div className={styles.featuresGrid}>
            {features.map((feature, i) => (
              <ScrollReveal key={feature.title} delay={i * 0.1}>
                <FeatureCard {...feature} />
              </ScrollReveal>
            ))}
          </div>
        </section>

        {/* Installation */}
        <section id="install" className={styles.section}>
          <ScrollReveal className={styles.installSection}>
            <h2 className={styles.sectionTitle}>Get Started</h2>
            <p className={styles.sectionSubtitle}>
              Install Hivemind in seconds. Works on macOS, Linux, and Windows.
            </p>
            <InstallTabs />
            <p className={styles.installPrereqs}>
              Prerequisites: tmux, gh (GitHub CLI)
            </p>
          </ScrollReveal>
        </section>

        {/* Footer */}
        <footer className={styles.footer}>
          <div className={styles.footerGradientLine} />
          <p className={styles.footerText}>
            &copy; {new Date().getFullYear()} Hivemind by{" "}
            <a
              href="https://github.com/ByteMirror"
              target="_blank"
              rel="noopener noreferrer"
            >
              ByteMirror
            </a>
            . Licensed under{" "}
            <a
              href="https://github.com/ByteMirror/hivemind/blob/main/LICENSE.md"
              target="_blank"
              rel="noopener noreferrer"
            >
              GNU AGPL v3.0
            </a>
          </p>
        </footer>
      </div>
    </div>
  );
}
