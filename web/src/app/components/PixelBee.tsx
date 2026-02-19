"use client";

import { useState, useEffect, useMemo } from "react";
import styles from "./PixelBee.module.css";

interface PixelBeeProps {
  scale?: number;
  bob?: boolean;
  className?: string;
}

// Color palette
const _ = null;                      // transparent
const O = "#2a2a3a";                 // outline (dark)
const A = "#F0A868";                 // amber body
const S = "#D4863C";                 // dark amber stripe
const E = "#FFFFFF";                 // eye white
const P = "#1a1a2e";                 // pupil
const W = "#7EC8D8";                 // wing teal
const T = "rgba(126,200,216,0.5)";   // wing translucent
const G = "#c97a3a";                 // stinger

// Frame 1: wings in normal (up) position
const frame1: (string | null)[][] = [
  //0  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15
  [_, _, _, _, _, O, _, _, _, _, O, _, _, _, _, _], // row 0: antennae tips
  [_, _, _, _, _, _, O, _, _, O, _, _, _, _, _, _], // row 1: antennae stems
  [_, _, _, _, _, O, A, O, O, A, O, _, _, _, _, _], // row 2: top of head
  [_, _, _, _, O, A, A, A, A, A, A, O, _, _, _, _], // row 3: head upper
  [_, _, _, _, O, E, P, A, A, E, P, O, _, _, _, _], // row 4: eyes row
  [_, _, _, _, O, A, A, O, O, A, A, O, _, _, _, _], // row 5: mouth/smile
  [_, _, T, W, O, A, A, A, A, A, A, O, W, T, _, _], // row 6: wings + upper body
  [_, T, W, T, O, O, A, A, A, A, O, O, T, W, T, _], // row 7: wings + body
  [_, _, _, _, O, S, S, S, S, S, S, O, _, _, _, _], // row 8: dark stripe 1
  [_, _, _, _, O, A, A, A, A, A, A, O, _, _, _, _], // row 9: amber band
  [_, _, _, _, O, S, S, S, S, S, S, O, _, _, _, _], // row 10: dark stripe 2
  [_, _, _, _, _, O, A, A, A, A, O, _, _, _, _, _], // row 11: tapering body
  [_, _, _, _, _, O, A, A, A, A, O, _, _, _, _, _], // row 12: lower body
  [_, _, _, _, _, _, O, O, O, O, _, _, _, _, _, _], // row 13: body bottom
  [_, _, _, _, _, _, _, O, O, _, _, _, _, _, _, _], // row 14: stinger base
  [_, _, _, _, _, _, _, G, G, _, _, _, _, _, _, _], // row 15: stinger tip
];

// Frame 2: wings shifted down 1px (flutter effect)
const frame2: (string | null)[][] = [
  //0  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15
  [_, _, _, _, _, O, _, _, _, _, O, _, _, _, _, _], // row 0: antennae tips
  [_, _, _, _, _, _, O, _, _, O, _, _, _, _, _, _], // row 1: antennae stems
  [_, _, _, _, _, O, A, O, O, A, O, _, _, _, _, _], // row 2: top of head
  [_, _, _, _, O, A, A, A, A, A, A, O, _, _, _, _], // row 3: head upper
  [_, _, _, _, O, E, P, A, A, E, P, O, _, _, _, _], // row 4: eyes row
  [_, _, _, _, O, A, A, O, O, A, A, O, _, _, _, _], // row 5: mouth/smile
  [_, _, _, _, O, A, A, A, A, A, A, O, _, _, _, _], // row 6: body (no wings this row)
  [_, _, T, W, O, O, A, A, A, A, O, O, W, T, _, _], // row 7: wings shifted down + body
  [_, T, W, T, O, S, S, S, S, S, S, O, T, W, T, _], // row 8: wings + dark stripe 1
  [_, _, _, _, O, A, A, A, A, A, A, O, _, _, _, _], // row 9: amber band
  [_, _, _, _, O, S, S, S, S, S, S, O, _, _, _, _], // row 10: dark stripe 2
  [_, _, _, _, _, O, A, A, A, A, O, _, _, _, _, _], // row 11: tapering body
  [_, _, _, _, _, O, A, A, A, A, O, _, _, _, _, _], // row 12: lower body
  [_, _, _, _, _, _, O, O, O, O, _, _, _, _, _, _], // row 13: body bottom
  [_, _, _, _, _, _, _, O, O, _, _, _, _, _, _, _], // row 14: stinger base
  [_, _, _, _, _, _, _, G, G, _, _, _, _, _, _, _], // row 15: stinger tip
];

function gridToShadow(grid: (string | null)[][]): string {
  const shadows: string[] = [];
  for (let y = 0; y < grid.length; y++) {
    for (let x = 0; x < grid[y].length; x++) {
      const color = grid[y][x];
      if (color) {
        shadows.push(`${x}px ${y}px 0 ${color}`);
      }
    }
  }
  return shadows.join(",");
}

export default function PixelBee({ scale = 4, bob = true, className }: PixelBeeProps) {
  const [wingUp, setWingUp] = useState<boolean>(true);

  useEffect(() => {
    const interval = setInterval(() => {
      setWingUp((prev) => !prev);
    }, 150);
    return () => clearInterval(interval);
  }, []);

  const shadow1 = useMemo(() => gridToShadow(frame1), []);
  const shadow2 = useMemo(() => gridToShadow(frame2), []);

  const shadow = wingUp ? shadow1 : shadow2;

  return (
    <div
      className={`${styles.beeContainer} ${bob ? styles.bob : ""} ${styles.hoverGlow} ${className || ""}`}
      style={{
        width: 16 * scale,
        height: 16 * scale,
      }}
    >
      <div
        className={styles.pixel}
        style={{
          boxShadow: shadow,
          transform: `scale(${scale})`,
          transformOrigin: "top left",
        }}
      />
    </div>
  );
}
