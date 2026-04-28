import { AppShell } from '@/components/app/app-shell';
import { UnitsProvider } from '@/components/units/units-context';
import { serverFetch } from '@/lib/api-server';
import type { Units } from '@/lib/units';

async function loadInitialUnits(): Promise<Units> {
  // First-paint preference. /v1/me/profile lazy-defaults to 'metric'
  // server-side; on any error (Clerk session not ready, transient API
  // hiccup), we fall back to metric so the UI never blocks on this.
  try {
    const res = await serverFetch<{ units: Units }>('/v1/me/profile');
    return res.units;
  } catch {
    return 'metric';
  }
}

export default async function AppRootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const initialUnits = await loadInitialUnits();
  return (
    <UnitsProvider initialUnits={initialUnits}>
      <AppShell>{children}</AppShell>
    </UnitsProvider>
  );
}
