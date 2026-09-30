import { cx } from "@/lib/format";

import styles from "./Spinner.module.css";

export interface SpinnerProps {
  large?: boolean;
  className?: string;
  /** Announced to assistive technology; pass null for a purely decorative spinner. */
  label?: string | null;
}

export function Spinner({ large = false, className, label = "Cargando" }: SpinnerProps) {
  return (
    <span
      className={cx(styles.spinner, large && styles.large, className)}
      role={label ? "status" : undefined}
      aria-label={label ?? undefined}
      aria-hidden={label ? undefined : true}
    />
  );
}
