import type { ReactNode, TdHTMLAttributes, ThHTMLAttributes } from "react";

import { cx } from "@/lib/format";

import styles from "./TableCell.module.css";

export interface TableCellProps extends TdHTMLAttributes<HTMLTableCellElement> {
  align?: "left" | "right";
  /** Money and ids: tabular digits so columns line up. */
  numeric?: boolean;
  /** The compact row height the design offers as an option. */
  dense?: boolean;
  nowrap?: boolean;
  children?: ReactNode;
}

export function TableCell({
  align = "left",
  numeric = false,
  dense = false,
  nowrap = false,
  className,
  children,
  ...rest
}: TableCellProps) {
  return (
    <td
      className={cx(
        styles.cell,
        dense && styles.dense,
        align === "right" && styles.right,
        numeric && styles.numeric,
        nowrap && styles.nowrap,
        className,
      )}
      {...rest}
    >
      {children}
    </td>
  );
}

export interface TableHeaderCellProps extends ThHTMLAttributes<HTMLTableCellElement> {
  align?: "left" | "right";
  children?: ReactNode;
}

export function TableHeaderCell({ align = "left", className, children, ...rest }: TableHeaderCellProps) {
  return (
    <th scope="col" className={cx(styles.header, align === "right" && styles.right, className)} {...rest}>
      {children}
    </th>
  );
}
