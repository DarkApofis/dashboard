"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Label } from "@/components/atoms/Label";

import styles from "./CopyableField.module.css";

export interface CopyableFieldProps {
  id: string;
  label: string;
  value: string;
}

/** A read-only value with a copy button, for the shareable link. */
export function CopyableField({ id, label, value }: CopyableFieldProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) {
      return;
    }
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
    } catch {
      // Clipboard access can be refused (insecure origin, denied permission). The
      // input is selectable, so the value is still reachable by hand.
      setCopied(false);
    }
  }

  return (
    <div className="field">
      <Label htmlFor={id}>{label}</Label>
      <div className={styles.row}>
        <Input id={id} readOnly value={value} className={styles.input} onFocus={(event) => event.target.select()} />
        <Button variant="secondary" onClick={copy}>
          {copied ? "Copiado" : "Copiar"}
        </Button>
      </div>
    </div>
  );
}
