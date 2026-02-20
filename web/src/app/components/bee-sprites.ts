// ─── Shared Bee Sprite Data ─────────────────────────────────
// Single source of truth for the 32x32 pixel art bee.
// Used by both BeeCompanion (interactive) and PixelBee (static hero/header).

// Color keys
export const _ = 0;   // transparent
export const O = 1;   // outline #2a2a3a
export const A = 2;   // amber body #F0A868
export const a = 3;   // amber mid #E09050
export const S = 4;   // stripe #5C3D1E
export const W = 8;   // wing teal #7EC8D8
export const w = 9;   // wing translucent rgba(126,200,216,0.4)
export const T = 10;  // wing highlight #A8DDE8
export const G = 11;  // stinger #c97a3a
export const H = 12;  // highlight #F0C878
export const L = 13;  // leg #3a3a4a
export const D = 15;  // drop shadow rgba(0,0,0,0.2)

// Used only in variant frames
const E = 6;   // eye white
const P = 7;   // pupil
const Z = 14;  // zzz color

export const COLOR_MAP: Record<number, string | null> = {
  [_]: null,
  [O]: "#2a2a3a",
  [A]: "#F0A868",
  [a]: "#E09050",
  [S]: "#5C3D1E",
  5: "#C07030",
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

// ─── Base Sprite: Idle wings UP (32x32) ─────────────────────
export const IDLE_1: number[][] = [
  [_,_,_,_,_,_,_,_,_,_,_,_,O,_,_,_,_,_,_,O,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,A,_,_,_,_,_,_,A,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,O,_,_,_,_,O,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,O,_,_,O,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,O,O,O,O,O,O,O,O,O,O,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,O,A,A,H,A,A,A,A,A,A,H,A,A,O,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,O,A,E,E,E,E,A,A,E,E,E,E,A,O,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,O,A,E,P,E,E,A,A,E,P,E,E,A,O,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,O,A,E,P,P,E,A,A,E,P,P,E,A,O,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,O,A,A,A,O,O,A,A,A,A,O,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,O,O,O,O,O,O,O,O,O,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,w,w,T,W,W,W,O,A,A,A,A,A,A,A,A,A,A,A,A,A,A,O,W,W,W,T,w,w,_,_],
  [_,w,W,T,W,W,W,O,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,O,W,W,W,T,W,w,_],
  [_,_,w,W,W,W,w,O,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,O,w,W,W,W,w,_,_],
  [_,_,_,_,w,W,w,O,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,O,w,W,w,_,_,_,_],
  [_,_,_,_,_,_,_,O,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,O,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,O,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,O,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,O,S,S,S,S,S,S,S,S,S,S,S,S,S,S,S,O,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,O,A,A,A,A,A,A,A,A,A,A,A,A,O,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,O,a,a,a,a,a,a,a,a,a,a,O,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,O,O,O,O,O,O,O,O,O,O,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,G,G,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,D,D,D,D,D,D,D,D,D,D,D,D,D,D,D,D,D,D,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,D,D,D,D,D,D,D,D,D,D,D,D,D,D,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
  [_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_,_],
];

// ─── Frame Variant Helpers ──────────────────────────────────

function cloneFrame(frame: number[][]): number[][] {
  return frame.map(row => [...row]);
}

export function makeWingsDown(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  // Clear wings-up positions (rows 12-15)
  for (let r = 12; r <= 15; r++) {
    for (let c = 0; c <= 7; c++) {
      const v = f[r][c];
      if (v === W || v === w || v === T) f[r][c] = _;
    }
    for (let c = 24; c <= 31; c++) {
      const v = f[r][c];
      if (v === W || v === w || v === T) f[r][c] = _;
    }
  }
  // Left wing (down position, rows 16-18)
  f[16][1] = w; f[16][2] = T; f[16][3] = W; f[16][4] = W; f[16][5] = W; f[16][6] = W;
  f[17][2] = w; f[17][3] = W; f[17][4] = W; f[17][5] = W; f[17][6] = w;
  f[18][3] = w; f[18][4] = W; f[18][5] = W; f[18][6] = w;
  // Right wing (down position, rows 16-18)
  f[16][25] = W; f[16][26] = W; f[16][27] = W; f[16][28] = W; f[16][29] = T; f[16][30] = w;
  f[17][25] = w; f[17][26] = W; f[17][27] = W; f[17][28] = W; f[17][29] = w;
  f[18][25] = w; f[18][26] = W; f[18][27] = W; f[18][28] = w;
  return f;
}

export function makeExcited(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  f[4][6] = H; f[4][25] = H;
  f[8][6] = H; f[8][25] = H;
  f[14][1] = H; f[14][30] = H;
  f[6][11] = E; f[6][12] = E; f[6][13] = E; f[6][14] = E; f[6][17] = E; f[6][18] = E; f[6][19] = E; f[6][20] = E;
  return f;
}

export function makeSleeping(frame: number[][]): number[][] {
  const f = cloneFrame(frame);
  f[7][11] = A; f[7][12] = A; f[7][13] = A; f[7][14] = A; f[7][17] = A; f[7][18] = A; f[7][19] = A; f[7][20] = A;
  f[8][11] = A; f[8][12] = A; f[8][13] = A; f[8][14] = A; f[8][17] = A; f[8][18] = A; f[8][19] = A; f[8][20] = A;
  f[9][11] = O; f[9][12] = O; f[9][13] = O; f[9][14] = O; f[9][17] = O; f[9][18] = O; f[9][19] = O; f[9][20] = O;
  f[0][22] = Z; f[0][23] = Z; f[0][24] = Z;
  f[1][24] = Z;
  f[2][23] = Z;
  f[3][22] = Z; f[3][23] = Z; f[3][24] = Z;
  return f;
}

export function makeSleeping2(frame: number[][]): number[][] {
  const f = makeSleeping(frame);
  f[0][22] = _; f[0][23] = _; f[0][24] = _;
  f[1][24] = _;
  f[2][23] = _;
  f[3][22] = _; f[3][23] = _; f[3][24] = _;
  f[0][23] = Z; f[0][24] = Z;
  f[1][24] = Z;
  f[2][23] = Z; f[2][24] = Z;
  return f;
}

// ─── Pre-computed Frames ────────────────────────────────────

export const IDLE_2 = makeWingsDown(IDLE_1);

export const ALL_FRAMES: Record<string, number[][]> = {
  A1: IDLE_1, A2: IDLE_2,
  B1: IDLE_1, B2: IDLE_2,         // flying = same sprite, context differs
  C1: IDLE_1, C2: IDLE_2,         // bankLeft
  D1: IDLE_1, D2: IDLE_2,         // bankRight
  E1: makeExcited(IDLE_1), E2: makeExcited(IDLE_2),
  F1: makeSleeping(IDLE_1), F2: makeSleeping2(IDLE_2),
};

// ─── Sprite Canvas Creator ──────────────────────────────────

export function createSpriteCanvas(grid: number[][]): HTMLCanvasElement {
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
