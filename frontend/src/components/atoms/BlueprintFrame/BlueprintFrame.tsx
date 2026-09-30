import type { ElementType, HTMLAttributes, ReactNode } from "react";

import { cx } from "@/lib/format";

export type Corner = "tl" | "tr" | "bl" | "br";

const ALL_CORNERS: readonly Corner[] = ["tl", "tr", "bl", "br"];

/**
 * The four "+" registration marks the design system draws outside a framed box.
 *
 * Exported on its own because the primary button wears them too, and the system is
 * explicit that a framed element must never lose them.
 */
export function BlueprintCorners({ corners = ALL_CORNERS }: { corners?: readonly Corner[] }) {
  return (
    <>
      {corners.map((corner) => (
        <i key={corner} className={`corner ${corner}`} aria-hidden="true" />
      ))}
    </>
  );
}

export interface BlueprintFrameProps extends HTMLAttributes<HTMLElement> {
  as?: "div" | "section" | "aside" | "article" | "form" | "header";
  /** Draw only some marks, for a panel flush against an edge. */
  corners?: readonly Corner[];
  children?: ReactNode;
}

/**
 * A wireframe object: square, hairline bordered, marked at the corners. Cards,
 * sections and dialogs are all built on it.
 */
export function BlueprintFrame({ as = "div", corners, className, children, ...rest }: BlueprintFrameProps) {
  const Element = as as ElementType;
  return (
    <Element className={cx("blueprint", className)} {...rest}>
      <BlueprintCorners corners={corners} />
      {children}
    </Element>
  );
}
