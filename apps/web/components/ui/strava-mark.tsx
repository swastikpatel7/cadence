/**
 * Strava chevron — the stylized "S" / chevron-stack mark used by Strava.
 * Drawn from the brand's official path so the geometry is right; rendered
 * in `currentColor` so we can theme it (white on orange, etc.).
 */
export function StravaMark({ size = 22 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      aria-hidden
      role="img"
    >
      <path
        d="M9.6 2 4 12.3h3.6L9.6 8.5l1.9 3.8h3.6L9.6 2Zm5.5 10.3-1.7 3.4-1.7-3.4H8.5l3.9 7.7 3.9-7.7h-1.2Z"
        fill="currentColor"
      />
    </svg>
  );
}
