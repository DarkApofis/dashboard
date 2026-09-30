/**
 * Presentation helpers. Formatting only: no arithmetic on money happens anywhere in
 * the frontend.
 */

const COP = new Intl.NumberFormat("es-CO", {
  style: "currency",
  currency: "COP",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

const DATE_TIME = new Intl.DateTimeFormat("es-CO", {
  day: "2-digit",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "UTC",
});

const DATE_TIME_WITH_YEAR = new Intl.DateTimeFormat("es-CO", {
  day: "2-digit",
  month: "short",
  year: "numeric",
  hour: "2-digit",
  minute: "2-digit",
  timeZone: "UTC",
});

/**
 * Formats an amount as Colombian pesos.
 *
 * The amount arrives from the API as a JSON number, and it is only ever formatted,
 * never added up: JavaScript numbers are binary floats, so summing them here would
 * reintroduce exactly the error the backend avoids by storing NUMERIC. Totals belong
 * to the API.
 */
export function formatCOP(amount: number): string {
  return COP.format(amount);
}

/**
 * Formats a timestamp in UTC, which is how the API stores and returns every one of
 * them. The suffix is explicit so a reader never has to guess the zone.
 */
export function formatDateTime(iso: string | null | undefined, options: { withYear?: boolean } = {}): string {
  if (!iso) {
    return "—";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }
  const formatter = options.withYear ? DATE_TIME_WITH_YEAR : DATE_TIME;
  return `${formatter.format(date)} UTC`;
}

/** Turns an ISO timestamp into the value a datetime-local input expects. */
export function toDateTimeLocalValue(iso: string | null | undefined): string {
  if (!iso) {
    return "";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/**
 * Turns a datetime-local value, which the browser reports in the viewer's own zone,
 * into the UTC instant the API expects.
 */
export function dateTimeLocalToISO(value: string): string | null {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return null;
  }
  return date.toISOString();
}

const THREE_DAYS_MS = 3 * 24 * 60 * 60 * 1000;

/**
 * Whether a reference is close enough to expiring to deserve attention. The design
 * keeps that date at full contrast instead of muting it.
 */
export function isExpiringSoon(iso: string, now: Date = new Date()): boolean {
  const expiresAt = new Date(iso).getTime();
  if (Number.isNaN(expiresAt)) {
    return false;
  }
  const remaining = expiresAt - now.getTime();
  return remaining > 0 && remaining < THREE_DAYS_MS;
}

/** Joins class names, dropping the falsy ones. */
export function cx(...values: Array<string | false | null | undefined>): string {
  return values.filter(Boolean).join(" ");
}
