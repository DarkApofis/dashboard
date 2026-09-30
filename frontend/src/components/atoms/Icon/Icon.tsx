import type { SVGProps } from "react";

/**
 * The icons the design uses, as inline Lucide paths.
 *
 * Lucide at stroke-width 1.5 is what the design system prescribes, and only the five
 * shapes the screens actually need are here: pulling in the whole icon package for
 * five paths would be a dependency in exchange for nothing.
 */
export type IconName = "plus" | "search" | "close" | "check" | "arrowRight" | "alert" | "copy";

const PATHS: Record<IconName, string[]> = {
  plus: ["M12 5v14M5 12h14"],
  search: ["m20 20-3.5-3.5"],
  close: ["M6 6l12 12M18 6 6 18"],
  check: ["M20 6 9 17l-5-5"],
  arrowRight: ["M5 12h14", "m12 5 7 7-7 7"],
  alert: ["M12 9v4", "M12 17h.01", "M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0Z"],
  copy: ["M9 9h10v10H9z", "M5 15V5h10"],
};

export interface IconProps extends Omit<SVGProps<SVGSVGElement>, "name"> {
  name: IconName;
  size?: number;
}

export function Icon({ name, size = 16, ...rest }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {name === "search" ? <circle cx="11" cy="11" r="7" /> : null}
      {PATHS[name].map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}
