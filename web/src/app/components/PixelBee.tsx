"use client";

import { useEffect, useRef, useState } from "react";
import styles from "./PixelBee.module.css";
import { IDLE_1, IDLE_2, createSpriteCanvas } from "./bee-sprites";

interface PixelBeeProps {
  scale?: number;
  bob?: boolean;
  className?: string;
}

export default function PixelBee({ scale = 3, bob = true, className }: PixelBeeProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [wingUp, setWingUp] = useState(true);
  const sprite1Ref = useRef<HTMLCanvasElement | null>(null);
  const sprite2Ref = useRef<HTMLCanvasElement | null>(null);
  const blinkCounterRef = useRef(0);
  const blinkActiveRef = useRef(0);
  const nextBlinkRef = useRef(20 + Math.floor(Math.random() * 10));

  // Pre-render sprites once on mount
  useEffect(() => {
    sprite1Ref.current = createSpriteCanvas(IDLE_1);
    sprite2Ref.current = createSpriteCanvas(IDLE_2);
  }, []);

  // Wing flutter
  useEffect(() => {
    const interval = setInterval(() => setWingUp((prev) => !prev), 150);
    return () => clearInterval(interval);
  }, []);

  // Draw current frame to visible canvas
  useEffect(() => {
    const canvas = canvasRef.current;
    const sprite = wingUp ? sprite1Ref.current : sprite2Ref.current;
    if (!canvas || !sprite) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    // Blink timing (piggybacks on wing flutter ticks, ~150ms each)
    blinkCounterRef.current++;
    if (blinkActiveRef.current > 0) {
      blinkActiveRef.current--;
    } else if (blinkCounterRef.current >= nextBlinkRef.current) {
      blinkActiveRef.current = 2; // blink for 2 ticks (~300ms)
      blinkCounterRef.current = 0;
      nextBlinkRef.current = 20 + Math.floor(Math.random() * 10); // 3-4.5s
    }

    ctx.clearRect(0, 0, 32, 32);
    ctx.imageSmoothingEnabled = false;
    ctx.drawImage(sprite, 0, 0);

    // Draw closed eyes overlay when blinking
    if (blinkActiveRef.current > 0) {
      ctx.fillStyle = "#F0A868";
      ctx.fillRect(11, 7, 4, 2);
      ctx.fillRect(17, 7, 4, 2);
      ctx.fillStyle = "#2a2a3a";
      ctx.fillRect(11, 9, 4, 1);
      ctx.fillRect(17, 9, 4, 1);
    }
  }, [wingUp]);

  return (
    <div
      className={`${styles.beeContainer} ${bob ? styles.bob : ""} ${styles.hoverGlow} ${className || ""}`}
      style={{
        width: 32 * scale,
        height: 32 * scale,
      }}
    >
      <canvas
        ref={canvasRef}
        width={32}
        height={32}
        style={{
          width: 32 * scale,
          height: 32 * scale,
          imageRendering: "pixelated",
        }}
      />
    </div>
  );
}
