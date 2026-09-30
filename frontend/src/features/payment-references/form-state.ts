import type { CreateReferenceField } from "./schemas";

/**
 * The shapes the Server Actions return.
 *
 * They live apart from actions.ts because a "use server" module may only export async
 * functions: anything else in it becomes a build error. Types are erased, but the
 * initial state is a real value, so it belongs here.
 */

export interface CreateFormState {
  status: "idle" | "error" | "success";
  /** A message about the request as a whole, when no single field owns the problem. */
  message?: string;
  fieldErrors?: Partial<Record<CreateReferenceField, string>>;
  /** What the user typed, so a rejected form comes back filled in. */
  values?: Record<string, string>;
  /** Set on success so the caller can navigate to the new reference. */
  createdId?: number;
}

export const initialCreateFormState: CreateFormState = { status: "idle" };

/** What a one-shot action (cancel, simulate) reports back. */
export interface ActionResult {
  ok: boolean;
  message?: string;
}
