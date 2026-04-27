import { redirect } from 'next/navigation';
import { ApiError, type UserGoal } from '@/lib/api-client';
import { serverFetch } from '@/lib/api-server';

/**
 * Onboarding root — decides whether to send the user into the wizard
 * or back to /home. Per insights.md §4.1:
 *   - 200 (goal exists)  → re-onboarding goes through Settings; bounce.
 *   - 404 (no goal)      → start at step 1.
 *
 * The redirect must run *outside* the try/catch — Next.js redirect
 * throws a `NEXT_REDIRECT` sentinel that we mustn't swallow.
 */
export default async function OnboardingRoot() {
  let hasGoal = false;
  try {
    await serverFetch<{ goal: UserGoal }>('/v1/me/goal');
    hasGoal = true;
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      hasGoal = false;
    } else {
      // Backend unreachable / other error — let the user proceed into
      // the wizard. They'll resolve the actual issue at submit time.
      hasGoal = false;
    }
  }
  if (hasGoal) {
    redirect('/home');
  }
  redirect('/onboarding/focus');
}
