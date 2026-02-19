"use client";

import { useEffect, useRef } from "react";
import { ALL_FRAMES, createSpriteCanvas } from "./bee-sprites";

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
