/**
 * The single place that talks HTTP to the Go API.
 *
 * Every error the API can return shares one envelope,
 * `{ "error": { "code", "message", "details" } }`, so this module turns any failure
 * into an ApiError carrying that stable code. Callers switch on the code, never on
 * the message, and never on the status alone.
 */
import { API_URL } from "@/lib/config";

/** The error envelope the API answers with. */
interface ApiErrorEnvelope {
  error?: {
    code?: string;
    message?: string;
    details?: Record<string, string>;
  };
}

/** Anything the API rejected, or could not be reached at all. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: Record<string, string>;
  /** The backend request id, when it answered. Makes a 500 traceable in its logs. */
  readonly requestId: string | null;

  constructor(params: {
    status: number;
    code: string;
    message: string;
    details?: Record<string, string>;
    requestId?: string | null;
  }) {
    super(params.message);
    this.name = "ApiError";
    this.status = params.status;
    this.code = params.code;
    this.details = params.details ?? {};
    this.requestId = params.requestId ?? null;
  }

  /** True when the API could not be reached, which is different from it saying no. */
  get isUnreachable(): boolean {
    return this.status === 0;
  }

  get isNotFound(): boolean {
    return this.status === 404;
  }

  /** 409: the request clashes with the current state of the resource. */
  get isConflict(): boolean {
    return this.status === 409;
  }

  /** 422: well-formed request that breaks a business rule. */
  get isUnprocessable(): boolean {
    return this.status === 422;
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

type QueryValue = string | number | undefined | null;

export interface ApiRequestOptions {
  method?: "GET" | "POST";
  /** Sent as JSON. */
  body?: unknown;
  /** Undefined and null entries are dropped rather than sent empty. */
  searchParams?: Record<string, QueryValue>;
  signal?: AbortSignal;
}

/**
 * Performs one API call and returns the parsed body.
 *
 * Reads are never cached: a dashboard that shows a stale payment status is worse than
 * one that is a few milliseconds slower.
 */
export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const url = new URL(API_URL + path);
  for (const [key, value] of Object.entries(options.searchParams ?? {})) {
    if (value !== undefined && value !== null && value !== "") {
      url.searchParams.set(key, String(value));
    }
  }

  const headers: Record<string, string> = { Accept: "application/json" };
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  let response: Response;
  try {
    response = await fetch(url, {
      method: options.method ?? "GET",
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      cache: "no-store",
      signal: options.signal,
    });
  } catch (cause) {
    throw new ApiError({
      status: 0,
      code: "api_unreachable",
      message: "No se pudo conectar con el API. Comprueba que esté en marcha.",
    });
  }

  const requestId = response.headers.get("x-request-id");

  if (response.status === 204) {
    return undefined as T;
  }

  const raw = await response.text();
  let parsed: unknown = null;
  if (raw.length > 0) {
    try {
      parsed = JSON.parse(raw);
    } catch {
      parsed = null;
    }
  }

  if (!response.ok) {
    const envelope = (parsed ?? {}) as ApiErrorEnvelope;
    throw new ApiError({
      status: response.status,
      code: envelope.error?.code ?? "unexpected_error",
      message: envelope.error?.message ?? `El API respondió ${response.status}.`,
      details: envelope.error?.details,
      requestId,
    });
  }

  return parsed as T;
}
