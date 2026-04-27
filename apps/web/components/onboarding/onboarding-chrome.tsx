'use client';

import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { type ReactNode, useEffect, useState } from 'react';
import { ConstellationStepper } from '@/components/onboarding/constellation-stepper';
import { clearWizardStorage } from '@/components/onboarding/wizard-context';
import { Aurora, type AuroraVariant } from '@/components/ui/aurora';

type AuroraIntensity = 'subtle' | 'normal' | 'vivid';

interface AuroraSpec {
  variant: AuroraVariant;
  intensity: AuroraIntensity;
  scale: number;
}

const STEP_CHROME: Record<
  string,
  { step: number; aurora: AuroraSpec }
> = {
  '/onboarding/focus':    { step: 1, aurora: { variant: 'marble', intensity: 'normal', scale: 1.40 } },
  '/onboarding/volume':   { step: 2, aurora: { variant: 'violet', intensity: 'normal', scale: 0.90 } },
  '/onboarding/days':     { step: 3, aurora: { variant: 'strava', intensity: 'normal', scale: 1.10 } },
  '/onboarding/target':   { step: 4, aurora: { variant: 'marble', intensity: 'vivid',  scale: 1.40 } },
  '/onboarding/baseline': { step: 5, aurora: { variant: 'violet', intensity: 'vivid',  scale: 0.85 } },
};

const FALLBACK: AuroraSpec = { variant: 'violet', intensity: 'normal', scale: 1.0 };

/**
 * Wizard chrome — stripped top bar (logo lives in AppShell already; we
 * render the constellation stepper + escape link), and a stacked pair
 * of <Aurora> layers whose opacities cross-fade as the user advances
 * through the steps. The "incoming" layer renders the variant for the
 * current path; the "outgoing" layer holds the previous variant during
 * a 600ms transition before being dismissed.
 *
 * Per insights.md §3: variant rotation is marble → violet → strava →
 * marble (vivid) → violet (vivid). The constellation stepper itself
 * pulses on the active step.
 */
export function OnboardingChrome({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname() ?? '/onboarding/focus';
  const current = STEP_CHROME[pathname]?.step ?? 1;

  const targetSpec: AuroraSpec = STEP_CHROME[pathname]?.aurora ?? FALLBACK;
  // Stable string key for the dependency comparison; avoids spec
  // identity churn on every render.
  const specKey = `${targetSpec.variant}-${targetSpec.intensity}-${targetSpec.scale}`;

  // Two aurora layers — `back` is the layer being dismissed, `front` is
  // the one fading in. We swap them on every path change so the React
  // tree for the `front` layer is always fresh (forces a remount of the
  // Aurora canvas, otherwise WebGL won't pick up the new variant).
  const [layers, setLayers] = useState<{
    front: { spec: AuroraSpec; key: number };
    back: { spec: AuroraSpec; key: number } | null;
  }>(() => ({ front: { spec: targetSpec, key: 0 }, back: null }));

  useEffect(() => {
    setLayers((prev) => {
      const prevKey = `${prev.front.spec.variant}-${prev.front.spec.intensity}-${prev.front.spec.scale}`;
      // No change, avoid the cross-fade.
      if (prevKey === specKey) return prev;
      return {
        front: { spec: targetSpec, key: prev.front.key + 1 },
        back: prev.front,
      };
    });
    // After 600ms the back layer is invisible; drop it from the tree so
    // we don't keep an idle WebGL context around.
    const t = window.setTimeout(() => {
      setLayers((prev) => ({ front: prev.front, back: null }));
    }, 700);
    return () => window.clearTimeout(t);
  }, [specKey, targetSpec]);

  // Esc closes back to /home with a confirmation.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Escape') return;
      if (
        window.confirm(
          "We'll save your progress. Continue building your plan later from /home.",
        )
      ) {
        router.push('/home');
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [router]);

  const isBaseline = pathname === '/onboarding/baseline';

  return (
    <section className="relative isolate min-h-[calc(100svh-3.5rem)] overflow-hidden">
      {/* Aurora cross-fade. Two stacked layers, each filling the
          section. The `back` layer fades out over 600ms; the
          `front` layer fades in. */}
      <div className="absolute inset-0 -z-10">
        {layers.back ? (
          <div
            key={`back-${layers.back.key}`}
            className="absolute inset-0 transition-opacity duration-[600ms] ease-out"
            style={{ opacity: 0 }}
          >
            <Aurora
              variant={layers.back.spec.variant}
              intensity={layers.back.spec.intensity}
              scale={layers.back.spec.scale}
            />
          </div>
        ) : null}
        <div
          key={`front-${layers.front.key}`}
          className="absolute inset-0"
          style={{
            animation: 'fade-in 600ms cubic-bezier(0.16, 1, 0.3, 1) both',
          }}
        >
          <Aurora
            variant={layers.front.spec.variant}
            intensity={layers.front.spec.intensity}
            scale={layers.front.spec.scale}
          />
        </div>

        {/* Vertical legibility scrim — keeps the centered card readable
            no matter which aurora variant lands behind it. */}
        <div
          className="absolute inset-0"
          style={{
            background:
              'linear-gradient(180deg, oklch(0.07 0.02 270 / 0.55) 0%, oklch(0.07 0.02 270 / 0.30) 35%, oklch(0.07 0.02 270 / 0.55) 100%)',
          }}
        />
      </div>

      {/* Wizard chrome row. AppShell already shows the cadence wordmark;
          we add the stepper + the close link aligned to the right. */}
      <div className="relative z-10 mx-auto flex max-w-[1080px] items-center justify-between px-6 pt-8 md:pt-10">
        <ConstellationStepper current={current} total={5} />

        {!isBaseline ? (
          <CloseLink onClose={() => clearWizardStorage()} />
        ) : (
          <span className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
            BUILDING YOUR PLAN
          </span>
        )}
      </div>

      <div className="relative z-10 mx-auto max-w-[1080px] px-6 pb-16 pt-8 md:pt-12">
        {children}
      </div>
    </section>
  );
}

function CloseLink({ onClose }: { onClose: () => void }) {
  return (
    <Link
      href="/home"
      onClick={onClose}
      className="group inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.04] px-3 py-1.5 text-[12px] text-white/55 backdrop-blur-md transition-colors hover:border-white/20 hover:text-white"
      aria-label="Close onboarding"
    >
      <span className="font-mono uppercase tracking-[0.18em]">esc</span>
      <CloseIcon />
    </Link>
  );
}

function CloseIcon() {
  return (
    <svg
      aria-hidden="true"
      role="presentation"
      width="10"
      height="10"
      viewBox="0 0 10 10"
    >
      <path
        d="M1.5 1.5l7 7M8.5 1.5l-7 7"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}
