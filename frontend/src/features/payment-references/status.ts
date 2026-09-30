import type { PaymentReferenceStatus } from "./types";

// Re-exported so a component can take the status vocabulary from one module.
export type { PaymentReferenceStatus };

/** Every status, in lifecycle order. */
export const STATUSES: readonly PaymentReferenceStatus[] = ["pending", "paid", "expired", "cancelled"];

export const STATUS_LABELS: Record<PaymentReferenceStatus, string> = {
  pending: "Pendiente",
  paid: "Pagada",
  expired: "Vencida",
  cancelled: "Cancelada",
};

/**
 * The palette is deliberately mono, so the four badges are told apart by fill, border
 * and weight rather than by hue. See StatusBadge.
 */
export type StatusBadgeTone = "pending" | "paid" | "expired" | "cancelled";

/** Only a pending reference can be cancelled; the API enforces the same rule. */
export function isCancellable(status: PaymentReferenceStatus): boolean {
  return status === "pending";
}

/** Why cancelling is unavailable, for the disabled control tooltip. */
export function cancellationBlockedReason(status: PaymentReferenceStatus): string | null {
  switch (status) {
    case "pending":
      return null;
    case "paid":
      return "Una referencia pagada no puede ser cancelada.";
    case "expired":
      return "Una referencia vencida ya no puede cancelarse.";
    case "cancelled":
      return "La referencia ya está cancelada.";
  }
}

/** Validates a status coming from a URL query string. */
export function parseStatus(raw: string | undefined | null): PaymentReferenceStatus | undefined {
  if (!raw) {
    return undefined;
  }
  return STATUSES.find((status) => status === raw);
}
