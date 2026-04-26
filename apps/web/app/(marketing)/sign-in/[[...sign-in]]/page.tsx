import { SignIn } from '@clerk/nextjs';
import { AuthShell } from '@/components/marketing/auth-shell';
import { clerkAppearance } from '@/components/marketing/clerk-appearance';

export const metadata = {
  title: 'Sign in — Cadence',
};

export default function SignInPage() {
  return (
    <AuthShell
      back={{ href: '/', label: 'Back to home' }}
      eyebrow="WELCOME BACK"
      title={
        <>
          Pick up where
          <br />
          <span className="display">you left off.</span>
        </>
      }
      pullquoteTitle="The first product I've trusted with my actual training data."
      pullquoteBody="— A user, eventually"
    >
      <SignIn appearance={clerkAppearance} />
    </AuthShell>
  );
}
