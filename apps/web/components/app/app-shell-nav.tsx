'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

/**
 * Top-nav links for the AppShell. Hidden on `/onboarding/*` routes
 * — the wizard owns its own progress chrome (constellation stepper)
 * and we don't want to tempt the user to bounce out mid-flow
 * (insights.md §4).
 */
export function AppShellNav() {
  const pathname = usePathname();
  if (pathname?.startsWith('/onboarding')) return null;

  const isHome = pathname?.startsWith('/home');
  const isSettings = pathname?.startsWith('/settings');

  return (
    <nav className="hidden items-center gap-1 md:flex" aria-label="App">
      <AppNavLink href="/home" active={isHome}>
        Today
      </AppNavLink>
      <AppNavLink href="/home#activities">Activities</AppNavLink>
      <AppNavLink href="/home#coach">Coach</AppNavLink>
      <AppNavLink href="/settings" active={isSettings}>
        Settings
      </AppNavLink>
    </nav>
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
