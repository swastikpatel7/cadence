'use client';

import { useEffect, useRef } from 'react';

/* ─────────────────────────────────────────────────────────────────
   AuroraShader — WebGL2 ink-in-water aurora.

   Aesthetic target (Lumen-class):
     • Most of the canvas rests at deep-navy bg. Aurora "blooms"
       in pockets driven by a noise-presence threshold, so dark
       breathing space is the default state, not the exception.
     • Where colour appears it transitions buttery-smooth between
       stops via wide, overlapping smoothsteps — no chrome ridges,
       no slimy metallic texture.
     • Slow directional drift via `wind`; never fast enough to
       feel busy.

   Technique:
     • Single-pass domain warp on top of a 4-octave fbm of 3D
       simplex noise (Ashima Arts / Stefan Gustavson, public
       domain). Faster amplitude rolloff per octave (×0.42) so
       big features dominate and small detail is suppressed —
       this is the main fix versus the previous build, which
       went two-pass + ×0.5 rolloff and produced chrome-like
       high-frequency filaments.
     • Per-variant `threshold` controls how much of the canvas
       sits below the aurora floor (sparse hero look) vs how
       much fills with marbling (Get-Lumen-style footer).
     • Aspect-corrected radial vignette around `focus` pulls
       corners back to bg.
     • ACES filmic tone-map keeps highlights from clipping.

   Render strategy:
     • Single full-screen quad, WebGL2.
     • devicePixelRatio capped at 1.5.
     • Paused on tab-hidden, off-screen (IntersectionObserver),
       and prefers-reduced-motion (single static frame).

   Fail-soft: if WebGL2 is unavailable or compile/link fails the
   canvas renders nothing and the parent's CSS gradient fallback
   shows through.
   ───────────────────────────────────────────────────────────────── */

export type AuroraVariant = 'violet' | 'marble' | 'strava';

interface PaletteVec3 {
  bg: [number, number, number];
  c1: [number, number, number];
  c2: [number, number, number];
  c3: [number, number, number];
  c4: [number, number, number];
  /** [lo, hi] aurora-presence smoothstep. Lower = fuller coverage. */
  threshold: [number, number];
}

const PALETTES: Record<AuroraVariant, PaletteVec3> = {
  // Hero / sign-in / sign-up. Indigo → violet → cyan → soft cyan-white
  // tip. Sparse threshold so dark navy dominates and wisps read like
  // smoke against the void (matches the Lumen hero reference).
  violet: {
    bg: [0.025, 0.025, 0.070],
    c1: [0.16, 0.09, 0.52],
    c2: [0.60, 0.28, 0.96],
    c3: [0.42, 0.85, 0.97],
    c4: [0.86, 0.94, 1.00],
    threshold: [0.46, 0.86],
  },
  // "Get started" footer. Deep blue → teal → lime → warm yellow.
  // Lower threshold = full coverage so the whole strip marbles like
  // Lumen's "Get Lumen" reference; vignette handles the corners.
  marble: {
    bg: [0.018, 0.038, 0.095],
    c1: [0.07, 0.20, 0.56],
    c2: [0.10, 0.55, 0.76],
    c3: [0.72, 0.90, 0.44],
    c4: [0.98, 0.86, 0.32],
    threshold: [0.28, 0.80],
  },
  // Strava connect. Deep indigo → red-orange → orange → amber. Medium
  // threshold so the GlassCard sits on a softly marbled field with
  // visible dark gutters.
  strava: {
    bg: [0.038, 0.028, 0.070],
    c1: [0.28, 0.10, 0.42],
    c2: [0.85, 0.22, 0.18],
    c3: [0.99, 0.56, 0.22],
    c4: [0.99, 0.86, 0.40],
    threshold: [0.40, 0.82],
  },
};

const VERT = `#version 300 es
in vec2 a_pos;
void main() {
  gl_Position = vec4(a_pos, 0.0, 1.0);
}`;

const FRAG = `#version 300 es
precision highp float;

uniform vec2  u_res;
uniform float u_time;
uniform float u_intensity;
uniform float u_scale;
uniform vec2  u_focus;        // 0..1 viewport coords; brightness centre
uniform vec2  u_wind;         // per-second drift on the noise field
uniform vec2  u_threshold;    // aurora-presence smoothstep (lo, hi)
uniform vec3  u_bg;
uniform vec3  u_c1, u_c2, u_c3, u_c4;

out vec4 outColor;

/* Simplex 3D noise — Ashima Arts / Stefan Gustavson, public domain. */
vec4 permute(vec4 x) { return mod(((x * 34.0) + 1.0) * x, 289.0); }
vec4 taylorInvSqrt(vec4 r) { return 1.79284291400159 - 0.85373472095314 * r; }

float snoise(vec3 v) {
  const vec2 C = vec2(1.0/6.0, 1.0/3.0);
  const vec4 D = vec4(0.0, 0.5, 1.0, 2.0);
  vec3 i  = floor(v + dot(v, C.yyy));
  vec3 x0 = v - i + dot(i, C.xxx);
  vec3 g  = step(x0.yzx, x0.xyz);
  vec3 l  = 1.0 - g;
  vec3 i1 = min(g.xyz, l.zxy);
  vec3 i2 = max(g.xyz, l.zxy);
  vec3 x1 = x0 - i1 + C.xxx;
  vec3 x2 = x0 - i2 + C.yyy;
  vec3 x3 = x0 - D.yyy;
  i = mod(i, 289.0);
  vec4 p = permute(permute(permute(
            i.z + vec4(0.0, i1.z, i2.z, 1.0))
          + i.y + vec4(0.0, i1.y, i2.y, 1.0))
          + i.x + vec4(0.0, i1.x, i2.x, 1.0));
  float n_ = 0.142857142857;
  vec3 ns = n_ * D.wyz - D.xzx;
  vec4 j  = p - 49.0 * floor(p * ns.z * ns.z);
  vec4 x_ = floor(j * ns.z);
  vec4 y_ = floor(j - 7.0 * x_);
  vec4 x  = x_ * ns.x + ns.yyyy;
  vec4 y  = y_ * ns.x + ns.yyyy;
  vec4 h  = 1.0 - abs(x) - abs(y);
  vec4 b0 = vec4(x.xy, y.xy);
  vec4 b1 = vec4(x.zw, y.zw);
  vec4 s0 = floor(b0) * 2.0 + 1.0;
  vec4 s1 = floor(b1) * 2.0 + 1.0;
  vec4 sh = -step(h, vec4(0.0));
  vec4 a0 = b0.xzyw + s0.xzyw * sh.xxyy;
  vec4 a1 = b1.xzyw + s1.xzyw * sh.zzww;
  vec3 p0 = vec3(a0.xy, h.x);
  vec3 p1 = vec3(a0.zw, h.y);
  vec3 p2 = vec3(a1.xy, h.z);
  vec3 p3 = vec3(a1.zw, h.w);
  vec4 norm = taylorInvSqrt(vec4(dot(p0, p0), dot(p1, p1), dot(p2, p2), dot(p3, p3)));
  p0 *= norm.x; p1 *= norm.y; p2 *= norm.z; p3 *= norm.w;
  vec4 m = max(0.6 - vec4(dot(x0, x0), dot(x1, x1), dot(x2, x2), dot(x3, x3)), 0.0);
  m = m * m;
  return 42.0 * dot(m * m, vec4(dot(p0, x0), dot(p1, x1), dot(p2, x2), dot(p3, x3)));
}

/* 4-octave fbm with aggressive amplitude rolloff (×0.42). The fast
   rolloff mutes high-frequency detail, which is what reads as smoke
   instead of chrome. */
float fbm(vec3 p) {
  float v = 0.0;
  float a = 0.55;
  for (int i = 0; i < 4; i++) {
    v += a * snoise(p);
    p *= 2.05;
    a *= 0.42;
  }
  return v;
}

/* ACES filmic tone-map — soft highlight rolloff. */
vec3 aces(vec3 x) {
  return clamp((x * (2.51 * x + 0.03)) / (x * (2.43 * x + 0.59) + 0.14),
               0.0, 1.0);
}

void main() {
  vec2 fragUv = gl_FragCoord.xy / u_res;                         // 0..1
  vec2 uv     = (gl_FragCoord.xy - 0.5 * u_res) / u_res.y;        // aspect, centre 0

  // Slow directional drift; gives the field a calm wind without
  // making motion feel busy.
  vec2 windOff = u_wind * u_time;

  vec3 q = vec3(uv * u_scale + windOff, u_time * 0.025);

  // Single-pass domain warp. Two-pass produced chrome filaments;
  // a single warp at moderate magnitude (×2.0) gives soft ribbon
  // shapes without metallic ridges.
  vec3 r = vec3(
    fbm(q + vec3(1.7, 9.2, 0.0)),
    fbm(q + vec3(8.3, 2.8, 0.0)),
    0.0
  );
  float n = fbm(q + 2.0 * r);
  n = clamp(n * 0.70 + 0.5, 0.0, 1.0);

  // Aurora "presence" — values below threshold.x stay at bg, values
  // above threshold.y are full-strength aurora. Wide gap = soft fade.
  // Per-variant threshold lets violet read sparse and marble fill.
  float aurora = smoothstep(u_threshold.x, u_threshold.y, n);

  // Internal colour ramp — overlapping wide smoothsteps so the
  // transitions read as ink-in-water bleeds, not hard banded ridges.
  vec3 ramp = u_c1;
  ramp = mix(ramp, u_c2, smoothstep(0.45, 0.74, n));
  ramp = mix(ramp, u_c3, smoothstep(0.66, 0.90, n));
  ramp = mix(ramp, u_c4, smoothstep(0.84, 0.97, n));

  vec3 col = mix(u_bg, ramp, aurora);

  // Asymmetric vignette around focus. Aspect-corrected so falloff
  // is circular regardless of canvas shape.
  vec2 d = (fragUv - u_focus);
  d.x *= u_res.x / u_res.y;
  float dist = length(d);
  float vign = smoothstep(0.95, 0.18, dist);
  col = mix(u_bg, col, vign);

  col *= u_intensity;
  col = aces(col);

  outColor = vec4(col, 1.0);
}`;

interface AuroraShaderProps {
  variant: AuroraVariant;
  /** 0.7 (subtle) → 1.0 (normal) → 1.3 (vivid). */
  intensity?: number;
  /** Time speed multiplier; 1.0 is the natural drift. */
  speed?: number;
  /** Brightness centre, viewport-relative [0..1, 0..1]. Default centre. */
  focus?: [number, number];
  /** Per-second drift of the noise field. Default subtle drift. */
  wind?: [number, number];
  /** Noise scale; <1 zooms in (bigger features), >1 zooms out. */
  scale?: number;
  className?: string;
}

export function AuroraShader({
  variant,
  intensity = 1.0,
  speed = 1.0,
  focus = [0.5, 0.5],
  wind = [0.010, 0.005],
  scale = 1.0,
  className,
}: AuroraShaderProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const gl = canvas.getContext('webgl2', {
      alpha: false,
      antialias: false,
      premultipliedAlpha: false,
      powerPreference: 'high-performance',
    });
    if (!gl) return;

    const compile = (type: number, src: string) => {
      const sh = gl.createShader(type);
      if (!sh) return null;
      gl.shaderSource(sh, src);
      gl.compileShader(sh);
      if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
        // eslint-disable-next-line no-console
        console.warn('AuroraShader compile error:', gl.getShaderInfoLog(sh));
        gl.deleteShader(sh);
        return null;
      }
      return sh;
    };

    const vs = compile(gl.VERTEX_SHADER, VERT);
    const fs = compile(gl.FRAGMENT_SHADER, FRAG);
    if (!vs || !fs) return;

    const program = gl.createProgram();
    if (!program) return;
    gl.attachShader(program, vs);
    gl.attachShader(program, fs);
    gl.linkProgram(program);
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      // eslint-disable-next-line no-console
      console.warn('AuroraShader link error:', gl.getProgramInfoLog(program));
      return;
    }
    gl.useProgram(program);

    // Full-screen triangle pair.
    const quad = new Float32Array([
      -1, -1, 1, -1, -1, 1,
      -1, 1, 1, -1, 1, 1,
    ]);
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, quad, gl.STATIC_DRAW);
    const aPos = gl.getAttribLocation(program, 'a_pos');
    gl.enableVertexAttribArray(aPos);
    gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

    const uRes = gl.getUniformLocation(program, 'u_res');
    const uTime = gl.getUniformLocation(program, 'u_time');
    const uIntensity = gl.getUniformLocation(program, 'u_intensity');
    const uScale = gl.getUniformLocation(program, 'u_scale');
    const uFocus = gl.getUniformLocation(program, 'u_focus');
    const uWind = gl.getUniformLocation(program, 'u_wind');
    const uThreshold = gl.getUniformLocation(program, 'u_threshold');
    const uBg = gl.getUniformLocation(program, 'u_bg');
    const uC1 = gl.getUniformLocation(program, 'u_c1');
    const uC2 = gl.getUniformLocation(program, 'u_c2');
    const uC3 = gl.getUniformLocation(program, 'u_c3');
    const uC4 = gl.getUniformLocation(program, 'u_c4');

    const palette = PALETTES[variant];
    gl.uniform3fv(uBg, palette.bg);
    gl.uniform3fv(uC1, palette.c1);
    gl.uniform3fv(uC2, palette.c2);
    gl.uniform3fv(uC3, palette.c3);
    gl.uniform3fv(uC4, palette.c4);
    gl.uniform2f(uThreshold, palette.threshold[0], palette.threshold[1]);
    gl.uniform1f(uIntensity, intensity);
    gl.uniform1f(uScale, scale);
    gl.uniform2f(uFocus, focus[0], focus[1]);
    gl.uniform2f(uWind, wind[0], wind[1]);

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
      const rect = canvas.getBoundingClientRect();
      const w = Math.max(1, Math.floor(rect.width * dpr));
      const h = Math.max(1, Math.floor(rect.height * dpr));
      if (canvas.width !== w || canvas.height !== h) {
        canvas.width = w;
        canvas.height = h;
      }
      gl.viewport(0, 0, w, h);
      gl.uniform2f(uRes, w, h);
    };
    resize();

    const ro = new ResizeObserver(resize);
    ro.observe(canvas);

    let visible = true;
    const io = new IntersectionObserver(
      ([entry]) => {
        visible = !!entry?.isIntersecting;
      },
      { threshold: 0 },
    );
    io.observe(canvas);

    const reduceMotion = window.matchMedia(
      '(prefers-reduced-motion: reduce)',
    ).matches;

    let raf = 0;
    const start = performance.now();
    const draw = () => {
      const t = ((performance.now() - start) / 1000) * speed;
      gl.uniform1f(uTime, t);
      gl.drawArrays(gl.TRIANGLES, 0, 6);
    };

    if (reduceMotion) {
      draw();
    } else {
      const loop = () => {
        if (visible && !document.hidden) draw();
        raf = requestAnimationFrame(loop);
      };
      raf = requestAnimationFrame(loop);
    }

    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
      io.disconnect();
      gl.deleteProgram(program);
      gl.deleteShader(vs);
      gl.deleteShader(fs);
      gl.deleteBuffer(buf);
    };
  }, [variant, intensity, speed, focus, wind, scale]);

  return (
    <canvas
      ref={canvasRef}
      aria-hidden
      className={className ?? 'absolute inset-0 h-full w-full'}
      style={{ display: 'block' }}
    />
  );
}
