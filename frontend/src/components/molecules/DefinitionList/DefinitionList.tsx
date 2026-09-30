import type { ReactNode } from "react";

import { cx } from "@/lib/format";

import styles from "./DefinitionList.module.css";

export interface DefinitionItem {
  term: string;
  value: ReactNode;
  /** Extra class for the value, such as a muted or highlighted date. */
  valueClassName?: string;
}

export interface DefinitionListProps {
  items: DefinitionItem[];
}

/** The metadata grid of the detail panel: API field names against their values. */
export function DefinitionList({ items }: DefinitionListProps) {
  return (
    <dl className={styles.list}>
      {items.map((item) => (
        <div key={item.term} style={{ display: "contents" }}>
          <dt className={cx(styles.term, "text-muted")}>{item.term}</dt>
          <dd className={cx(styles.value, item.valueClassName)}>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
