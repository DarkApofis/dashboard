import type { SelectHTMLAttributes } from "react";

import { cx } from "@/lib/format";

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean;
}

/** A native select wearing the design system input treatment. */
export function Select({ invalid = false, className, children, ...rest }: SelectProps) {
  return (
    <select className={cx("input", className)} aria-invalid={invalid || undefined} {...rest}>
      {children}
    </select>
  );
}
