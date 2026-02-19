"use client";

import { useEffect, useState } from "react";
import styles from "./Header.module.css";

export default function Header() {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 50);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <header className={`${styles.header} ${scrolled ? styles.scrolled : ""}`}>
      <a href="#" className={styles.logo}>
        hivemind
      </a>
      <nav className={styles.nav}>
        <a
          href="https://github.com/ByteMirror/hivemind"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.navLink}
        >
          GitHub
        </a>
        <a
          href="https://github.com/ByteMirror/hivemind#readme"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.navLink}
        >
          Docs
        </a>
        <a
          href="https://github.com/ByteMirror/hivemind/releases"
          target="_blank"
          rel="noopener noreferrer"
          className={styles.navLink}
        >
          Releases
        </a>
      </nav>
    </header>
  );
}
