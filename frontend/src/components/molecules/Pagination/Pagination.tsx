import { ButtonLink } from "@/components/atoms/ButtonLink";
import { Text } from "@/components/atoms/Text";

import styles from "./Pagination.module.css";

export interface PaginationProps {
  total: number;
  limit: number;
  offset: number;
  /** Builds the href for a given offset, so every page is a real link. */
  hrefForOffset: (offset: number) => string;
}

/**
 * Offset pagination.
 *
 * The reference design had none, because it was drawn with seven rows of fixed data.
 * The API caps a page at 100 rows, so the dashboard needs it; the controls are links
 * rather than buttons so a page can be bookmarked and opened in a new tab.
 */
export function Pagination({ total, limit, offset, hrefForOffset }: PaginationProps) {
  if (total === 0) {
    return null;
  }

  // An offset past the end is reachable by editing the URL. Now that the API reports
  // the real total even for an empty page, the labels have to stay honest instead of
  // rendering something like "501-3 de 3".
  const outOfRange = offset >= total;
  const first = outOfRange ? 0 : offset + 1;
  const last = outOfRange ? 0 : Math.min(offset + limit, total);
  const hasPrevious = offset > 0;
  const hasNext = !outOfRange && last < total;
  const previousOffset = Math.max(offset - limit, 0);

  return (
    <nav className={styles.wrapper} aria-label="Paginación">
      <Text size="sm" muted numeric>
        {first}–{last} de {total}
      </Text>
      <div className={styles.controls}>
        {hasPrevious ? (
          <ButtonLink href={hrefForOffset(previousOffset)} variant="secondary" prefetch={false}>
            ← Anteriores
          </ButtonLink>
        ) : (
          <span className="btn btn-secondary" aria-disabled="true">
            ← Anteriores
          </span>
        )}
        {hasNext ? (
          <ButtonLink href={hrefForOffset(offset + limit)} variant="secondary" prefetch={false}>
            Siguientes →
          </ButtonLink>
        ) : (
          <span className="btn btn-secondary" aria-disabled="true">
            Siguientes →
          </span>
        )}
      </div>
    </nav>
  );
}
