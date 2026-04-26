import { currentUser } from '@clerk/nextjs/server';
import Link from 'next/link';
import { Aurora } from '@/components/ui/aurora';
import { ArrowRight, Button } from '@/components/ui/button';
import { GlassCard } from '@/components/ui/glass-card';
import { StravaMark } from '@/components/ui/strava-mark';

export const metadata = {
  title: 'Today — Cadence',
};

export default async function HomePage() {
  const user = await currentUser();
  const greetName = user?.firstName ?? user?.username ?? 'athlete';

  return (
    <>
      <Hero name={greetName} />
      <PlaceholderGrid />
    </>
  );
}

function Hero({ name }: { name: string }) {
  const today = new Date().toLocaleDateString('en-US', {
    weekday: 'long',
    month: 'long',
    day: 'numeric',
  });

  return (
    <section className="relative overflow-hidden border-b border-white/[0.06]">
      {/* Aurora dominates the right two-thirds; text stays legible
          on the left thanks to the focus offset + a soft left-side
          gradient mask. */}
      <div className="pointer-events-none absolute inset-0">
        <Aurora
          variant="violet"
          intensity="normal"
          focus={[0.80, 0.48]}
          wind={[-0.012, 0.004]}
          scale={0.85}
        />
        {/* Left-side darkening for text legibility. */}
        <div
          className="absolute inset-0"
          style={{
            background:
              'linear-gradient(90deg, oklch(0.07 0.02 270 / 0.90) 0%, oklch(0.07 0.02 270 / 0.55) 35%, oklch(0.07 0.02 270 / 0) 70%)',
          }}
        />
        {/* Soft bottom fade into the next section. */}
        <div
          className="absolute inset-x-0 bottom-0 h-[35%]"
          style={{
            background:
              'linear-gradient(to bottom, transparent 0%, var(--color-bg-deep) 100%)',
          }}
        />
      </div>

      <div className="relative z-10 mx-auto max-w-[1280px] px-6 py-20 md:py-28">
        <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/45">
          {today.toUpperCase()}
        </p>
        <h1 className="mt-4 max-w-[20ch] text-[52px] font-semibold leading-[0.98] tracking-[-0.035em] text-white md:text-[80px]">
          Welcome back,{' '}
          <span className="display font-normal text-white/95">{name}.</span>
        </h1>
        <p className="mt-6 max-w-[58ch] text-[16.5px] leading-[1.55] text-white/65">
          Cadence is empty until your first activity lands. Connect Strava and
          we'll backfill the last 90 days, then keep streaming new sessions
          within sixty seconds.
        </p>
      </div>
    </section>
  );
}

function PlaceholderGrid() {
  return (
    <section className="mx-auto max-w-[1280px] px-6 py-12">
      <div className="grid grid-cols-1 gap-5 lg:grid-cols-[1.4fr_1fr]">
        <ConnectCard />
        <CoachStub />
      </div>

      <div className="mt-5 grid grid-cols-1 gap-5 md:grid-cols-3">
        <StatStub label="WEEKLY VOLUME" hint="awaiting data" />
        <StatStub label="HR ZONES" hint="awaiting data" />
        <StatStub label="RECOVERY" hint="awaiting data" />
      </div>

      <p className="mt-12 text-center font-mono text-[11px] tracking-[0.18em] text-white/30">
        v0.1 · WIRES BACKEND IN PHASE 3 · YOUR DATA, YOUR PACE
      </p>
    </section>
  );
}

function ConnectCard() {
  return (
    <GlassCard className="relative overflow-hidden p-7 md:p-8">
      <div
        className="pointer-events-none absolute -right-20 -top-20 h-72 w-72 rounded-full opacity-50"
        style={{
          background:
            'radial-gradient(closest-side, oklch(0.72 0.22 45 / 0.55) 0%, transparent 70%)',
          filter: 'blur(40px)',
        }}
      />
      <div className="relative">
        <div className="flex items-center gap-3">
          <span
            className="flex h-10 w-10 items-center justify-center rounded-xl text-white"
            style={{
              background:
                'linear-gradient(135deg, oklch(0.72 0.22 45) 0%, oklch(0.58 0.22 38) 100%)',
            }}
            aria-hidden
          >
            <StravaMark size={18} />
          </span>
          <div>
            <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
              STEP 01 · CONNECT
            </p>
            <h3 className="mt-1 text-[22px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
              Pair Strava in one click.
            </h3>
          </div>
        </div>

        <p className="mt-5 max-w-[52ch] text-[14.5px] leading-[1.55] text-white/60">
          Read-only OAuth. We pull your last 90 days of activities, splits, GPS
          streams, heart rate, and power, then keep streaming new sessions as
          you upload them.
        </p>

        <div className="mt-6 flex flex-wrap items-center gap-3">
          <Button href="/connect/strava" variant="strava">
            <StravaMark size={16} />
            Connect Strava
            <ArrowRight />
          </Button>
          <Link
            href="/#privacy"
            className="text-[13.5px] text-white/55 underline-offset-4 hover:text-white hover:underline"
          >
            What does Cadence read?
          </Link>
        </div>
      </div>
    </GlassCard>
  );
}

function CoachStub() {
  return (
    <GlassCard className="flex flex-col p-7 md:p-8">
      <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/45">
        STEP 02 · COACH
      </p>
      <h3 className="mt-3 text-[22px] font-medium leading-[1.1] tracking-[-0.02em] text-white">
        Your coach, asleep.
      </h3>
      <p className="mt-3 text-[14px] leading-[1.55] text-white/55">
        The coach wakes up after your first activity syncs. It reads your last
        14 days, weekly load, and recovery signals before answering — never
        before.
      </p>

      <div className="mt-5 flex-1 rounded-xl border border-white/10 bg-black/30 p-4">
        <div className="flex items-center gap-2 font-mono text-[10.5px] tracking-[0.16em] text-white/35">
          <span className="h-1.5 w-1.5 rounded-full bg-white/20" />
          ZZZ · LISTENING FOR ACTIVITIES
        </div>
        <div className="mt-4 space-y-2">
          <div className="h-2.5 w-3/4 rounded-full bg-white/[0.06]" />
          <div className="h-2.5 w-2/3 rounded-full bg-white/[0.05]" />
          <div className="h-2.5 w-1/2 rounded-full bg-white/[0.04]" />
        </div>
      </div>
    </GlassCard>
  );
}

function StatStub({ label, hint }: { label: string; hint: string }) {
  return (
    <div className="rounded-[var(--radius-card)] border border-white/[0.07] bg-gradient-to-b from-white/[0.025] to-transparent p-5">
      <p className="font-mono text-[10.5px] uppercase tracking-[0.22em] text-white/40">
        {label}
      </p>
      <p className="num mt-3 text-[36px] tracking-[-0.04em] text-white/30">
        —
      </p>
      <p className="mt-2 font-mono text-[10.5px] uppercase tracking-[0.18em] text-white/30">
        ○ {hint}
      </p>
    </div>
  );
}
