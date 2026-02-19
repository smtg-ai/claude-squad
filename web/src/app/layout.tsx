import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  userScalable: true,
  themeColor: "#0a0b14",
};

export const metadata: Metadata = {
  title: "Hivemind - Agent-Driven IDE for AI Pair Programming",
  description:
    "A TUI-based agent-driven IDE that manages multiple AI agents (Claude Code, Codex, Aider, Gemini) in isolated workspaces, so you can work on multiple tasks simultaneously.",
  keywords: [
    "hivemind", "tui", "ai", "ide", "agent", "terminal", "tmux",
    "claude code", "codex", "aider", "pair programming",
  ],
  authors: [{ name: "ByteMirror" }],
  openGraph: {
    title: "Hivemind",
    description:
      "A TUI-based agent-driven IDE for managing multiple AI agents in isolated workspaces",
    url: "https://github.com/ByteMirror/hivemind",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Hivemind",
    description:
      "A TUI-based agent-driven IDE for managing multiple AI agents in isolated workspaces",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${geistSans.variable} ${geistMono.variable}`}>
        {children}
      </body>
    </html>
  );
}
