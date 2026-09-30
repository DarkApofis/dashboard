import type { ElementType, HTMLAttributes, ReactNode } from "react";

import { cx } from "@/lib/format";

import styles from "./Text.module.css";

export interface TextProps extends HTMLAttributes<HTMLElement> {
  as?: "p" | "span" | "div" | "dd" | "dt";
  size?: "xs" | "sm" | "md";
  muted?: boolean;
  numeric?: boolean;
  /** The condensed, letterspaced treatment the design gives API strings. */
  technical?: boolean;
  children?: ReactNode;
}

export function Text({
  as = "p",
  size = "md",
  muted = false,
  numeric = false,
  technical = false,
  className,
  children,
  ...rest
}: TextProps) {
  const Element = as as ElementType;
  return (
    <Element
      className={cx(
        styles.reset,
        styles[size],
        muted && "text-muted",
        numeric && styles.numeric,
        technical && styles.technical,
        className,
      )}
      {...rest}
    >
      {children}
    </Element>
  );
}
