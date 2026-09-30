"use client";

import { cx } from "@/lib/format";

import styles from "./SegmentedFilter.module.css";

export interface SegmentedFilterOption<T extends string> {
  value: T;
  label: string;
  count?: number;
}

export interface SegmentedFilterProps<T extends string> {
  options: ReadonlyArray<SegmentedFilterOption<T>>;
  value: T;
  onChange: (value: T) => void;
  disabled?: boolean;
  /** Names the group for assistive technology. */
  label: string;
}

/** The segmented control, with a count next to each option. */
export function SegmentedFilter<T extends string>({
  options,
  value,
  onChange,
  disabled = false,
  label,
}: SegmentedFilterProps<T>) {
  return (
    <div className={cx("seg", styles.group)} role="group" aria-label={label}>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          className={cx("seg-opt", styles.option)}
          aria-pressed={option.value === value}
          disabled={disabled}
          onClick={() => onChange(option.value)}
        >
          {option.label}
          {option.count === undefined ? null : <span className={styles.count}>{option.count}</span>}
        </button>
      ))}
    </div>
  );
}
