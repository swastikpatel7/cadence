import { cn } from '@/lib/cn';

/**
 * Lumen-style kbd hint row. Renders ⌘ + K style chips with a quiet caption.
 * Mono font, tabular numbers, very low contrast — ambient information.
 */
export function KeyHint({
  keys,
  caption,
  className,
}: {
  keys: string[];
  caption: string;
  className?: string;
}) {
  return (
    <div
      className={cn(
        'flex items-center gap-3 text-[12px] uppercase tracking-[0.18em] text-white/35',
        className,
      )}
    >
      <span className="flex items-center gap-1.5">
        {keys.map((k) => (
          <kbd
            key={k}
            className="num inline-flex h-6 min-w-6 items-center justify-center rounded-md border border-white/10 bg-white/[0.03] px-1.5 text-[11px] text-white/70"
          >
            {k}
          </kbd>
        ))}
      </span>
      <span className="font-mono text-[11px] tracking-[0.16em]">{caption}</span>
    </div>
  );
}
