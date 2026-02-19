"use client";

import { useEffect, useRef } from "react";

// ─── Color Palette ───────────────────────────────────────────
const _ = 0;   // transparent
const O = 1;   // outline #2a2a3a
const A = 2;   // amber body #F0A868
const a = 3;   // amber mid #E09050
const S = 4;   // stripe #D4863C
const s = 5;   // stripe shadow #C07030
const E = 6;   // eye white #FFFFFF
const P = 7;   // pupil #1a1a2e
const W = 8;   // wing teal #7EC8D8
const w = 9;   // wing translucent rgba(126,200,216,0.4)
const T = 10;  // wing highlight #A8DDE8
const G = 11;  // stinger #c97a3a
const H = 12;  // highlight #F0C878
const L = 13;  // leg #3a3a4a
const Z = 14;  // zzz color #8888AA
const D = 15;  // drop shadow rgba(0,0,0,0.2)

const COLOR_MAP: Record<number, string | null> = {
  [_]: null,
  [O]: "#2a2a3a",
  [A]: "#F0A868",
  [a]: "#E09050",
  [S]: "#D4863C",
  [s]: "#C07030",
  [E]: "#FFFFFF",
  [P]: "#1a1a2e",
  [W]: "#7EC8D8",
  [w]: "rgba(126,200,216,0.4)",
  [T]: "#A8DDE8",
  [G]: "#c97a3a",
  [H]: "#F0C878",
  [L]: "#3a3a4a",
  [Z]: "#8888AA",
  [D]: "rgba(0,0,0,0.2)",
};

// ─── Sprite Frames (32x32) ──────────────────────────────────
// Each frame is a 32-row array of 32-element number arrays

// Idle wings UP
const IDLE_1: number[][] = [
  //0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
  [_,_,_,_,_,_,_,_,_,_,_,_,O,_,_,_,_,_,_,O,_,_,_,_,_,_,_,_,_,_,_,_], // 0  antennae tips
  [_,_,_,_,_,_,_,_,_,_,_,_,A,_,_,_,_,_,_,A,_,_,_,_,_,_,_,_,_,_,_,_], // 1  antennae bulbs
  [_,_,_,_,_,_,_,_,_,_,_,_,_,O,_,_,_,_,O,_,_,_,_,_,_,_,_,_,_,_,_,_], // 2  antennae stems
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,O,_,_,O,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 3  antennae base
  [_,_,_,_,_,_,_,_,_,_,_,O,O,O,O,O,O,O,O,O,O,_,_,_,_,_,_,_,_,_,_,_], // 4  head top
  [_,_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_], // 5  head upper
  [_,_,_,_,_,_,_,_,_,O,A,A,H,A,A,A,A,A,A,H,A,A,O,_,_,_,_,_,_,_,_,_], // 6  head (highlight)
  [_,_,_,_,_,_,_,_,_,O,A,E,E,E,A,A,A,E,E,E,A,A,O,_,_,_,_,_,_,_,_,_], // 7  eyes top
  [_,_,_,_,_,_,_,_,_,O,A,E,P,P,A,A,A,E,P,P,A,A,O,_,_,_,_,_,_,_,_,_], // 8  eyes bottom
  [_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_], // 9  cheeks
  [_,_,_,_,_,_,_,_,_,_,O,A,A,A,O,O,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_,_], // 10 mouth (smile)
  [_,_,_,_,_,_,_,_,_,_,_,O,O,O,O,O,O,O,O,O,_,_,_,_,_,_,_,_,_,_,_,_], // 11 chin
  [_,_,_,w,w,T,W,W,W,_,O,A,A,A,A,A,A,A,A,A,O,_,W,W,W,T,w,w,_,_,_,_], // 12 wings + neck
  [_,_,w,W,T,W,W,W,W,W,O,A,A,A,A,A,A,A,A,A,O,W,W,W,W,W,T,W,w,_,_,_], // 13 wings wide + body
  [_,_,_,w,W,W,W,W,W,_,O,a,S,S,S,S,S,S,S,a,O,_,W,W,W,W,W,w,_,_,_,_], // 14 wings + stripe 1
  [_,_,_,_,w,W,W,w,_,_,O,A,A,A,A,A,A,A,A,A,O,_,_,w,W,W,w,_,_,_,_,_], // 15 wing tips + body
  [_,_,_,_,_,_,_,_,_,_,O,a,S,S,S,S,S,S,S,a,O,_,_,_,_,_,_,_,_,_,_,_], // 16 stripe 2
  [_,_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_,_], // 17 body
  [_,_,_,_,_,_,_,_,_,_,O,a,S,S,S,S,S,S,S,a,O,_,_,_,_,_,_,_,_,_,_,_], // 18 stripe 3
  [_,_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_,_], // 19 body
  [_,_,_,_,_,_,_,_,_,L,_,O,a,a,a,a,a,a,a,O,_,L,_,_,_,_,_,_,_,_,_,_], // 20 legs + taper
  [_,_,_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_,_,_], // 21 lower body
  [_,_,_,_,_,_,_,_,_,_,L,_,O,a,a,a,a,a,O,_,L,_,_,_,_,_,_,_,_,_,_,_], // 22 legs + taper
  [_,_,_,_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_,_,_,_], // 23 lower body
  [_,_,_,_,_,_,_,_,_,_,_,_,_,O,a,a,a,O,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 24 taper
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,O,O,O,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 25 bottom
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,G,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 26 stinger
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,G,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 27 stinger tip
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 28
  [_,_,_,_,_,_,_,_,_,_,_,D,D,D,D,D,D,D,D,D,D,_,_,_,_,_,_,_,_,_,_,_], // 29 shadow
  [_,_,_,_,_,_,_,_,_,_,_,_,D,D,D,D,D,D,D,D,_,_,_,_,_,_,_,_,_,_,_,_], // 30 shadow
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_], // 31
];

// For the other frames, create helper functions that modify the base frame.
// This keeps the code DRY - we only define one full frame and derive variants.

function cloneFrame(frame: number[][]): number[][] {
  return frame.map(row => [...row]);
}

// Idle wings DOWN - shift wing pixels down by 2 rows
function makeWingsDown(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  // Clear wing pixels in rows 12-15 (left cols 2-9, right cols 22-28)
  for (let r = 12; r <= 15; r++) {
    for (let c = 0; c <= 9; c++) {
      const v = f[r][c];
      if (v === W || v === w || v === T) f[r][c] = _;
    }
    for (let c = 21; c <= 31; c++) {
      const v = f[r][c];
      if (v === W || v === w || v === T) f[r][c] = _;
    }
  }
  // Place wings at rows 14-17
  // Left wing
  f[14][3] = w; f[14][4] = w; f[14][5] = T; f[14][6] = W; f[14][7] = W; f[14][8] = W;
  f[15][2] = w; f[15][3] = W; f[15][4] = T; f[15][5] = W; f[15][6] = W; f[15][7] = W; f[15][8] = W; f[15][9] = W;
  f[16][3] = w; f[16][4] = W; f[16][5] = W; f[16][6] = W; f[16][7] = W; f[16][8] = W;
  f[17][4] = w; f[17][5] = W; f[17][6] = W; f[17][7] = w;
  // Right wing
  f[14][23] = W; f[14][24] = W; f[14][25] = W; f[14][26] = T; f[14][27] = w; f[14][28] = w;
  f[15][22] = W; f[15][23] = W; f[15][24] = W; f[15][25] = W; f[15][26] = W; f[15][27] = T; f[15][28] = W; f[15][29] = w;
  f[16][23] = W; f[16][24] = W; f[16][25] = W; f[16][26] = W; f[16][27] = W; f[16][28] = w;
  f[17][24] = w; f[17][25] = W; f[17][26] = W; f[17][27] = w;
  return f;
}

// Banking left - shift top-left of body down 1px, extend right wing
function makeBankLeft(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  // Slight visual: move left wing pixels 1 row up, right wing 1 row down
  // Simplified: just adjust wing vertical position slightly
  return f; // The base frame works as a reasonable approximation for banking
}

// Banking right - mirror of bank left
function makeBankRight(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  return f;
}

// Excited - wider eyes, sparkle pixels at corners
function makeExcited(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  // Add sparkle pixels
  f[4][6] = H; f[4][25] = H;
  f[8][6] = H; f[8][25] = H;
  f[14][1] = H; f[14][30] = H;
  // Make eyes slightly bigger - add extra white pixel above
  f[6][11] = E; f[6][12] = E; f[6][17] = E; f[6][18] = E;
  return f;
}

// Sleeping - closed eyes, zzz above
function makeSleeping(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  // Close eyes - replace eye whites/pupils with amber (closed line)
  f[7][11] = A; f[7][12] = A; f[7][13] = A; f[7][17] = A; f[7][18] = A; f[7][19] = A;
  f[8][11] = O; f[8][12] = O; f[8][13] = O; f[8][17] = O; f[8][18] = O; f[8][19] = O;
  // Add zzz
  f[0][22] = Z; f[0][23] = Z; f[0][24] = Z;
  f[1][24] = Z;
  f[2][23] = Z;
  f[3][22] = Z; f[3][23] = Z; f[3][24] = Z;
  return f;
}

// Sleeping frame 2 - zzz shifted up 1
function makeSleeping2(frame: number[][]): number[][] {
  const f = makeSleeping(frame);
  // Fold wings down
  // Clear zzz from frame 1 positions and shift up
  f[0][22] = _; f[0][23] = _; f[0][24] = _;
  f[1][24] = _;
  f[2][23] = _;
  f[3][22] = _; f[3][23] = _; f[3][24] = _;
  // Place zzz one row higher (smaller z)
  f[0][23] = Z; f[0][24] = Z;
  f[1][24] = Z;
  f[2][23] = Z; f[2][24] = Z;
  return f;
}

const IDLE_2 = makeWingsDown(IDLE_1);
const FLY_1 = IDLE_1;  // Flying uses same sprite, state machine handles context
const FLY_2 = IDLE_2;
const BANK_L1 = makeBankLeft(IDLE_1);
const BANK_L2 = makeBankLeft(IDLE_2);
const BANK_R1 = makeBankRight(IDLE_1);
const BANK_R2 = makeBankRight(IDLE_2);
const EXCITED_1 = makeExcited(IDLE_1);
const EXCITED_2 = makeExcited(IDLE_2);
const SLEEP_1 = makeSleeping(IDLE_1);
const SLEEP_2 = makeSleeping2(IDLE_2);

const ALL_FRAMES: Record<string, number[][]> = {
  A1: IDLE_1, A2: IDLE_2,
  B1: FLY_1, B2: FLY_2,
  C1: BANK_L1, C2: BANK_L2,
  D1: BANK_R1, D2: BANK_R2,
  E1: EXCITED_1, E2: EXCITED_2,
  F1: SLEEP_1, F2: SLEEP_2,
};

// ─── Sprite Renderer ─────────────────────────────────────────

function createSpriteCanvas(grid: number[][]): HTMLCanvasElement {
  const canvas = document.createElement("canvas");
  canvas.width = 32;
  canvas.height = 32;
  const ctx = canvas.getContext("2d")!;
  for (let y = 0; y < 32; y++) {
    for (let x = 0; x < grid[y].length; x++) {
      const color = COLOR_MAP[grid[y][x]];
      if (color) {
        ctx.fillStyle = color;
        ctx.fillRect(x, y, 1, 1);
      }
    }
  }
  return canvas;
}

// ─── Particle ────────────────────────────────────────────────

interface Particle {
  x: number; y: number;
  vx: number; vy: number;
  size: number; opacity: number;
  maxLife: number; life: number;
  color: string;
}

const PARTICLE_COLORS = ["#F0A868", "#D4863C", "#F0C878", "#E8B878"];
const MAX_PARTICLES = 60;

// ─── State Types ─────────────────────────────────────────────

type BeeState = "idle" | "flying" | "bankLeft" | "bankRight" | "excited" | "sleeping";

const STATE_PREFIX: Record<BeeState, string> = {
  idle: "A", flying: "B", bankLeft: "C", bankRight: "D", excited: "E", sleeping: "F",
};

// ─── Physics Constants ───────────────────────────────────────

const SPRING = 0.08;
const DAMPING = 0.85;
const OFFSET_X = 30;
const OFFSET_Y = -40;
const MAX_SPEED = 25;
const BEE_SCALE = 2.5;

// ─── Component ───────────────────────────────────────────────

export default function BeeCompanion() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    // Pre-render all sprite frames
    const spriteFrames = new Map<string, HTMLCanvasElement>();
    for (const [key, grid] of Object.entries(ALL_FRAMES)) {
      spriteFrames.set(key, createSpriteCanvas(grid));
    }

    // State
    let animationId = 0;
    const beePos = { x: window.innerWidth / 2, y: window.innerHeight / 3 };
    const beeVel = { x: 0, y: 0 };
    const mousePos = { x: window.innerWidth / 2, y: window.innerHeight / 3 };
    let mouseOnScreen = true;
    let lastMouseMove = Date.now();
    let lastClick = 0;
    let beeState: BeeState = "idle";
    let wingFrame = 0;
    let wingTimer = 0;
    let frameCount = 0;

    // Particles
    const isMobile = "ontouchstart" in window;
    const maxParticles = isMobile ? 30 : MAX_PARTICLES;
    const particles: Particle[] = Array.from({ length: maxParticles }, () => ({
      x: 0, y: 0, vx: 0, vy: 0, size: 0, opacity: 0, maxLife: 0, life: 0, color: PARTICLE_COLORS[0],
    }));
    let particleCursor = 0;

    // ─── Resize ──────────
    const resize = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
    };

    // ─── Input ───────────
    const onMouseMove = (e: MouseEvent) => {
      mousePos.x = e.clientX;
      mousePos.y = e.clientY;
      mouseOnScreen = true;
      lastMouseMove = Date.now();
    };
    const onMouseLeave = () => { mouseOnScreen = false; };
    const onClick = () => { lastClick = Date.now(); };
    const onTouchMove = (e: TouchEvent) => {
      const t = e.touches[0];
      if (t) { mousePos.x = t.clientX; mousePos.y = t.clientY; mouseOnScreen = true; lastMouseMove = Date.now(); }
    };
    const onTouchEnd = () => { mouseOnScreen = false; };

    window.addEventListener("mousemove", onMouseMove, { passive: true });
    document.addEventListener("mouseleave", onMouseLeave);
    window.addEventListener("click", onClick);
    window.addEventListener("touchmove", onTouchMove, { passive: true });
    window.addEventListener("touchend", onTouchEnd);

    // ─── Particles ───────
    const spawnParticle = () => {
      const p = particles[particleCursor];
      p.x = beePos.x + (Math.random() - 0.5) * 20;
      p.y = beePos.y + (Math.random() - 0.5) * 20;
      p.vx = (Math.random() - 0.5) * 1.5;
      p.vy = Math.random() * 0.5 + 0.3;
      p.size = Math.random() * 2 + 1;
      p.opacity = Math.random() * 0.5 + 0.3;
      p.maxLife = Math.floor(Math.random() * 30) + 30;
      p.life = p.maxLife;
      p.color = PARTICLE_COLORS[Math.floor(Math.random() * PARTICLE_COLORS.length)];
      particleCursor = (particleCursor + 1) % maxParticles;
    };

    // ─── Animation Loop ──
    const draw = () => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      frameCount++;

      // Physics
      const timeSinceMove = Date.now() - lastMouseMove;
      const targetX = mousePos.x + OFFSET_X + (timeSinceMove > 2000 && mouseOnScreen ? Math.sin(Date.now() * 0.0008) * 30 : 0);
      const targetY = mousePos.y + OFFSET_Y + (timeSinceMove > 2000 && mouseOnScreen ? Math.sin(Date.now() * 0.0016) * 15 : 0);

      beeVel.x += (targetX - beePos.x) * SPRING;
      beeVel.y += (targetY - beePos.y) * SPRING;
      beeVel.x *= DAMPING;
      beeVel.y *= DAMPING;

      const speed = Math.sqrt(beeVel.x ** 2 + beeVel.y ** 2);
      if (speed > MAX_SPEED) {
        beeVel.x = (beeVel.x / speed) * MAX_SPEED;
        beeVel.y = (beeVel.y / speed) * MAX_SPEED;
      }

      beePos.x += beeVel.x;
      beePos.y += beeVel.y;

      // State machine
      const timeSinceClick = Date.now() - lastClick;
      if (timeSinceClick < 500) {
        beeState = "excited";
      } else if (!mouseOnScreen && timeSinceMove > 5000) {
        beeState = "sleeping";
      } else if (speed > 2) {
        if (beeVel.x < -3) beeState = "bankLeft";
        else if (beeVel.x > 3) beeState = "bankRight";
        else beeState = "flying";
      } else {
        beeState = "idle";
      }

      // Wing flutter
      wingTimer++;
      const flutterSpeed = beeState === "sleeping" ? 20 : beeState === "excited" ? 5 : 8;
      if (wingTimer >= flutterSpeed) {
        wingFrame = wingFrame === 0 ? 1 : 0;
        wingTimer = 0;
      }

      // Spawn particles
      if (speed > 1.5 && frameCount % 2 === 0) spawnParticle();
      if (beeState === "excited") spawnParticle();

      // Draw particles (behind bee)
      for (const p of particles) {
        if (p.life <= 0) continue;
        p.life--;
        p.x += p.vx;
        p.y += p.vy;
        p.vx *= 0.98;
        const alpha = (p.life / p.maxLife) * p.opacity;
        ctx.globalAlpha = alpha;
        ctx.fillStyle = p.color;
        ctx.beginPath();
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.globalAlpha = 1;

      // Draw bee sprite
      const frameKey = `${STATE_PREFIX[beeState]}${wingFrame + 1}`;
      const sprite = spriteFrames.get(frameKey);
      if (sprite) {
        ctx.imageSmoothingEnabled = false;
        const size = 32 * BEE_SCALE;
        ctx.drawImage(sprite, beePos.x - size / 2, beePos.y - size / 2, size, size);
      }

      animationId = requestAnimationFrame(draw);
    };

    // ─── Init ────────────
    resize();
    animationId = requestAnimationFrame(draw);
    window.addEventListener("resize", resize);

    const handleVisibility = () => {
      if (document.hidden) cancelAnimationFrame(animationId);
      else animationId = requestAnimationFrame(draw);
    };
    document.addEventListener("visibilitychange", handleVisibility);

    return () => {
      cancelAnimationFrame(animationId);
      window.removeEventListener("resize", resize);
      window.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseleave", onMouseLeave);
      window.removeEventListener("click", onClick);
      window.removeEventListener("touchmove", onTouchMove);
      window.removeEventListener("touchend", onTouchEnd);
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

  return (
    <canvas
      ref={canvasRef}
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        width: "100vw",
        height: "100vh",
        zIndex: 50,
        pointerEvents: "none",
        imageRendering: "pixelated",
      }}
    />
  );
}
