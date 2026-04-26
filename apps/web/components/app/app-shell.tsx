import { UserButton } from '@clerk/nextjs';
import Link from 'next/link';
import { Suspense } from 'react';
import { StravaStatusPill } from '@/components/app/strava-status-pill';
import { WordMark } from '@/components/ui/brand-mark';

/**
 * Signed-in chrome. Quieter than the marketing nav: a single rail at the top
 * of the viewport, no aurora behind, the deep navy bg shows through. The
 * nav links are real app destinations (will fill in as features land).
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative min-h-screen bg-[var(--color-bg-deep)]">
      <header className="sticky top-0 z-40 border-b border-white/[0.06] bg-[oklch(0.07_0.02_270_/_0.7)] backdrop-blur-xl">
        <div className="mx-auto flex h-14 max-w-[1280px] items-center justify-between px-6">
          <Link
            href="/home"
            className="group inline-flex items-baseline rounded-full px-2 py-1 transition-colors hover:bg-white/[0.03]"
          >
            <WordMark size={17} className="text-white/95" />
          </Link>

          <nav className="hidden items-center gap-1 md:flex" aria-label="App">
            <AppNavLink href="/home" active>
              Today
            </AppNavLink>
            <AppNavLink href="/home#activities">Activities</AppNavLink>
            <AppNavLink href="/home#coach">Coach</AppNavLink>
            <AppNavLink href="/settings">Settings</AppNavLink>
          </nav>

          <div className="flex items-center gap-3">
            <Suspense fallback={null}>
              <StravaStatusPill />
            </Suspense>
            <UserButton
              appearance={{
                elements: {
                  avatarBox: 'h-8 w-8 ring-1 ring-white/15',
                },
              }}
            />
          </div>
        </div>
      </header>
      <main>{children}</main>
    </div>
  );
}

function AppNavLink({
  href,
  active,
  children,
}: {
  href: string;
  active?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className={
        active
          ? 'rounded-full border border-white/10 bg-white/[0.06] px-3.5 py-1.5 text-[13px] text-white'
          : 'rounded-full px-3.5 py-1.5 text-[13px] text-white/55 transition-colors hover:bg-white/[0.03] hover:text-white'
      }
    >
      {children}
    </Link>
  );
}
