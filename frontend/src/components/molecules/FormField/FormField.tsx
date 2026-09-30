import type { ReactNode } from "react";

import { Label } from "@/components/atoms/Label";
import { Text } from "@/components/atoms/Text";

import styles from "./FormField.module.css";

export interface FormFieldProps {
  /** Must match the id of the control passed as children. */
  htmlFor: string;
  label: string;
  error?: string;
  hint?: string;
  children: ReactNode;
}

/**
 * A labelled control with its error message. Purely presentational: it does not know
 * where the error came from, so the same component renders a client-side validation
 * failure and one the API returned.
 */
export function FormField({ htmlFor, label, error, hint, children }: FormFieldProps) {
  const errorId = `${htmlFor}-error`;
  const hintId = `${htmlFor}-hint`;

  return (
    <div className="field">
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {hint && !error ? (
        <Text as="div" size="xs" muted className={styles.hint} id={hintId}>
          {hint}
        </Text>
      ) : null}
      {error ? (
        <div className={styles.error} id={errorId} role="alert">
          {error}
        </div>
      ) : null}
    </div>
  );
}
