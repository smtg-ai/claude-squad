"use client";

import { motion } from "motion/react";
import { ReactNode } from "react";

interface GradientTextProps {
  children: ReactNode;
  className?: string;
  as?: "h1" | "h2" | "span" | "p";
}

export default function GradientText({
  children,
  className,
  as = "span",
}: GradientTextProps) {
  const Tag = motion[as] as typeof motion.span;

  return (
    <Tag
      className={className}
      style={{
        background: "linear-gradient(135deg, #F0A868, #7EC8D8)",
        WebkitBackgroundClip: "text",
        backgroundClip: "text",
        color: "transparent",
        display: "inline-block",
      }}
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.8, ease: "easeOut" }}
    >
      {children}
    </Tag>
  );
}
