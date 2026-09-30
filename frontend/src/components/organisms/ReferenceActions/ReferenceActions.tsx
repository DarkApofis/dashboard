"use client";

import { Button } from "@/components/atoms/Button";
import { Card, CardBody, CardKicker } from "@/components/atoms/Card";
import { Spinner } from "@/components/atoms/Spinner";
import { Text } from "@/components/atoms/Text";
import { ConfirmAction } from "@/components/molecules/ConfirmAction";
import { Toast } from "@/components/molecules/Toast";
import { useAutoRefresh, useCancelReference, useSimulatePayment } from "@/features/payment-references/hooks";
import { cancellationBlockedReason, isCancellable } from "@/features/payment-references/status";
import type { PaymentReference } from "@/features/payment-references/types";

import styles from "./ReferenceActions.module.css";

export interface ReferenceActionsProps {
  reference: PaymentReference;
  /** Shows the control that posts a fake provider notification. */
  simulatorEnabled: boolean;
}

/**
 * Everything on the detail panel that can change the reference.
 *
 * Grouped into one client component because they share a concern: this is the only part
 * of the screen that is interactive. The rest of the detail is server-rendered.
 *
 * While the reference is pending it also polls, because a payment arrives through a
 * webhook rather than through anything the viewer does, so the panel would otherwise
 * sit there stale.
 */
export function ReferenceActions({ reference, simulatorEnabled }: ReferenceActionsProps) {
  const cancellable = isCancellable(reference.status);
  const blockedReason = cancellationBlockedReason(reference.status);

  useAutoRefresh(reference.status === "pending");

  const cancellation = useCancelReference(reference.id);
  const simulation = useSimulatePayment(reference.id);

  if (!cancellable) {
    return (
      <>
        <Text muted className={styles.closed}>
          {blockedReason}
        </Text>
        {simulation.result?.ok ? <Toast message={simulation.result.message ?? ""} /> : null}
      </>
    );
  }

  return (
    <div className={styles.zone}>
      {simulatorEnabled ? (
        <Card className={styles.simulator}>
          <CardKicker>Modo prueba · POST /webhooks/payments</CardKicker>
          <CardBody>
            Simula la notificación del proveedor financiero para esta referencia. Requiere que el backend corra con
            WEBHOOK_SECRET vacío.
          </CardBody>
          <Button
            variant="secondary"
            className={styles.simulatorButton}
            onClick={simulation.simulate}
            disabled={simulation.isPending}
          >
            {simulation.isPending ? <Spinner label="Enviando" /> : null}
            Simular pago recibido
          </Button>
          {simulation.result && !simulation.result.ok ? (
            <Text size="xs" role="alert" style={{ color: "var(--color-accent-700)" }}>
              {simulation.result.message}
            </Text>
          ) : null}
        </Card>
      ) : null}

      <ConfirmAction
        triggerLabel="Cancelar referencia"
        question={
          <>
            ¿Cancelar <strong>{reference.external_reference}</strong>? Esta acción no se puede deshacer.
          </>
        }
        confirmLabel="Sí, cancelar"
        onConfirm={cancellation.cancel}
        isPending={cancellation.isPending}
        error={cancellation.result && !cancellation.result.ok ? cancellation.result.message : null}
      />

      {cancellation.result?.ok ? <Toast message={cancellation.result.message ?? ""} /> : null}
      {simulation.result?.ok ? <Toast message={simulation.result.message ?? ""} /> : null}
    </div>
  );
}
