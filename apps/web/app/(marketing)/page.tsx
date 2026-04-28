import { auth } from '@clerk/nextjs/server';
import { ArrowRight, Button } from '@/components/ui/button';
import { Aurora } from '@/components/ui/aurora';
import { GlassCard } from '@/components/ui/glass-card';
import { KeyHint } from '@/components/ui/key-hint';
import { Pill } from '@/components/ui/pill';

export default async function LandingPage() {
  // Signed-in visitors can still browse the landing page; the CTAs swap to
  // dashboard links so "Start training" doesn't dead-end through /sign-up.
  const { userId } = await auth();
  const signedIn = userId !== null && userId !== undefined;

  return (
    <>
      <Hero signedIn={signedIn} />
      <HowItWorks />
      <PrivacyStrip />
      <BottomCTA signedIn={signedIn} />
    </>
  );
}

/* ─────────────────────────────────────────────────────────────────
   Hero — violet aurora, status pill, editorial italic headline.
   ───────────────────────────────────────────────────────────────── */
function Hero({ signedIn }: { signedIn: boolean }) {
  return (
    <section className="relative min-h-[100svh] overflow-hidden">
      <Aurora
        variant="violet"
        intensity="normal"
        focus={[0.78, 0.45]}
        wind={[-0.012, 0.004]}
        scale={0.85}
      />
      {/* Left-side darkening so the headline reads against the aurora —
          stronger here because we want the wisps to live in the right
          half of the canvas, like the Lumen hero. */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 z-[1]"
        style={{
          background:
            'linear-gradient(90deg, oklch(0.05 0.02 270 / 0.85) 0%, oklch(0.05 0.02 270 / 0.45) 35%, oklch(0.05 0.02 270 / 0.0) 65%)',
        }}
      />

      {/* Faint horizon line at the bottom of the hero — anchors the eye. */}
      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-10 h-px bg-gradient-to-r from-transparent via-white/15 to-transparent" />

      <div className="relative z-20 mx-auto flex min-h-[100svh] max-w-[1180px] flex-col justify-between px-6 pb-12 pt-32 md:pt-36">
        <div>
          <div
            className="mb-7 animate-[rise-in_900ms_var(--ease-out-expo)_both]"
            style={{ animationDelay: '50ms' }}
          >
            <Pill className="text-white/75">
              <span
                className="relative inline-flex h-1.5 w-1.5 rounded-full bg-[var(--color-success)]"
                style={{ animation: 'pulse-dot 2.4s ease-out infinite' }}
                aria-hidden
              />
              <span className="font-mono text-[12px] tracking-[0.14em]">
                CADENCE 0.1 — TRAINING JOURNAL FOR ENDURANCE ATHLETES
              </span>
            </Pill>
          </div>

          <h1
            className="display max-w-[14ch] animate-[rise-in_900ms_var(--ease-out-expo)_both] text-[14vw] leading-[0.92] text-white sm:text-[88px] md:text-[112px] lg:text-[128px]"
            style={{ animationDelay: '180ms' }}
          >
            <span className="not-italic font-sans tracking-[-0.04em] font-medium">
              A coach
            </span>
            <br />
            <span className="not-italic font-sans tracking-[-0.04em] font-medium">
              that
            </span>{' '}
            <span
              className="bg-clip-text text-transparent"
              style={{
                backgroundImage:
                  'linear-gradient(120deg, oklch(0.92 0.04 200) 0%, oklch(0.78 0.16 220) 60%, oklch(0.70 0.20 295) 100%)',
              }}
            >
              trains
            </span>
            <span className="not-italic font-sans tracking-[-0.04em] font-medium">
              {' '}
              with you.
            </span>
          </h1>

          <p
            className="mt-7 max-w-[44ch] animate-[rise-in_900ms_var(--ease-out-expo)_both] text-[17px] leading-[1.55] text-white/65 md:text-[18px]"
            style={{ animationDelay: '320ms' }}
          >
            Cadence reads your runs, rides, swims, and lifts — then tells you,
            in your voice, how to push, when to pull back, and what your data is
            actually saying. Strava-native. Privacy-first. Built for the long
            run.
          </p>

          <div
            className="mt-10 flex flex-wrap items-center gap-3 animate-[rise-in_900ms_var(--ease-out-expo)_both]"
            style={{ animationDelay: '460ms' }}
          >
            <Button href={signedIn ? '/home' : '/sign-up'} size="lg">
              {signedIn ? 'Open dashboard' : 'Start training'}
              <ArrowRight />
            </Button>
            <Button href="#how" variant="ghost" size="lg">
              <span aria-hidden className="text-white/45">▸</span>
              See how it works
            </Button>
          </div>
        </div>

        <div
          className="mt-16 flex items-end justify-between gap-6 animate-[fade-in_1.2s_ease-out_both]"
          style={{ animationDelay: '900ms' }}
        >
          <KeyHint
            keys={['⌘', 'K']}
            caption="Move your mouse to interact · Press to ask the coach"
          />
          <ScenePicker />
        </div>
      </div>
    </section>
  );
}

/* The lower-right scene chip stack — echoes Lumen's preset picker, but for
   the data surfaces Cadence ships with. */
function ScenePicker() {
  const scenes = [
    'Daily readout',
    'Weekly load',
    'Coach',
    'Strava',
    'Privacy',
  ];
  return (
    <div className="hidden flex-wrap items-center gap-1.5 rounded-full border border-white/10 bg-black/30 px-2 py-2 backdrop-blur-md md:flex">
      {scenes.map((s, i) => (
        <span
          key={s}
          className={
            i === 0
              ? 'rounded-full border border-white/30 bg-white/[0.06] px-3 py-1 text-[12px] text-white/95'
              : 'rounded-full px-3 py-1 text-[12px] text-white/55 transition-colors hover:bg-white/[0.04] hover:text-white/85'
          }
        >
          {s}
        </span>
      ))}
    </div>
  );
}

/* ─────────────────────────────────────────────────────────────────
   How it works — three numbered cards on the deep navy. Mono labels,
   glass surfaces, ASCII diagrams.
   ───────────────────────────────────────────────────────────────── */
function HowItWorks() {
  return (
    <section
      id="how"
      className="relative bg-[var(--color-bg-deep)] py-28 md:py-36"
    >
      <div className="mx-auto max-w-[1180px] px-6">
        <div className="mb-16 max-w-3xl">
          <p className="font-mono text-[12px] uppercase tracking-[0.22em] text-white/40">
            ⏱ THE LOOP
          </p>
          <h2 className="mt-4 text-[44px] font-medium leading-[1.02] tracking-[-0.03em] text-white md:text-[56px]">
            Three steps,{' '}
            <span className="display text-white/95">forever after.</span>
          </h2>
          <p className="mt-5 max-w-[55ch] text-[17px] leading-[1.55] text-white/55">
            Cadence is small on purpose. You connect Strava once, your activities
            stream in within sixty seconds, and your coach has the context to
            actually be useful — not just a chatbot with vibes.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
          <StepCard
            id="strava"
            number="01"
            label="CONNECT"
            title="Strava in one click."
            body="Read-only OAuth. Cadence pulls activities, splits, GPS streams, heart rate, and power. We never post on your behalf and never write to your account."
            ascii={`┌──────────────┐
│  STRAVA      │
│  ↓ webhook   │
│  cadence.    │
└──────────────┘`}
          />
          <StepCard
            id="coach"
            number="02"
            label="UNDERSTAND"
            title="A coach with your data."
            body="The coach reads your last 14 days of training, weekly volume, and recovery signals before answering. Citations link back to specific activities — no fabrications."
            ascii={`load.weekly = 38mi
hr.zone3      = 18%
↳ "ease tomorrow"`}
          />
          <StepCard
            id="privacy"
            number="03"
            label="OWN"
            title="Yours, encrypted, deletable."
            body="Tokens AES-256-GCM at rest. Your activities live in your Postgres row. Press one button to export everything. Press another to erase it within 24 hours."
            ascii={`AES-256-GCM
key.rotated  = 30d
delete       = 24h`}
          />
        </div>
      </div>
    </section>
  );
}

function StepCard({
  id,
  number,
  label,
  title,
  body,
  ascii,
}: {
  id: string;
  number: string;
  label: string;
  title: string;
  body: string;
  ascii: string;
}) {
  return (
    <div
      id={id}
      className="group relative scroll-mt-28 overflow-hidden rounded-[var(--radius-card)] border border-white/[0.07] bg-gradient-to-b from-white/[0.035] to-white/[0.01] p-6 transition-colors duration-[var(--duration-base)] hover:border-white/15"
    >
      <div className="flex items-center justify-between">
        <span className="font-mono text-[11px] tracking-[0.2em] text-white/40">
          {label}
        </span>
        <span className="font-mono text-[28px] tracking-[-0.04em] text-white/15">
          {number}
        </span>
      </div>
      <h3 className="mt-7 text-[22px] font-medium leading-[1.12] tracking-[-0.02em] text-white">
        {title}
      </h3>
      <p className="mt-3 text-[14.5px] leading-[1.55] text-white/55">{body}</p>
      <pre
        className="mt-7 overflow-hidden rounded-md border border-white/[0.06] bg-black/30 p-3 font-mono text-[11.5px] leading-[1.55] text-white/45"
        style={{ whiteSpace: 'pre' }}
      >
        {ascii}
      </pre>
      {/* Bottom-edge highlight on hover — subtle "lit" feel. */}
      <div
        className="pointer-events-none absolute inset-x-0 bottom-0 h-px bg-gradient-to-r from-transparent via-white/30 to-transparent opacity-0 transition-opacity duration-[var(--duration-slow)] group-hover:opacity-100"
        aria-hidden
      />
    </div>
  );
}

/* ─────────────────────────────────────────────────────────────────
   Privacy strip — compact, mono-flavored manifesto. Reuses the brand
   trust we already establish on /connect/strava.
   ───────────────────────────────────────────────────────────────── */
function PrivacyStrip() {
  const lines = [
    ['ENCRYPTED', 'AES-256-GCM at rest, TLS 1.3 in flight.'],
    ['NEVER SOLD', 'No ads. No analytics broker. No model training on you.'],
    ['EXPORTABLE', 'JSON dump in one click. Includes everything we know.'],
    ['DELETABLE', 'Erase your account; data is gone within 24 hours.'],
  ] as const;
  return (
    <section
      id="privacy"
      className="relative scroll-mt-28 border-y border-white/[0.06] bg-[var(--color-bg-base)]"
    >
      <div className="mx-auto grid max-w-[1180px] grid-cols-2 px-6 md:grid-cols-4">
        {lines.map(([tag, body], i) => (
          <div
            key={tag}
            className={
              'border-white/[0.06] py-7 ' +
              (i % 4 !== 3 ? 'md:border-r ' : '') +
              (i < 2 ? 'border-b md:border-b-0 ' : '') +
              (i % 2 === 0 ? 'pr-6 ' : 'pl-6 md:pl-7')
            }
          >
            <span className="font-mono text-[11px] tracking-[0.22em] text-[var(--color-success)]">
              {tag}
            </span>
            <p className="mt-3 text-[14px] leading-[1.5] text-white/65">{body}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ─────────────────────────────────────────────────────────────────
   Bottom CTA — marble aurora. Different palette, same component.
   ───────────────────────────────────────────────────────────────── */
function BottomCTA({ signedIn }: { signedIn: boolean }) {
  return (
    <section className="relative overflow-hidden">
      <Aurora
        variant="marble"
        intensity="normal"
        focus={[0.5, 0.55]}
        wind={[0.012, -0.004]}
        scale={1.40}
      />
      <div className="relative z-10 mx-auto flex max-w-[1180px] flex-col items-center px-6 py-32 text-center md:py-40">
        <p className="font-mono text-[11px] uppercase tracking-[0.22em] text-white/55">
          AVAILABLE NOW · INVITE-ONLY BETA
        </p>
        <h2 className="mt-4 text-[48px] font-medium leading-[1.02] tracking-[-0.03em] text-white md:text-[80px]">
          Built for the
          <br />
          <span className="display text-white/95"> long run.</span>
        </h2>
        <p className="mt-6 max-w-[48ch] text-[17px] leading-[1.55] text-white/65">
          Pair it with your Strava. Keep training. Cadence quietly learns your
          rhythm and earns its place on your home screen.
        </p>

        <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
          <Button href={signedIn ? '/home' : '/sign-up'} size="lg">
            {signedIn ? 'Open dashboard' : 'Get early access'}
            <ArrowRight />
          </Button>
          {signedIn ? null : (
            <Button href="/sign-in" variant="ghost" size="lg">
              I already have an account
            </Button>
          )}
        </div>

        <GlassCard className="mt-14 grid w-full max-w-3xl grid-cols-3 gap-px overflow-hidden bg-white/[0.04] p-0">
          {(
            [
              ['STRAVA', 'Strava', 'live'],
              ['WHOOP', 'Whoop', 'soon'],
              ['GARMIN', 'Garmin', 'soon'],
            ] as const
          ).map(([tag, sub, status]) => (
            <div
              key={tag}
              className="flex flex-col items-start gap-1 bg-black/30 px-6 py-5 text-left"
            >
              <span className="font-mono text-[10.5px] tracking-[0.22em] text-white/45">
                {tag}
              </span>
              <span className="text-[15px] text-white/95">{sub}</span>
              <span
                className={
                  'mt-2 font-mono text-[10.5px] tracking-[0.18em] ' +
                  (status === 'live'
                    ? 'text-[var(--color-success)]'
                    : 'text-white/35')
                }
              >
                {status === 'live' ? '● LIVE' : '○ ' + status.toUpperCase()}
              </span>
            </div>
          ))}
        </GlassCard>

        <p className="mt-10 font-mono text-[11px] tracking-[0.16em] text-white/35">
          v0.1 · RELEASED THIS WEEK · <a href="/#privacy" className="hover:text-white/65 underline underline-offset-2">CHANGELOG</a>
        </p>
      </div>
    </section>
  );
}
