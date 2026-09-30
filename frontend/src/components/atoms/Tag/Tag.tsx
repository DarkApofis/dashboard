import type { HTMLAttributes, ReactNode } from "react";

import { cx } from "@/lib/format";

/** The tints the design system ships. The palette is mono by design. */
export type TagVariant = "accent" | "neutral" | "outline";

const VARIANT_CLASS: Record<TagVariant, string> = {
  accent: "tag-accent",
  neutral: "tag-neutral",
  outline: "tag-outline",
};

export interface TagProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: TagVariant;
  children?: ReactNode;
}

export function Tag({ variant = "neutral", className, children, ...rest }: TagProps) {
  return (
    <span className={cx("tag", VARIANT_CLASS[variant], className)} {...rest}>
      {children}
    </span>
  );
}
