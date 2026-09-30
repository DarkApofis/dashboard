import type { ButtonHTMLAttributes, ReactNode } from "react";

import { BlueprintCorners } from "@/components/atoms/BlueprintFrame";
import { cx } from "@/lib/format";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "icon" | "danger";

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary: "btn-primary",
  secondary: "btn-secondary",
  ghost: "btn-ghost",
  icon: "btn-icon btn-ghost",
  danger: "btn-danger",
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  /**
   * Draws the registration marks. The design system reserves the framed, solid
   * treatment for the one primary action on a screen.
   */
  blueprint?: boolean;
  children?: ReactNode;
}

export function Button({
  variant = "secondary",
  blueprint = false,
  type = "button",
  className,
  children,
  ...rest
}: ButtonProps) {
  return (
    <button type={type} className={cx("btn", VARIANT_CLASS[variant], blueprint && "blueprint", className)} {...rest}>
      {blueprint ? <BlueprintCorners /> : null}
      {children}
    </button>
  );
}
