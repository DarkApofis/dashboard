"use client";

import { Select } from "@/components/atoms/Select";
import { Spinner } from "@/components/atoms/Spinner";
import { Text } from "@/components/atoms/Text";
import { VisuallyHidden } from "@/components/atoms/VisuallyHidden";
import { SearchInput } from "@/components/molecules/SearchInput";
import { SegmentedFilter, type SegmentedFilterOption } from "@/components/molecules/SegmentedFilter";
import type { Business } from "@/features/businesses/types";
import { useReferenceFilters } from "@/features/payment-references/hooks";
import { STATUSES, STATUS_LABELS } from "@/features/payment-references/status";
import type { PaymentReferenceStatus, StatusCounts } from "@/features/payment-references/types";

import styles from "./ReferencesFilters.module.css";

type StatusFilterValue = PaymentReferenceStatus | "all";

export interface ReferencesFiltersProps {
  businesses: Business[];
  counts: StatusCounts;
  /** How many rows the current page shows, out of how many match. */
  shown: number;
  total: number;
}

/**
 * The filter row.
 *
 * Every filter is written to the URL rather than to component state, so a filtered
 * view is a link someone can share, bookmark or reload. This is the only organism that
 * reaches for a hook, and it reaches for the one that owns the URL.
 */
export function ReferencesFilters({ businesses, counts, shown, total }: ReferencesFiltersProps) {
  const { values, isPending, setStatus, setBusiness, setExternalReference } = useReferenceFilters();

  const statusOptions: Array<SegmentedFilterOption<StatusFilterValue>> = [
    { value: "all", label: "Todas", count: counts.all },
    ...STATUSES.map((status) => ({
      value: status as StatusFilterValue,
      label: STATUS_LABELS[status],
      count: counts[status],
    })),
  ];

  return (
    <section className={styles.wrapper} aria-label="Filtros">
      <SegmentedFilter
        label="Filtrar por estado"
        options={statusOptions}
        value={values.status}
        onChange={setStatus}
        disabled={isPending}
      />

      <label htmlFor="business-filter">
        <VisuallyHidden>Filtrar por empresa</VisuallyHidden>
      </label>
      <Select
        id="business-filter"
        className={styles.businessSelect}
        value={values.businessId}
        disabled={isPending}
        onChange={(event) => setBusiness(event.target.value)}
      >
        <option value="all">Todas las empresas</option>
        {businesses.map((business) => (
          <option key={business.id} value={business.id}>
            {business.name}
          </option>
        ))}
      </Select>

      <SearchInput
        id="external_reference"
        label="Buscar por referencia externa"
        // The API matches this exactly, so the placeholder says so rather than
        // promising a substring search the backend does not do.
        placeholder="external_reference exacta (INV-10023)"
        value={values.externalReference}
        onSubmit={setExternalReference}
        disabled={isPending}
      />

      <Text as="span" muted className={styles.result}>
        {isPending ? <Spinner label="Aplicando filtros" /> : null}
        {shown} de {total}
      </Text>
    </section>
  );
}
