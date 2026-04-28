import { UserButton } from '@clerk/nextjs';
import Link from 'next/link';
import { Suspense } from 'react';
import { AppShellNav } from '@/components/app/app-shell-nav';
import { StravaStatusPill } from '@/components/app/strava-status-pill';
import { WordMark } from '@/components/ui/brand-mark';
import { UnitToggle } from '@/components/units/unit-toggle';

/**
 * Signed-in chrome. Quieter than the marketing nav: a single rail at the top
 * of the viewport, no aurora behind, the deep navy bg shows through. The
 * nav links are real app destinations (will fill in as features land).
 *
 * On `/onboarding/*` routes the nav rail is suppressed (via
 * `<AppShellNav>` reading `usePathname()`) — the wizard owns its own
 * progress chrome and shouldn't tempt the user to bounce out mid-flow
 * (insights.md §4).
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

          <AppShellNav />

          <div className="flex items-center gap-3">
            <UnitToggle />
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

