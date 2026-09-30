import type { ReactNode } from "react";

import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";
import { ButtonLink } from "@/components/atoms/ButtonLink";
import { Icon } from "@/components/atoms/Icon";
import { TableCell, TableHeaderCell } from "@/components/atoms/TableCell";
import { Text } from "@/components/atoms/Text";
import { StatusBadge } from "@/components/molecules/StatusBadge";
import type { PaymentReference } from "@/features/payment-references/types";
import { cx, formatCOP, formatDateTime, isExpiringSoon } from "@/lib/format";

import styles from "./ReferencesTable.module.css";

export interface ReferencesTableProps {
  references: PaymentReference[];
  /** Business id to name, so a row shows who is charging. */
  businessNames: Map<number, string>;
  /** Rendered inside the frame when there is nothing to list. */
  emptyState: ReactNode;
  /** The compact row height the design offers as an option. */
  dense?: boolean;
}

/**
 * The listing.
 *
 * A pure presentational organism: it receives rows and renders them. It does not fetch,
 * filter, sort or paginate, which is why the same table serves the unfiltered list and
 * any filtered view.
 */
export function ReferencesTable({ references, businessNames, emptyState, dense = false }: ReferencesTableProps) {
  const isEmpty = references.length === 0;

  return (
    <BlueprintFrame as="section" className={styles.frame} aria-label="Referencias de pago">
      {isEmpty ? null : (
        <div className={styles.scroll}>
          <table className="table">
            <thead>
              <tr>
                <TableHeaderCell>Referencia</TableHeaderCell>
                <TableHeaderCell>Empresa</TableHeaderCell>
                <TableHeaderCell align="right">Valor</TableHeaderCell>
                <TableHeaderCell>Estado</TableHeaderCell>
                <TableHeaderCell>Vence</TableHeaderCell>
                <TableHeaderCell>Creada</TableHeaderCell>
                <TableHeaderCell>
                  <span aria-hidden="true" />
                </TableHeaderCell>
              </tr>
            </thead>
            <tbody>
              {references.map((reference) => {
                const expiringSoon = reference.status === "pending" && isExpiringSoon(reference.expires_at);

                return (
                  <tr key={reference.id} className={styles.row}>
                    <TableCell dense={dense}>
                      <div className={styles.reference}>{reference.external_reference}</div>
                      {reference.description ? (
                        <Text as="div" muted className={styles.description}>
                          {reference.description}
                        </Text>
                      ) : null}
                      {reference.requires_review ? (
                        <span className={styles.reviewFlag}>
                          <Icon name="alert" size={12} />
                          Requiere revisión
                        </span>
                      ) : null}
                    </TableCell>
                    <TableCell dense={dense}>{businessNames.get(reference.business_id) ?? "—"}</TableCell>
                    <TableCell dense={dense} align="right" numeric>
                      {formatCOP(reference.amount)}
                    </TableCell>
                    <TableCell dense={dense}>
                      <StatusBadge status={reference.status} />
                    </TableCell>
                    <TableCell dense={dense} className={cx(!expiringSoon && "text-muted")}>
                      <Text as="span" size="sm">
                        {formatDateTime(reference.expires_at)}
                      </Text>
                    </TableCell>
                    <TableCell dense={dense} className="text-muted">
                      <Text as="span" size="sm">
                        {formatDateTime(reference.created_at)}
                      </Text>
                    </TableCell>
                    <TableCell dense={dense} align="right" nowrap>
                      <ButtonLink href={`/payment-references/${reference.id}`} variant="ghost">
                        Detalle
                        <Icon name="arrowRight" size={14} />
                      </ButtonLink>
                    </TableCell>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      {isEmpty ? emptyState : null}
    </BlueprintFrame>
  );
}
