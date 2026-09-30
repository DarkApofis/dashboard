import type { ReactNode } from "react";

export interface ListTemplateProps {
  header: ReactNode;
  stats?: ReactNode;
  filters?: ReactNode;
  table: ReactNode;
  pagination?: ReactNode;
  footer?: ReactNode;
}

/**
 * The listing screen, as slots.
 *
 * It owns the order and the rhythm of the page and nothing else: it receives rendered
 * regions, so the same template could list something other than payment references.
 */
export function ListTemplate({ header, stats, filters, table, pagination, footer }: ListTemplateProps) {
  return (
    <>
      {header}
      {stats}
      {filters}
      {table}
      {pagination}
      {footer}
    </>
  );
}
