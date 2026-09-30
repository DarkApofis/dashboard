import type { ReactNode } from "react";

import { ButtonLink } from "@/components/atoms/ButtonLink";

import styles from "./DialogTemplate.module.css";

export interface DialogTemplateProps {
  children: ReactNode;
  /** Where the escape route goes back to. */
  dismissHref: string;
  dismissLabel: string;
}

/** A dialog-shaped screen: one framed panel, centred, with a way back. */
export function DialogTemplate({ children, dismissHref, dismissLabel }: DialogTemplateProps) {
  return (
    <div className={styles.wrapper}>
      {children}
      <ButtonLink href={dismissHref} variant="ghost">
        {dismissLabel}
      </ButtonLink>
    </div>
  );
}
