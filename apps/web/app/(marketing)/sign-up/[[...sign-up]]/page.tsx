import { SignUp } from '@clerk/nextjs';
import { AuthShell } from '@/components/marketing/auth-shell';
import { clerkAppearance } from '@/components/marketing/clerk-appearance';

export const metadata = {
  title: 'Get started — Cadence',
};

export default function SignUpPage() {
  return (
    <AuthShell
      back={{ href: '/', label: 'Back to home' }}
      eyebrow="GET STARTED"
      title={
        <>
          A coach
          <br />
          <span className="display">that earns it.</span>
        </>
      }
      pullquoteTitle="It reads my last fourteen days before it answers. That changes the conversation."
      pullquoteBody="— Cadence design principle #1"
    >
      <SignUp appearance={clerkAppearance} />
    </AuthShell>
  );
}
