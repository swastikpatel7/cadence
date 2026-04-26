import { cn } from '@/lib/cn';

type PillVariant = 'glass' | 'ghost' | 'solid';

interface PillProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: PillVariant;
  asChild?: boolean;
}

const VARIANTS: Record<PillVariant, string> = {
  glass:
    'bg-white/[0.04] border border-white/10 backdrop-blur-md text-white/80',
  ghost: 'border border-white/10 text-white/70',
  solid: 'bg-white text-black',
};

/**
 * Capsule container. Used for status chips ("Cadence v0.1"), nav rails,
 * keyboard hints, and the pill-style top navigation bar.
 */
export function Pill({
  variant = 'glass',
  className,
  children,
  ...rest
}: PillProps) {
  return (
    <div
      className={cn(
        'inline-flex items-center gap-2 rounded-full px-4 py-1.5 text-sm leading-none',
        VARIANTS[variant],
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}
