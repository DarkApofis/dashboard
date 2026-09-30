import Link from "next/link";

import { cx } from "@/lib/format";

import styles from "./AppHeader.module.css";

export interface AppHeaderProps {
  /** Rendered on the right, as the design does with the API version and the date. */
  meta?: string;
}

/**
 * The header bar.
 *
 * The design also sketched "Empresas" and "Webhooks" links. They are left out rather
 * than rendered dead: a link that goes nowhere is worse than one that is not there.
 */
export function AppHeader({ meta }: AppHeaderProps) {
  return (
    <header className={cx("nav", styles.header)}>
      <div className={cx("nav-brand", styles.brand)}>
        <span className={styles.mark} aria-hidden="true" />
        Payment Links
      </div>
      <Link href="/payment-references" aria-current="page">
        Referencias
      </Link>
      {meta ? <span className={cx("text-muted", styles.meta)}>{meta}</span> : null}
    </header>
  );
}
