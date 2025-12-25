'use client';

import React, { useState } from 'react';
import styles from './docs.module.css';

type DocType = 'tutorial' | 'howto' | 'reference' | 'explanation';

interface DocSection {
  title: string;
  description: string;
  icon: string;
  color: string;
  examples: string[];
}

const docSections: Record<DocType, DocSection> = {
  tutorial: {
    title: 'Tutorials',
    description: 'Learning-oriented guides that take you step-by-step through using Claude Squad',
    icon: '📚',
    color: '#2196f3',
    examples: [
      'Getting Started with Claude Squad',
      'Creating Your First Multi-Agent Workflow',
      'Managing Concurrent AI Sessions',
    ],
  },
  howto: {
    title: 'How-To Guides',
    description: 'Task-oriented guides that show you how to solve specific problems',
    icon: '🛠',
    color: '#9c27b0',
    examples: [
      'How to Manage Multiple Agents Concurrently',
      'How to Debug Issues with Git Worktrees',
      'How to Configure Auto-Accept Mode',
    ],
  },
  reference: {
    title: 'Reference',
    description: 'Technical descriptions and API documentation for Claude Squad',
    icon: '📖',
    color: '#4caf50',
    examples: [
      'CLI Command Reference',
      'Configuration File Reference',
      'Keyboard Shortcuts Reference',
    ],
  },
  explanation: {
    title: 'Explanation',
    description: 'Understanding-oriented discussions of concepts and design decisions',
    icon: '💡',
    color: '#ff9800',
    examples: [
      'Understanding the Diataxis Framework',
      'Why Git Worktrees for Isolation',
      'The Architecture of Claude Squad',
    ],
  },
};

export default function DocsPage() {
  const [activeTab, setActiveTab] = useState<DocType>('tutorial');

  return (
    <div className={styles.container}>
      <header className={styles.header}>
        <h1>Claude Squad Documentation</h1>
        <p className={styles.subtitle}>
          Organized using the Diataxis framework for better learning and reference
        </p>
      </header>

      <main className={styles.main}>
        <section className={styles.intro}>
          <h2>Four Types of Documentation</h2>
          <p>
            This documentation follows the <strong>Diataxis framework</strong>, which organizes
            content into four distinct types, each serving a different purpose:
          </p>
        </section>

        <div className={styles.grid}>
          {(Object.entries(docSections) as [DocType, DocSection][]).map(([type, section]) => (
            <div
              key={type}
              className={`${styles.card} ${activeTab === type ? styles.active : ''}`}
              onClick={() => setActiveTab(type)}
              style={{ borderColor: section.color }}
            >
              <div className={styles.cardIcon}>{section.icon}</div>
              <h3 style={{ color: section.color }}>{section.title}</h3>
              <p>{section.description}</p>
            </div>
          ))}
        </div>

        <section className={styles.details}>
          <div className={styles.detailsHeader} style={{ borderColor: docSections[activeTab].color }}>
            <span className={styles.detailsIcon}>{docSections[activeTab].icon}</span>
            <h2 style={{ color: docSections[activeTab].color }}>{docSections[activeTab].title}</h2>
          </div>

          <div className={styles.detailsContent}>
            <h3>Example Topics:</h3>
            <ul>
              {docSections[activeTab].examples.map((example, index) => (
                <li key={index}>{example}</li>
              ))}
            </ul>

            <div className={styles.characteristics}>
              <h3>Characteristics:</h3>
              {getCharacteristics(activeTab).map((char, index) => (
                <div key={index} className={styles.characteristic}>
                  <span className={styles.checkmark}>✓</span>
                  <span>{char}</span>
                </div>
              ))}
            </div>

            <div className={styles.cta}>
              <button
                className={styles.button}
                style={{ backgroundColor: docSections[activeTab].color }}
              >
                Browse {docSections[activeTab].title}
              </button>
            </div>
          </div>
        </section>

        <section className={styles.features}>
          <h2>Advanced Documentation Features</h2>
          <div className={styles.featuresGrid}>
            <div className={styles.feature}>
              <h3>⚡ Concurrent Processing</h3>
              <p>Documentation generated using up to 10 concurrent workers for maximum performance</p>
            </div>
            <div className={styles.feature}>
              <h3>🎨 Syntax Highlighting</h3>
              <p>Code examples with beautiful syntax highlighting powered by Chroma</p>
            </div>
            <div className={styles.feature}>
              <h3>🔍 Advanced Validation</h3>
              <p>Automatic validation of structure, cross-references, and Diataxis compliance</p>
            </div>
            <div className={styles.feature}>
              <h3>📊 Quality Metrics</h3>
              <p>Automated quality scoring based on content depth and structure</p>
            </div>
            <div className={styles.feature}>
              <h3>🔗 Cross-References</h3>
              <p>Intelligent linking between related documents across all four types</p>
            </div>
            <div className={styles.feature}>
              <h3>📝 Markdown Support</h3>
              <p>Full GitHub Flavored Markdown with tables, task lists, and more</p>
            </div>
          </div>
        </section>

        <section className={styles.cli}>
          <h2>CLI Documentation Tools</h2>
          <div className={styles.codeBlock}>
            <pre>
              <code>{`# Initialize documentation structure
claude-squad docs init

# Generate documentation site
claude-squad docs generate --workers 10

# Validate all documentation
claude-squad docs validate

# Show statistics
claude-squad docs stats`}</code>
            </pre>
          </div>
        </section>
      </main>

      <footer className={styles.footer}>
        <p>
          Learn more about the Diataxis framework at{' '}
          <a href="https://diataxis.fr" target="_blank" rel="noopener noreferrer">
            diataxis.fr
          </a>
        </p>
      </footer>
    </div>
  );
}

function getCharacteristics(type: DocType): string[] {
  const characteristics: Record<DocType, string[]> = {
    tutorial: [
      'Step-by-step instructions for learning',
      'Concrete, repeatable outcomes',
      'Builds confidence through practice',
      'Friendly and encouraging tone',
    ],
    howto: [
      'Problem-focused and goal-oriented',
      'Assumes basic knowledge',
      'Direct and concise language',
      'Practical solutions to real problems',
    ],
    reference: [
      'Technical and information-oriented',
      'Accurate and complete descriptions',
      'Structured for easy lookup',
      'Neutral and factual tone',
    ],
    explanation: [
      'Understanding-oriented discussions',
      'Provides context and background',
      'Makes conceptual connections',
      'Reflective and thoughtful tone',
    ],
  };

  return characteristics[type];
}
