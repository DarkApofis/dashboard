import type { ReactNode } from "react";

import { AppHeader } from "@/components/organisms/AppHeader";

import styles from "./DashboardLayout.module.css";

export interface DashboardLayoutProps {
  children: ReactNode;
  headerMeta?: string;
}

/** The shell every screen sits in: header bar plus a centred column. */
export function DashboardLayout({ children, headerMeta }: DashboardLayoutProps) {
  return (
    <div className="app-shell">
      <AppHeader meta={headerMeta} />
      <main className={styles.main}>{children}</main>
    </div>
  );
}
