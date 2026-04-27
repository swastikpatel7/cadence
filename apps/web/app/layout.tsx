import { ClerkProvider } from '@clerk/nextjs';
import { GeistMono } from 'geist/font/mono';
import { GeistPixelSquare } from 'geist/font/pixel';
import { GeistSans } from 'geist/font/sans';
import type { Metadata } from 'next';
import { Instrument_Serif } from 'next/font/google';
import './globals.css';

const instrumentSerif = Instrument_Serif({
  subsets: ['latin'],
  weight: '400',
  style: 'italic',
  variable: '--font-instrument-serif',
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'Cadence — a coach that trains with you.',
  description:
    'Cadence reads your runs, rides, swims, and lifts and tells you, in your voice, how to push, when to pull back, and what your data is really saying.',
  metadataBase: new URL('https://cadence.local'),
  openGraph: {
    title: 'Cadence',
    description: 'A coach that trains with you.',
    type: 'website',
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <ClerkProvider>
      <html
        lang="en"
        className={`${GeistSans.variable} ${GeistMono.variable} ${GeistPixelSquare.variable} ${instrumentSerif.variable}`}
      >
        <body className="antialiased">{children}</body>
      </html>
    </ClerkProvider>
  );
}
