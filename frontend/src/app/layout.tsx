import type { Metadata } from "next";
import { Barlow, Barlow_Condensed } from "next/font/google";

import { DashboardLayout } from "@/components/templates/DashboardLayout";

import "./globals.css";

/*
 * The design system loaded its fonts with an @import inside the stylesheet, which
 * blocks the first paint. next/font self-hosts them and exposes them as the two
 * variables tokens.css points at.
 */
const barlow = Barlow({
  subsets: ["latin"],
  weight: ["400", "500", "700"],
  variable: "--font-barlow",
  display: "swap",
});

const barlowCondensed = Barlow_Condensed({
  subsets: ["latin"],
  weight: ["400", "600"],
  variable: "--font-barlow-condensed",
  display: "swap",
});

export const metadata: Metadata = {
  title: "Payment Links",
  description: "Referencias de pago: creación, seguimiento y cancelación.",
};

const DATE_FORMAT = new Intl.DateTimeFormat("es-CO", {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es" className={`${barlow.variable} ${barlowCondensed.variable}`}>
      <body>
        <DashboardLayout headerMeta={`API v1 · ${DATE_FORMAT.format(new Date())}`}>{children}</DashboardLayout>
      </body>
    </html>
  );
}
