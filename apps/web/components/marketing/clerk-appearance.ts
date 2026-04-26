/**
 * Clerk appearance preset that themes the SignIn / SignUp / UserProfile
 * components into our aurora aesthetic. Variables drive Clerk's defaults,
 * `elements` overrides specific parts (card, inputs, social buttons) so they
 * feel native inside our glass card.
 *
 * Tailwind utility classes are passed verbatim — Clerk merges them into its
 * own classNames using Tailwind's class string concat, which works fine in
 * a v4 setup as long as Clerk's selectors aren't more specific. Type is
 * inferred at the call site (<SignIn appearance={...}>), which gives us full
 * compile-time validation without depending on @clerk/types directly.
 */
export const clerkAppearance = {
  variables: {
    colorBackground: 'transparent',
    colorPrimary: 'oklch(0.96 0 0)',
    colorText: 'oklch(0.97 0.01 180)',
    colorTextSecondary: 'oklch(0.78 0.01 200)',
    colorInputBackground: 'oklch(1 0 0 / 0.04)',
    colorInputText: 'oklch(0.97 0.01 180)',
    colorDanger: 'oklch(0.66 0.22 25)',
    colorSuccess: 'oklch(0.78 0.20 150)',
    colorNeutral: 'oklch(1 0 0)',
    fontFamily: 'var(--font-sans)',
    fontFamilyButtons: 'var(--font-sans)',
    fontSize: '14.5px',
    borderRadius: '12px',
    spacingUnit: '1rem',
  },
  elements: {
    rootBox: 'w-full',
    cardBox: 'bg-transparent shadow-none border-0 w-full',
    card: 'bg-transparent shadow-none border-0 px-0 py-0',
    header: 'text-center mb-2',
    headerTitle:
      'text-white text-[26px] font-medium tracking-[-0.02em] leading-[1.1]',
    headerSubtitle: 'text-white/55 text-[14.5px] mt-2',
    socialButtonsBlockButton:
      'bg-white/[0.04] border border-white/10 text-white hover:bg-white/[0.08] hover:border-white/20 rounded-xl h-11 transition-colors',
    socialButtonsBlockButtonText: 'text-white/90 font-medium',
    dividerLine: 'bg-white/10',
    dividerText:
      'text-white/40 font-mono text-[10.5px] tracking-[0.18em] uppercase',
    formFieldLabel:
      'text-white/65 text-[12px] font-mono tracking-[0.16em] uppercase mb-1.5',
    formFieldInput:
      'bg-white/[0.04] border border-white/10 text-white rounded-xl h-11 px-4 placeholder:text-white/30 focus:border-white/30 focus:bg-white/[0.06] transition-colors',
    formButtonPrimary:
      'bg-white text-black hover:bg-white/90 rounded-full h-11 font-medium tracking-tight transition-colors shadow-[0_8px_24px_-8px_rgb(255_255_255_/_0.35)]',
    formFieldAction: 'text-white/70 hover:text-white text-[12.5px]',
    footer: 'mt-6 [&_.cl-internal-1c1pp9p]:bg-transparent',
    footerAction: 'text-[13px]',
    footerActionText: 'text-white/55',
    footerActionLink: 'text-white hover:text-white/80 underline-offset-4',
    identityPreviewEditButton: 'text-white/70 hover:text-white',
    formResendCodeLink: 'text-white/70 hover:text-white',
    otpCodeFieldInput:
      'bg-white/[0.04] border border-white/10 text-white rounded-lg',
    alert:
      'bg-[oklch(0.66_0.22_25_/_0.1)] border border-[oklch(0.66_0.22_25_/_0.3)] text-[oklch(0.85_0.10_25)] rounded-xl',
    badge:
      'bg-white/[0.06] border border-white/10 text-white/80 rounded-full font-mono text-[10.5px] tracking-[0.16em]',
  },
  layout: {
    socialButtonsPlacement: 'top' as const,
    socialButtonsVariant: 'blockButton' as const,
    showOptionalFields: true,
  },
};
