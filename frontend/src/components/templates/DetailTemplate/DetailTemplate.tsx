import type { ReactNode } from "react";

import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";

import styles from "./DetailTemplate.module.css";

export interface DetailTemplateProps {
  children: ReactNode;
  /** A back link rendered above the panel. */
  breadcrumb?: ReactNode;
}

/** The detail screen: one framed panel, at the width the drawer had. */
export function DetailTemplate({ children, breadcrumb }: DetailTemplateProps) {
  return (
    <div className={styles.wrapper}>
      <BlueprintFrame as="article" className={styles.panel} aria-label="Detalle de la referencia">
        {children}
      </BlueprintFrame>
      {breadcrumb}
    </div>
  );
}
