'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { type ReactNode, useCallback } from 'react';
import { CalligraphicUnderline } from '@/components/onboarding/calligraphic-underline';
import { ArrowRight } from '@/components/ui/button';
import { cn } from '@/lib/cn';

interface StepFrameProps {
  /** Small uppercase mono eyebrow above the headline. */
  eyebrow: string;
  /** Headline. JSX so `<span className="display">italic</span>` words are easy. */
  title: ReactNode;
  /** Sub-prompt under the headline. */
  subprompt: ReactNode;
  /** The step body — input controls. */
  children: ReactNode;
  /** Whether the primary CTA is enabled. */
  canContinue: boolean;
  /** Path of the previous step (or null for step 1). */
  backHref?: string | null;
  /** Path of the next step. */
  nextHref: string;
  /** Optional secondary action — e.g. "Skip this step" link. */
  secondaryAction?: ReactNode;
  /** Override the primary CTA label (default: Continue). */
  ctaLabel?: string;
}

/**
 * Common chrome for every onboarding step. Owns the headline +
 * underline + nav buttons; lays out the content area in a single
 * centered column. Continue button fires an aurora ripple from the
 * click point before navigating.
 */
export function StepFrame({
  eyebrow,
  title,
  subprompt,
  children,
  canContinue,
  backHref,
  nextHref,
  secondaryAction,
  ctaLabel = 'Continue',
}: StepFrameProps) {
  const router = useRouter();

  const handleContinue = useCallback(
    (e: React.MouseEvent<HTMLButtonElement>) => {
      if (!canContinue) return;
      const target = e.currentTarget;
      const rect = target.getBoundingClientRect();
      target.style.setProperty('--ripple-x', `${e.clientX - rect.left}px`);
      target.style.setProperty('--ripple-y', `${e.clientY - rect.top}px`);
      target.dataset.rippling = 'true';
      // Let the ripple play for ~280ms before navigating so it
      // doesn't get unmounted mid-bloom.
      window.setTimeout(() => router.push(nextHref), 280);
      window.setTimeout(() => {
        target.removeAttribute('data-rippling');
      }, 720);
    },
    [canContinue, nextHref, router],
  );

  return (
    <div className="mx-auto flex max-w-[760px] flex-col">
      <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/45">
        {eyebrow}
      </p>
      <h1 className="mt-4 max-w-[18ch] text-[40px] font-medium leading-[0.98] tracking-[-0.03em] text-white md:text-[56px]">
        {title}
      </h1>
      <CalligraphicUnderline
        className="mt-2 text-white/65"
        color="currentColor"
        width={220}
        delayMs={500}
      />
      <p className="mt-5 max-w-[58ch] text-[14.5px] leading-[1.55] text-white/55">
        {subprompt}
      </p>

      <div className="mt-10">{children}</div>

      <div className="mt-12 flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          {backHref ? (
            <Link
              href={backHref}
              className="inline-flex h-11 items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-5 text-[13px] text-white/70 backdrop-blur-md transition hover:border-white/20 hover:text-white"
            >
              <span aria-hidden>←</span> Back
            </Link>
          ) : (
            <span className="inline-block h-11" />
          )}
          {secondaryAction}
        </div>

        <RippleCta canContinue={canContinue} onClick={handleContinue} label={ctaLabel} />
      </div>
    </div>
  );
}

function RippleCta({
  canContinue,
  onClick,
  label,
}: {
  canContinue: boolean;
  onClick: (e: React.MouseEvent<HTMLButtonElement>) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!canContinue}
      className={cn(
        'group relative inline-flex h-11 items-center gap-2 overflow-hidden rounded-full px-5 text-[13.5px] font-medium tracking-tight transition disabled:cursor-not-allowed',
        canContinue
          ? 'bg-white text-black shadow-[0_8px_24px_-8px_rgb(255_255_255_/_0.35)] hover:bg-white/90'
          : 'bg-white/10 text-white/40',
      )}
      style={
        {
          // Custom properties consumed by the ::after rule below.
          '--ripple-x': '50%',
          '--ripple-y': '50%',
        } as React.CSSProperties
      }
    >
      <style>{`
        button[data-rippling="true"]::after {
          content: '';
          position: absolute;
          left: var(--ripple-x);
          top: var(--ripple-y);
          width: 60px;
          height: 60px;
          border-radius: 9999px;
          background: radial-gradient(closest-side,
            oklch(0.55 0.27 295 / 0.55) 0%,
            oklch(0.72 0.22 45 / 0.35) 55%,
            transparent 75%);
          transform: translate(-50%, -50%) scale(0);
          pointer-events: none;
          animation: ripple-bloom 700ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
          mix-blend-mode: screen;
        }
      `}</style>
      <span className="relative z-10">{label}</span>
      <ArrowRight className="relative z-10" />
    </button>
  );
}
