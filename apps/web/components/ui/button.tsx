import Link from 'next/link';
import { cn } from '@/lib/cn';

type ButtonVariant = 'primary' | 'ghost' | 'strava';
type ButtonSize = 'md' | 'lg';

interface BaseProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  className?: string;
  children: React.ReactNode;
}

type AnchorProps = BaseProps &
  Omit<React.AnchorHTMLAttributes<HTMLAnchorElement>, keyof BaseProps> & {
    href: string;
  };

type ButtonProps = BaseProps &
  Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, keyof BaseProps> & {
    href?: undefined;
  };

const VARIANT: Record<ButtonVariant, string> = {
  primary:
    'bg-white text-black hover:bg-white/90 shadow-[0_8px_24px_-8px_rgb(255_255_255_/_0.35)]',
  ghost:
    'bg-white/[0.04] text-white border border-white/10 hover:bg-white/[0.08] hover:border-white/20 backdrop-blur-md',
  strava:
    'text-white shadow-[0_10px_30px_-10px_oklch(0.68_0.21_45_/_0.55)] hover:brightness-110',
};

const SIZE: Record<ButtonSize, string> = {
  md: 'h-11 px-5 text-sm',
  lg: 'h-14 px-7 text-base',
};

const stravaInline =
  'linear-gradient(180deg, oklch(0.72 0.22 45) 0%, oklch(0.62 0.22 38) 100%)';

const base =
  'group inline-flex items-center justify-center gap-2 rounded-full font-medium tracking-tight transition-all duration-[var(--duration-base)] ease-[var(--ease-out-expo)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40 focus-visible:ring-offset-2 focus-visible:ring-offset-black';

/**
 * CTA button. Two surfaces:
 * - `primary`: white pill (matches Lumen's "Get Lumen / Download for macOS")
 * - `ghost`: glass pill (matches "Browse shaders")
 * - `strava`: orange pill with the brand gradient (matches "Continue with Strava")
 *
 * Renders as `<a>` if `href` is provided, else `<button>`. The trailing arrow
 * slides on hover via group-hover.
 */
export function Button(props: AnchorProps | ButtonProps) {
  const { variant = 'primary', size = 'md', className, children } = props;

  const cls = cn(base, VARIANT[variant], SIZE[size], className);
  const inlineStyle =
    variant === 'strava' ? { backgroundImage: stravaInline } : undefined;

  if ('href' in props && props.href) {
    const { variant: _v, size: _s, className: _c, children: _ch, href, ...rest } = props;
    return (
      <Link href={href} className={cls} style={inlineStyle} {...rest}>
        {children}
      </Link>
    );
  }

  const { variant: _v, size: _s, className: _c, children: _ch, ...rest } =
    props as ButtonProps;
  return (
    <button className={cls} style={inlineStyle} {...rest}>
      {children}
    </button>
  );
}

/** Small chevron used inside CTAs. Slides right on group-hover. */
export function ArrowRight({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 16 16"
      width="14"
      height="14"
      aria-hidden
      className={cn(
        'transition-transform duration-[var(--duration-base)] ease-[var(--ease-out-expo)] group-hover:translate-x-0.5',
        className,
      )}
    >
      <path
        d="M3 8h10M9 4l4 4-4 4"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
