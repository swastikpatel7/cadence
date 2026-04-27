import type { ReactNode } from 'react';
import { OnboardingChrome } from '@/components/onboarding/onboarding-chrome';
import { WizardProvider } from '@/components/onboarding/wizard-context';

export const metadata = {
  title: 'Onboarding — Cadence',
};

/**
 * Wizard layout. Wraps every `/onboarding/*` route in the WizardProvider
 * so step state survives navigation across step pages. The
 * `OnboardingChrome` client component owns the cross-fading Aurora pair
 * (per-step variant rotation) + the constellation stepper + escape hatch.
 *
 * Inherits the AppShell from `(app)/layout.tsx`. AppShell hides its own
 * nav links on `/onboarding/*` — see `app-shell-nav.tsx`.
 */
export default function OnboardingLayout({ children }: { children: ReactNode }) {
  return (
    <WizardProvider>
      <OnboardingChrome>{children}</OnboardingChrome>
    </WizardProvider>
  );
}
