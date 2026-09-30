import Link from "next/link";
import type { ComponentProps, ReactNode } from "react";

import { BlueprintCorners } from "@/components/atoms/BlueprintFrame";
import { cx } from "@/lib/format";
import type { ButtonVariant } from "@/components/atoms/Button";

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: "btn-primary",
  secondary: "btn-secondary",
  ghost: "btn-ghost",
  icon: "btn-icon btn-ghost",
  danger: "btn-danger",
};

export interface ButtonLinkProps extends Omit<ComponentProps<typeof Link>, "className"> {
  variant?: ButtonVariant;
  blueprint?: boolean;
  className?: string;
  children?: ReactNode;
}

/**
 * A navigation that looks like a button.
 *
 * Kept apart from Button on purpose: anything that changes the URL has to be a real
 * link, so it can be opened in a new tab, copied, or crawled. Only actions that mutate
 * are buttons.
 */
export function ButtonLink({ variant = "secondary", blueprint = false, className, children, ...rest }: ButtonLinkProps) {
  return (
    <Link className={cx("btn", VARIANT_CLASS[variant], blueprint && "blueprint", className)} {...rest}>
      {blueprint ? <BlueprintCorners /> : null}
      {children}
    </Link>
  );
}
