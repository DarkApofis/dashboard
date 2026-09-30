import { Tag, type TagVariant } from "@/components/atoms/Tag";
import { STATUS_LABELS, type PaymentReferenceStatus } from "@/features/payment-references/status";
import { cx } from "@/lib/format";

import styles from "./StatusBadge.module.css";

const VARIANT: Record<PaymentReferenceStatus, TagVariant> = {
  pending: "outline",
  paid: "accent",
  expired: "neutral",
  cancelled: "neutral",
};

const EXTRA_CLASS: Partial<Record<PaymentReferenceStatus, string>> = {
  expired: styles.expired,
  cancelled: styles.cancelled,
};

export interface StatusBadgeProps {
  status: PaymentReferenceStatus;
  /** The detail panel shows the badge a step larger. */
  large?: boolean;
  className?: string;
}

export function StatusBadge({ status, large = false, className }: StatusBadgeProps) {
  return (
    <Tag variant={VARIANT[status]} className={cx(EXTRA_CLASS[status], large && styles.large, className)}>
      {STATUS_LABELS[status]}
    </Tag>
  );
}
