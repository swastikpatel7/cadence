import Link from 'next/link';
import { WordMark } from '@/components/ui/brand-mark';

/**
 * Quiet bottom rail. Wordmark + version + a few links in mono. Not the bottom
 * marble CTA — that's a separate section. This is the very last strip on the
 * page, the one that says "an honest product".
 */
export function Footer() {
  return (
    <footer className="relative z-20 border-t border-white/[0.06] bg-[var(--color-bg-deep)]">
      <div className="mx-auto flex max-w-[1180px] flex-col items-start gap-6 px-6 py-10 text-[12.5px] text-white/45 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <WordMark size={15} className="text-white/70" />
          <span className="font-mono tracking-[0.14em]">
            v0.1 · BUILT IN PUBLIC
          </span>
        </div>
        <div className="flex items-center gap-6 font-mono uppercase tracking-[0.14em]">
          <Link href="/#privacy" className="transition-colors hover:text-white">
            Privacy
          </Link>
          <Link href="/sign-in" className="transition-colors hover:text-white">
            Sign in
          </Link>
          <a
            href="https://strava.com"
            target="_blank"
            rel="noreferrer noopener"
            className="transition-colors hover:text-white"
          >
            Strava
          </a>
        </div>
      </div>
    </footer>
  );
}
