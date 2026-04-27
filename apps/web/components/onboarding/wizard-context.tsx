'use client';

import {
  createContext,
  type Dispatch,
  type ReactNode,
  useContext,
  useEffect,
  useReducer,
} from 'react';
import type { GoalFocus } from '@/lib/api-client';

const STORAGE_KEY = 'cadence-onboarding-v1';

export interface WizardState {
  focus: GoalFocus | null;
  weekly_miles_target: number | null;
  days_per_week: number | null;
  target_distance_km: number | null;
  target_pace_sec_per_km: number | null;
  race_date: string | null;
}

const INITIAL: WizardState = {
  focus: null,
  weekly_miles_target: null,
  days_per_week: null,
  target_distance_km: null,
  target_pace_sec_per_km: null,
  race_date: null,
};

export type WizardAction =
  | { type: 'SET_FIELD'; field: keyof WizardState; value: WizardState[keyof WizardState] }
  | { type: 'HYDRATE'; payload: WizardState }
  | { type: 'RESET' };

function reducer(state: WizardState, action: WizardAction): WizardState {
  switch (action.type) {
    case 'SET_FIELD':
      return { ...state, [action.field]: action.value };
    case 'HYDRATE':
      return action.payload;
    case 'RESET':
      return INITIAL;
    default:
      return state;
  }
}

interface ContextValue {
  state: WizardState;
  dispatch: Dispatch<WizardAction>;
}

const WizardContext = createContext<ContextValue | null>(null);

/**
 * React context + reducer holding the four-step wizard state.
 * Persisted to `sessionStorage` keyed `cadence-onboarding-v1` so a
 * hard refresh (or a midway browser-tab switch) resumes cleanly.
 *
 * Hydration is client-only — the reducer initializes to `INITIAL` on
 * first render so server-rendered HTML matches client-rendered HTML;
 * the `useEffect` below reads sessionStorage and dispatches `HYDRATE`
 * before the user can interact.
 */
export function WizardProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, INITIAL);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      const raw = window.sessionStorage.getItem(STORAGE_KEY);
      if (!raw) return;
      const parsed = JSON.parse(raw) as Partial<WizardState>;
      dispatch({ type: 'HYDRATE', payload: { ...INITIAL, ...parsed } });
    } catch {
      // ignore — corrupt storage just resets the wizard.
    }
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      window.sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      // private mode / quota — non-fatal, the wizard still works in-memory.
    }
  }, [state]);

  return (
    <WizardContext.Provider value={{ state, dispatch }}>{children}</WizardContext.Provider>
  );
}

export function useWizard(): ContextValue {
  const ctx = useContext(WizardContext);
  if (!ctx) {
    throw new Error('useWizard must be used inside a <WizardProvider>');
  }
  return ctx;
}

export function clearWizardStorage() {
  if (typeof window === 'undefined') return;
  try {
    window.sessionStorage.removeItem(STORAGE_KEY);
  } catch {
    // ignore
  }
}
