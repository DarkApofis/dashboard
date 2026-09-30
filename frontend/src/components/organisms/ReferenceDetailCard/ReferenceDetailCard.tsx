import { Figure } from "@/components/atoms/Heading";
import { Heading } from "@/components/atoms/Heading";
import { Icon } from "@/components/atoms/Icon";
import { Text } from "@/components/atoms/Text";
import { CardKicker } from "@/components/atoms/Card";
import { CopyableField } from "@/components/molecules/CopyableField";
import { DefinitionList, type DefinitionItem } from "@/components/molecules/DefinitionList";
import { StatusBadge } from "@/components/molecules/StatusBadge";
import { ReferenceActions } from "@/components/organisms/ReferenceActions";
import type { PaymentReference } from "@/features/payment-references/types";
import { cx, formatCOP, formatDateTime, isExpiringSoon } from "@/lib/format";

import styles from "./ReferenceDetailCard.module.css";

export interface ReferenceDetailCardProps {
  reference: PaymentReference;
  businessName: string;
  /** The shareable link, built by the caller from the configured base. */
  paymentLink: string;
  simulatorEnabled: boolean;
  /** The close control, a link back to the listing. */
  closeSlot: React.ReactNode;
}

/**
 * The detail panel.
 *
 * A Server Component: everything here is read-only, so it renders on the server and
 * only the action zone ships JavaScript.
 */
export function ReferenceDetailCard({
  reference,
  businessName,
  paymentLink,
  simulatorEnabled,
  closeSlot,
}: ReferenceDetailCardProps) {
  const isPaid = reference.status === "paid";
  const expiringSoon = reference.status === "pending" && isExpiringSoon(reference.expires_at);

  const items: DefinitionItem[] = [
    { term: "id", value: reference.id },
    {
      term: "business",
      value: (
        <>
          {businessName} <span className="text-muted">· #{reference.business_id}</span>
        </>
      ),
    },
    {
      term: "expires_at",
      value: formatDateTime(reference.expires_at, { withYear: true }),
      valueClassName: expiringSoon ? undefined : "text-muted",
    },
    { term: "created_at", value: formatDateTime(reference.created_at, { withYear: true }) },
    { term: "updated_at", value: formatDateTime(reference.updated_at, { withYear: true }) },
  ];

  if (reference.cancelled_at) {
    items.push({ term: "cancelled_at", value: formatDateTime(reference.cancelled_at, { withYear: true }) });
  }

  return (
    <>
      <div className={styles.header}>
        <div>
          <Heading level={6} size="kicker">
            GET /payment-references/{reference.id}
          </Heading>
          <div className={styles.reference}>{reference.external_reference}</div>
          {reference.description ? (
            <Text muted className={styles.description}>
              {reference.description}
            </Text>
          ) : null}
        </div>
        {closeSlot}
      </div>

      <div className={styles.body}>
        {isPaid ? (
          <div className={cx("blueprint", styles.paidBanner)}>
            <i className="corner tl" aria-hidden="true" />
            <i className="corner tr" aria-hidden="true" />
            <i className="corner bl" aria-hidden="true" />
            <i className="corner br" aria-hidden="true" />
            <div className={styles.paidTitle}>
              <Icon name="check" size={20} />
              Pago confirmado
            </div>
            <div className={styles.paidMeta}>
              Recibido vía webhook · {formatDateTime(reference.paid_at, { withYear: true })}
            </div>
            {reference.transaction_id ? (
              <div className={styles.paidTransaction}>transaction_id {reference.transaction_id}</div>
            ) : null}
          </div>
        ) : null}

        {reference.requires_review ? (
          <div className={styles.reviewNotice} role="note">
            <Icon name="alert" size={16} />
            <span>
              Requiere revisión manual
              {reference.review_reason ? <> · {reference.review_reason}</> : null}
            </span>
          </div>
        ) : null}

        <div className={styles.amountRow}>
          <div>
            <CardKicker>Valor</CardKicker>
            <Figure large>{formatCOP(reference.amount)}</Figure>
          </div>
          <StatusBadge status={reference.status} large />
        </div>

        <DefinitionList items={items} />

        <CopyableField id="payment-link" label="Enlace de pago para compartir" value={paymentLink} />

        <ReferenceActions reference={reference} simulatorEnabled={simulatorEnabled} />
      </div>
    </>
  );
}
