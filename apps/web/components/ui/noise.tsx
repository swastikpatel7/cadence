import { cn } from '@/lib/cn';

/**
 * Full-bleed grain overlay. SVG turbulence baseFrequency tuned for a fine,
 * filmic grain — not the chunky dithered look. Sits above content with a
 * very low opacity and `mix-blend-mode: overlay` so it adds texture
 * without flattening the colors beneath.
 */
export function Noise({ className }: { className?: string }) {
  return (
    <div
      aria-hidden
      className={cn(
        'pointer-events-none absolute inset-0 z-10 opacity-[0.06] mix-blend-overlay',
        className,
      )}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='220' height='220'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2' stitchTiles='stitch'/><feColorMatrix values='0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 0.6 0'/></filter><rect width='100%' height='100%' filter='url(%23n)'/></svg>\")",
        backgroundSize: '220px 220px',
      }}
    />
  );
}
