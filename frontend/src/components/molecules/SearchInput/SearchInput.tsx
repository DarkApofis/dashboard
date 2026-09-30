"use client";

import { useEffect, useState, type FormEvent } from "react";

import { Icon } from "@/components/atoms/Icon";
import { Input } from "@/components/atoms/Input";
import { VisuallyHidden } from "@/components/atoms/VisuallyHidden";

import styles from "./SearchInput.module.css";

export interface SearchInputProps {
  id: string;
  label: string;
  placeholder?: string;
  /** The value currently applied, which comes from the URL. */
  value: string;
  /** Called on submit, not on every keystroke: each search is a navigation. */
  onSubmit: (value: string) => void;
  disabled?: boolean;
}

/**
 * A search box that commits on submit.
 *
 * Deliberately not debounced-as-you-type: the value it filters by ends up in the URL,
 * and pushing a history entry per keystroke would make the back button useless.
 */
export function SearchInput({ id, label, placeholder, value, onSubmit, disabled = false }: SearchInputProps) {
  const [draft, setDraft] = useState(value);

  // Keep the box in step when the URL changes from elsewhere, such as "clear filters".
  useEffect(() => {
    setDraft(value);
  }, [value]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit(draft);
  }

  return (
    <form className={styles.wrapper} onSubmit={handleSubmit} role="search">
      <label htmlFor={id}>
        <VisuallyHidden>{label}</VisuallyHidden>
      </label>
      <Icon name="search" className={styles.icon} />
      <Input
        id={id}
        name={id}
        type="search"
        className={styles.input}
        placeholder={placeholder}
        value={draft}
        disabled={disabled}
        onChange={(event) => setDraft(event.target.value)}
      />
    </form>
  );
}
