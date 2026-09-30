import type { ReactNode } from "react";

import { Text } from "@/components/atoms/Text";

import styles from "./EmptyState.module.css";

export interface EmptyStateProps {
  title: string;
  description: string;
  /** An escape hatch such as "clear filters", supplied by the caller. */
  action?: ReactNode;
}

export function EmptyState({ title, description, action }: EmptyStateProps) {
  return (
    <div className={styles.wrapper}>
      <div className={styles.title}>{title}</div>
      <Text muted className={styles.description}>
        {description}
      </Text>
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  );
}
