'use client';

import { useUnits } from '@/components/units/units-context';
import type { Units } from '@/lib/units';

/**
 * Two-pill segmented toggle: `mi | km`. Lives in the top-right of the
 * AppShell header (visible on every signed-in route, including the
 * onboarding wizard). Clicking either pill optimistically updates the
 * UnitsContext and PATCHes /v1/me/profile.
 */
export function UnitToggle({ className }: { className?: string }) {
  const { units, setUnits } = useUnits();

  return (
    <div
      role="radiogroup"
      aria-label="Distance units"
      className={[
        'inline-flex h-7 items-center rounded-full border border-white/10 bg-white/[0.04] p-0.5 backdrop-blur-md',
        className ?? '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <Pill active={units === 'imperial'} onSelect={() => setUnits('imperial')} value="imperial">
        mi
      </Pill>
      <Pill active={units === 'metric'} onSelect={() => setUnits('metric')} value="metric">
        km
      </Pill>
    </div>
  );
}

function Pill({
  active,
  value,
  onSelect,
  children,
}: {
  active: boolean;
  value: Units;
  onSelect: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={active}
      aria-label={value === 'imperial' ? 'Miles' : 'Kilometers'}
      onClick={onSelect}
      className={
        active
          ? 'h-6 rounded-full bg-white px-2.5 font-mono text-[10.5px] uppercase tracking-[0.18em] text-black transition-colors'
          : 'h-6 rounded-full px-2.5 font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/60 transition-colors hover:text-white'
      }
    >
      {children}
    </button>
  );
}
