import { cn } from '@/lib/cn';

interface SelectProps
  extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'className'> {
  className?: string;
}

/**
 * Glass-morphic select. Same visual language as GlassCard / Pill —
 * hairline border, subtle inner highlight, mono-feeling caret. Wraps a
 * native <select> so a11y + keyboard nav are free; styling targets the
 * shell only.
 */
export function Select({ className, children, ...rest }: SelectProps) {
  return (
    <div
      className={cn(
        'relative inline-flex items-center',
        'rounded-full border border-white/10 bg-white/[0.04] backdrop-blur-md',
        'shadow-[inset_0_1px_0_0_rgb(255_255_255_/_0.06)]',
        'transition-colors hover:border-white/20',
        'focus-within:ring-2 focus-within:ring-white/20',
        className,
      )}
    >
      <select
        {...rest}
        className="appearance-none bg-transparent pl-4 pr-9 py-2 text-[13px] text-white outline-none cursor-pointer"
      >
        {children}
      </select>
      <svg
        aria-hidden="true"
        width="10"
        height="6"
        viewBox="0 0 10 6"
        className="pointer-events-none absolute right-3 text-white/50"
      >
        <path
          d="M1 1l4 4 4-4"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}
