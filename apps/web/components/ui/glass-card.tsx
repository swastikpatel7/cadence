import { cn } from '@/lib/cn';

/**
 * Glass surface. The hairline border + inner highlight + heavy backdrop blur
 * is what gives the card the "lit-from-inside" feel in the references. The
 * inner highlight is faked via a top-loaded box-shadow (1px white at 8%) so we
 * don't need a separate ::before pseudo-element.
 */
export function GlassCard({
  className,
  children,
  ...rest
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'relative overflow-hidden rounded-[var(--radius-card-lg)]',
        'border border-white/10 bg-black/40 backdrop-blur-2xl',
        'shadow-[inset_0_1px_0_0_rgb(255_255_255_/_0.08),0_30px_80px_-30px_rgb(0_0_0_/_0.6)]',
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}
