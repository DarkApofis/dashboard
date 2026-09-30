import type { HTMLAttributes, ReactNode } from "react";

import { cx } from "@/lib/format";

import styles from "./Heading.module.css";

export type HeadingLevel = 1 | 2 | 3 | 4 | 5 | 6;

/**
 * Visual size, chosen apart from the heading level so the document outline stays
 * correct while the type scale follows the design.
 */
export type HeadingSize = "inherit" | "display" | "kicker";

export interface HeadingProps extends HTMLAttributes<HTMLHeadingElement> {
  level: HeadingLevel;
  size?: HeadingSize;
  children?: ReactNode;
}

export function Heading({ level, size = "inherit", className, children, ...rest }: HeadingProps) {
  const Element = `h${level}` as "h1" | "h2" | "h3" | "h4" | "h5" | "h6";
  const sizeClass = size === "display" ? styles.display : size === "kicker" ? styles.kicker : undefined;

  return (
    <Element className={cx(styles.reset, sizeClass, className)} {...rest}>
      {children}
    </Element>
  );
}

/** A large number presented as a figure rather than as a heading. */
export function Figure({
  large = false,
  className,
  children,
  ...rest
}: HTMLAttributes<HTMLDivElement> & { large?: boolean }) {
  return (
    <div className={cx(large ? styles.figureLarge : styles.figure, className)} {...rest}>
      {children}
    </div>
  );
}
