'use client';

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { browserFetch } from '@/lib/api-browser';
import type { Units } from '@/lib/units';

const STORAGE_KEY = 'cadence-units-v1';

interface UnitsValue {
  units: Units;
  setUnits: (next: Units) => void;
}

const UnitsContext = createContext<UnitsValue | null>(null);

/**
 * Wraps the signed-in app tree and the onboarding wizard. The
 * `initialUnits` prop is the server-fetched value from
 * `GET /v1/me/profile`, so first paint matches the persisted preference.
 *
 * Mutations are optimistic: we update local state + localStorage
 * synchronously, then PATCH the backend in the background. On PATCH
 * failure we revert and surface a console warning (no user-facing
 * error — the toggle is reversible by definition).
 */
export function UnitsProvider({
  initialUnits,
  children,
}: {
  initialUnits: Units;
  children: ReactNode;
}) {
  const [units, setUnitsState] = useState<Units>(initialUnits);
  const lastPersistedRef = useRef<Units>(initialUnits);

  // Hydrate from localStorage on mount if it disagrees with the server.
  // Covers the case where the user toggled mid-onboarding and a hard
  // refresh raced the server fetch.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (stored === 'metric' || stored === 'imperial') {
        if (stored !== units) setUnitsState(stored);
      }
    } catch {
      // ignore — private mode or quota
    }
    // intentional: only reconcile once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const setUnits = useCallback((next: Units) => {
    setUnitsState((prev) => {
      if (prev === next) return prev;
      try {
        if (typeof window !== 'undefined') {
          window.localStorage.setItem(STORAGE_KEY, next);
        }
      } catch {
        // non-fatal
      }
      const previous = prev;
      void browserFetch('/v1/me/profile', {
        method: 'PATCH',
        body: { units: next },
      })
        .then(() => {
          lastPersistedRef.current = next;
        })
        .catch((err) => {
          console.warn('units: failed to persist preference; reverting', err);
          setUnitsState(previous);
          try {
            if (typeof window !== 'undefined') {
              window.localStorage.setItem(STORAGE_KEY, previous);
            }
          } catch {
            // ignore
          }
        });
      return next;
    });
  }, []);

  return (
    <UnitsContext.Provider value={{ units, setUnits }}>
      {children}
    </UnitsContext.Provider>
  );
}

export function useUnits(): UnitsValue {
  const ctx = useContext(UnitsContext);
  if (!ctx) {
    throw new Error('useUnits must be used inside a <UnitsProvider>');
  }
  return ctx;
}
