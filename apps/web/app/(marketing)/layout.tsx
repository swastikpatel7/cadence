import { FloatingNav } from '@/components/marketing/floating-nav';
import { Footer } from '@/components/marketing/footer';

/**
 * Marketing chrome — applied to /, /sign-in, /sign-up, /connect/*. The
 * `relative` + min-h-screen pair lets each page mount its own aurora as a
 * direct child without leaking past section boundaries. The footer renders
 * after the page so it sits beneath any aurora canvas.
 */
export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="relative min-h-screen bg-[var(--color-bg-deep)]">
      <FloatingNav />
      <main className="relative">{children}</main>
      <Footer />
    </div>
  );
}
