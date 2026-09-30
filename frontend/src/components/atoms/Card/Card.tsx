import type { HTMLAttributes, ReactNode } from "react";

import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";
import { cx } from "@/lib/format";

export interface CardProps extends HTMLAttributes<HTMLElement> {
  children?: ReactNode;
}

/** A transparent, hairline-bordered card with its registration marks. */
export function Card({ className, children, ...rest }: CardProps) {
  return (
    <BlueprintFrame className={cx("card", className)} {...rest}>
      {children}
    </BlueprintFrame>
  );
}

export function CardKicker({ className, children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cx("card-kicker", className)} {...rest}>
      {children}
    </div>
  );
}

export function CardTitle({ className, children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cx("card-title", className)} {...rest}>
      {children}
    </div>
  );
}

export function CardBody({ className, children, ...rest }: HTMLAttributes<HTMLParagraphElement>) {
  return (
    <p className={cx("card-body", className)} {...rest}>
      {children}
    </p>
  );
}

export function CardMeta({ className, children, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cx("card-meta", className)} {...rest}>
      {children}
    </div>
  );
}
