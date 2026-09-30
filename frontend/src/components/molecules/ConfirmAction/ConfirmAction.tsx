"use client";

import { useState, type ReactNode } from "react";

import { Button } from "@/components/atoms/Button";
import { Spinner } from "@/components/atoms/Spinner";

import styles from "./ConfirmAction.module.css";

export interface ConfirmActionProps {
  /** The label of the button that opens the confirmation. */
  triggerLabel: string;
  /** What the user is being asked, rendered inline like in the design. */
  question: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  onConfirm: () => void;
  isPending?: boolean;
  error?: string | null;
  /** When set the trigger is disabled and this explains why. */
  disabledReason?: string | null;
}

/**
 * A two-step destructive action: the trigger swaps itself for an inline confirmation,
 * the way the reference design does it, rather than opening a modal over a panel that
 * is already a modal.
 *
 * Presentational: it owns only whether the confirmation is showing. What confirming
 * does, whether it is in flight and what failed all arrive as props.
 */
export function ConfirmAction({
  triggerLabel,
  question,
  confirmLabel,
  cancelLabel = "Volver",
  onConfirm,
  isPending = false,
  error = null,
  disabledReason = null,
}: ConfirmActionProps) {
  const [asking, setAsking] = useState(false);

  if (disabledReason) {
    return (
      <div className={styles.wrapper}>
        <Button variant="secondary" className={styles.trigger} disabled title={disabledReason}>
          {triggerLabel}
        </Button>
        <span className="text-muted" style={{ fontSize: 12 }}>
          {disabledReason}
        </span>
      </div>
    );
  }

  return (
    <div className={styles.wrapper}>
      {asking ? (
        <div className={styles.row}>
          <span className={styles.question}>{question}</span>
          <div className={styles.actions}>
            <Button variant="secondary" onClick={() => setAsking(false)} disabled={isPending}>
              {cancelLabel}
            </Button>
            <Button variant="danger" onClick={onConfirm} disabled={isPending}>
              {isPending ? <Spinner label="Procesando" /> : null}
              {confirmLabel}
            </Button>
          </div>
        </div>
      ) : (
        <Button variant="secondary" className={styles.trigger} onClick={() => setAsking(true)}>
          {triggerLabel}
        </Button>
      )}
      {error ? (
        <div className={styles.error} role="alert">
          {error}
        </div>
      ) : null}
    </div>
  );
}
