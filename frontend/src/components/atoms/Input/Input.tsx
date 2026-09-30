import type { InputHTMLAttributes } from "react";

import { cx } from "@/lib/format";

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  /** Marks the control invalid for assistive technology and for the themed border. */
  invalid?: boolean;
}

export function Input({ invalid = false, className, ...rest }: InputProps) {
  return <input className={cx("input", className)} aria-invalid={invalid || undefined} {...rest} />;
}
