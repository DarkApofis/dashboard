import type { ReactNode } from "react";

import { Heading } from "@/components/atoms/Heading";

import styles from "./PageHeader.module.css";

export interface PageHeaderProps {
  kicker: string;
  title: string;
  /** The one primary action of the screen. */
  action?: ReactNode;
}

export function PageHeader({ kicker, title, action }: PageHeaderProps) {
  return (
    <div className={styles.wrapper}>
      <div>
        <Heading level={6} size="kicker">
          {kicker}
        </Heading>
        <Heading level={1} size="display">
          {title}
        </Heading>
      </div>
      {action}
    </div>
  );
}
