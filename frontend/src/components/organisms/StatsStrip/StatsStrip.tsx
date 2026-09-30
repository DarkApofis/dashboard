import { StatCard } from "@/components/molecules/StatCard";
import type { StatusCounts } from "@/features/payment-references/types";

import styles from "./StatsStrip.module.css";

export interface StatsStripProps {
  counts: StatusCounts;
  /** True when a filter is narrowing the counts, so the caption can say so. */
  filtered: boolean;
}

/**
 * The four counters above the table.
 *
 * The design showed two of these as sums of money. The API exposes no aggregates, and
 * a sum computed from the current page would be wrong, so these are counts: numbers
 * that are actually true. Adding GET /payment-references/stats to the backend is what
 * it would take to show the amounts.
 */
export function StatsStrip({ counts, filtered }: StatsStripProps) {
  const scope = filtered ? "con los filtros actuales" : "en total";

  return (
    <section className={styles.strip} aria-label="Resumen">
      <StatCard label="Cobradas" value={counts.paid} meta={`pagadas ${scope}`} />
      <StatCard label="Por cobrar" value={counts.pending} meta={`pendientes ${scope}`} />
      <StatCard label="Vencidas" value={counts.expired} meta="sin pago antes de expires_at" />
      <StatCard label="Canceladas" value={counts.cancelled} meta="canceladas por la empresa" />
    </section>
  );
}
