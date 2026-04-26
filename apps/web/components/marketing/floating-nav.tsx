import { Show, SignInButton, UserButton } from '@clerk/nextjs';
import Link from 'next/link';
import { ArrowRight } from '@/components/ui/button';
import { WordMark } from '@/components/ui/brand-mark';

/**
 * Floating pill nav. One capsule containing brand on the left and auth /
 * actions on the right. Hovers slightly above the page; the aurora's top
 * vignette pushes color underneath so it reads against any section.
 *
 * Auth state is rendered with Clerk's <Show>, so we serve the same nav for
 * both signed-in and signed-out users without client-side flicker.
 */
export function FloatingNav() {
  return (
    <div className="pointer-events-none fixed inset-x-0 top-4 z-50 flex justify-center px-4">
      <nav
        className="pointer-events-auto flex w-full max-w-[1180px] items-center justify-between gap-4 rounded-full border border-white/10 bg-black/40 px-4 py-2 backdrop-blur-xl shadow-[inset_0_1px_0_0_rgb(255_255_255_/_0.06),0_20px_60px_-20px_rgb(0_0_0_/_0.6)]"
        aria-label="Primary"
      >
        <Link
          href="/"
          className="group inline-flex items-baseline rounded-full px-2 py-1 transition-colors hover:bg-white/[0.03]"
        >
          <WordMark size={17} className="text-white/95" />
        </Link>

        <div className="hidden items-center gap-1 md:flex">
          <NavLink href="/#strava">Strava</NavLink>
          <NavLink href="/#coach">Coach</NavLink>
          <NavLink href="/#privacy">Privacy</NavLink>
        </div>

        <div className="flex items-center gap-1.5">
          <Show when="signed-out">
            <SignInButton mode="redirect">
              <button className="hidden h-9 items-center rounded-full px-4 text-[13.5px] text-white/80 transition-colors hover:text-white sm:inline-flex">
                Sign in
              </button>
            </SignInButton>
            <Link
              href="/sign-up"
              className="group inline-flex h-9 items-center gap-1.5 rounded-full bg-white pl-4 pr-3.5 text-[13.5px] font-medium text-black transition-all duration-[var(--duration-base)] hover:bg-white/90"
            >
              Get started
              <ArrowRight />
            </Link>
          </Show>
          <Show when="signed-in">
            <Link
              href="/home"
              className="hidden h-9 items-center rounded-full px-4 text-[13.5px] text-white/80 transition-colors hover:text-white sm:inline-flex"
            >
              Home
            </Link>
            <UserButton
              appearance={{
                elements: {
                  avatarBox: 'h-8 w-8 ring-1 ring-white/15',
                },
              }}
            />
          </Show>
        </div>
      </nav>
    </div>
  );
}

function NavLink({
  href,
  children,
}: {
  href: string;
  children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className="rounded-full px-3 py-1.5 text-[13.5px] text-white/65 transition-colors hover:bg-white/[0.04] hover:text-white"
    >
      {children}
    </Link>
  );
}
