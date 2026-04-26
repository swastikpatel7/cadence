import Link from 'next/link';
import { Aurora } from '@/components/ui/aurora';
import { GlassCard } from '@/components/ui/glass-card';

/**
 * Shared shell for /sign-in and /sign-up. Aurora canvas behind a centered
 * glass card. Side rail offers a light editorial pull-quote so the page is
 * not just "form on aurora" — it sells the product even at the front door.
 */
export function AuthShell({
  eyebrow,
  title,
  pullquoteTitle,
  pullquoteBody,
  children,
  back,
}: {
  eyebrow: string;
  title: React.ReactNode;
  pullquoteTitle: string;
  pullquoteBody: string;
  children: React.ReactNode;
  back?: { href: string; label: string };
}) {
  return (
    <section className="relative min-h-[100svh] overflow-hidden">
      {/* Aurora pushed to the left so it dominates the pull-quote
          half of the page; the form card on the right reads cleanly
          against deeper navy. */}
      <Aurora
        variant="violet"
        intensity="normal"
        focus={[0.25, 0.5]}
        wind={[0.012, -0.005]}
        scale={0.90}
      />
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            'linear-gradient(90deg, transparent 0%, transparent 40%, oklch(0.07 0.02 270 / 0.65) 75%, oklch(0.07 0.02 270 / 0.92) 100%)',
        }}
      />

      <div className="relative z-20 mx-auto flex min-h-[100svh] max-w-[1180px] flex-col px-6 pb-12 pt-28 md:pt-32">
        {back ? (
          <Link
            href={back.href}
            className="mb-8 inline-flex w-max items-center gap-1.5 rounded-full border border-white/10 bg-black/30 px-3.5 py-1.5 text-[12.5px] text-white/70 backdrop-blur-md transition-colors hover:bg-white/[0.06] hover:text-white"
          >
            <span aria-hidden>←</span> {back.label}
          </Link>
        ) : null}

        <div className="grid flex-1 grid-cols-1 items-center gap-10 md:grid-cols-[1.05fr_minmax(0,440px)] md:gap-14">
          <div className="hidden md:block">
            <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/40">
              {eyebrow}
            </p>
            <h1 className="mt-5 text-[64px] font-medium leading-[0.98] tracking-[-0.03em] text-white">
              {title}
            </h1>

            <figure className="mt-12 max-w-[44ch] border-l border-white/15 pl-5 text-white/70">
              <blockquote className="display text-[26px] leading-[1.2] text-white/90">
                "{pullquoteTitle}"
              </blockquote>
              <figcaption className="mt-4 font-mono text-[11px] uppercase tracking-[0.18em] text-white/45">
                {pullquoteBody}
              </figcaption>
            </figure>
          </div>

          <GlassCard className="px-7 py-8 md:px-10 md:py-10">{children}</GlassCard>
        </div>

        <p className="mt-12 text-center font-mono text-[11px] tracking-[0.16em] text-white/35">
          PROTECTED BY CLERK · TLS 1.3 · NEVER SHARED
        </p>
      </div>
    </section>
  );
}
