"use client";

import { useEffect, useState } from "react";

import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";
import { cx } from "@/lib/format";

import styles from "./Toast.module.css";

export interface ToastProps {
  message: string;
  /** Milliseconds before it disappears. */
  duration?: number;
}

/** A transient confirmation, announced politely to assistive technology. */
export function Toast({ message, duration = 2600 }: ToastProps) {
  const [visible, setVisible] = useState(true);

  useEffect(() => {
    setVisible(true);
    const timer = setTimeout(() => setVisible(false), duration);
    return () => clearTimeout(timer);
  }, [message, duration]);

  if (!visible) {
    return null;
  }

  return (
    <BlueprintFrame className={cx(styles.toast, "elev-lg")} role="status" aria-live="polite">
      {message}
    </BlueprintFrame>
  );
}
