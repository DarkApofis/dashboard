import type { ReactNode } from "react";

import styles from "./VisuallyHidden.module.css";

/** Available to a screen reader, invisible on screen. */
export function VisuallyHidden({ children }: { children: ReactNode }) {
  return <span className={styles.hidden}>{children}</span>;
}
