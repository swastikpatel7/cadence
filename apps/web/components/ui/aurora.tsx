import { cn } from '@/lib/cn';
import { AuroraShader, type AuroraVariant } from './aurora-shader';
import { Noise } from './noise';

type AuroraIntensity = 'subtle' | 'normal' | 'vivid';

interface AuroraProps {
  variant?: AuroraVariant;
  intensity?: AuroraIntensity;
  /** Viewport-relative brightness centre. Defaults per variant. */
  focus?: [number, number];
  /** Per-second drift on the noise field. Defaults per variant. */
  wind?: [number, number];
  /** Noise scale; <1 zooms in. Default 1.35. */
  scale?: number;
  /** Time-speed multiplier. */
  speed?: number;
  className?: string;
  /** Fix the canvas to the viewport (full-page treatments). */
  fixed?: boolean;
  /** Disable the CSS fallback gradient (useful when the shader is
   *  stacked on a section with its own background). */
  noFallback?: boolean;
}

/* Cheap CSS fallback so SSR + no-WebGL still see something close.
   These are static; the shader covers them once it mounts. */
const FALLBACK_BG: Record<AuroraVariant, string> = {
  violet: [
    'radial-gradient(80% 60% at 70% 35%, oklch(0.55 0.27 295 / 0.55) 0%, transparent 60%)',
    'radial-gradient(70% 60% at 25% 70%, oklch(0.62 0.22 220 / 0.45) 0%, transparent 60%)',
    'radial-gradient(60% 50% at 50% 90%, oklch(0.65 0.24 330 / 0.40) 0%, transparent 60%)',
    'linear-gradient(180deg, oklch(0.07 0.02 270) 0%, oklch(0.07 0.02 270) 100%)',
  ].join(', '),
  marble: [
    'radial-gradient(80% 60% at 30% 30%, oklch(0.78 0.18 105 / 0.55) 0%, transparent 60%)',
    'radial-gradient(70% 60% at 70% 65%, oklch(0.55 0.20 230 / 0.55) 0%, transparent 60%)',
    'radial-gradient(60% 50% at 50% 90%, oklch(0.50 0.22 280 / 0.35) 0%, transparent 60%)',
    'linear-gradient(180deg, oklch(0.07 0.02 270) 0%, oklch(0.07 0.02 270) 100%)',
  ].join(', '),
  strava: [
    'radial-gradient(80% 60% at 50% 40%, oklch(0.72 0.22 45 / 0.55) 0%, transparent 60%)',
    'radial-gradient(70% 60% at 25% 70%, oklch(0.55 0.20 30 / 0.50) 0%, transparent 60%)',
    'radial-gradient(60% 50% at 80% 80%, oklch(0.45 0.18 280 / 0.45) 0%, transparent 60%)',
    'linear-gradient(180deg, oklch(0.07 0.02 270) 0%, oklch(0.07 0.02 270) 100%)',
  ].join(', '),
};

/* Per-variant default focus + wind + scale. Calibrated against the
   Lumen reference set: violet sits at low scale (large soft features,
   sparse wisps), marble at higher scale (small marbling cells), strava
   in between. Wind is intentionally slow — anything faster than ~0.015
   reads as restless rather than meditative. */
const DEFAULTS: Record<
  AuroraVariant,
  { focus: [number, number]; wind: [number, number]; scale: number }
> = {
  violet: { focus: [0.72, 0.42], wind: [-0.013, 0.005], scale: 0.95 },
  marble: { focus: [0.50, 0.55], wind: [0.011, -0.004], scale: 1.40 },
  strava: { focus: [0.50, 0.55], wind: [0.009, 0.008], scale: 1.10 },
};

const INTENSITY_NUM: Record<AuroraIntensity, number> = {
  subtle: 0.78,
  normal: 1.05,
  vivid: 1.32,
};

/**
 * Aurora — composes a WebGL2 shader on top of a CSS gradient fallback.
 * The CSS layer is what users see during SSR, on no-WebGL browsers, or
 * during the first ~50ms before the shader hydrates and starts drawing.
 */
export function Aurora({
  variant = 'violet',
  intensity = 'normal',
  focus,
  wind,
  scale,
  speed = 1.0,
  className,
  fixed = false,
  noFallback = false,
}: AuroraProps) {
  const defaults = DEFAULTS[variant];
  const numericIntensity = INTENSITY_NUM[intensity];

  return (
    <div
      aria-hidden
      className={cn(
        'pointer-events-none isolate overflow-hidden',
        fixed ? 'fixed inset-0 -z-10' : 'absolute inset-0',
        className,
      )}
    >
      {/* Static fallback — visible while the shader compiles, and on
          environments without WebGL2. */}
      {!noFallback ? (
        <div
          className="absolute inset-0"
          style={{ background: FALLBACK_BG[variant] }}
        />
      ) : null}

      {/* The real thing. */}
      <AuroraShader
        variant={variant}
        intensity={numericIntensity}
        speed={speed}
        focus={focus ?? defaults.focus}
        wind={wind ?? defaults.wind}
        scale={scale ?? defaults.scale}
      />

      {/* Filmic grain — sells the GPU look. */}
      <Noise />
    </div>
  );
}

export type { AuroraVariant };
