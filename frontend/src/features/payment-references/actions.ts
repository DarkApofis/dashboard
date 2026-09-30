"use server";

import { revalidatePath } from "next/cache";

import { ApiError, isApiError } from "@/lib/http-client";

import { cancelPaymentReference, createPaymentReference, getPaymentReference, simulateProviderPayment } from "./api";
import type { ActionResult, CreateFormState } from "./form-state";
import { createReferenceSchema, fieldErrorsOf, type CreateReferenceField } from "./schemas";

/**
 * Server Actions: every mutation runs on the server, so the API URL and any future
 * credential never reach the browser.
 *
 * This module may only export async functions. The state shapes these return live in
 * form-state.ts for that reason.
 */

/**
 * Maps an API rejection onto the field that caused it, so the message lands next to
 * the input instead of floating at the top of the form. Codes come from the Go domain
 * and are stable; the messages are not, which is why nothing switches on them.
 */
function apiErrorToFieldErrors(error: ApiError): Partial<Record<CreateReferenceField, string>> | null {
  switch (error.code) {
    case "duplicate_external_reference":
      return { external_reference: "Ya existe una referencia con este valor." };
    case "invalid_external_reference":
      return { external_reference: error.message };
    case "business_not_found":
    case "invalid_business_id":
      return { business_id: "La empresa seleccionada no existe." };
    case "invalid_expires_at":
      return { expires_at: "Debe ser una fecha futura." };
    case "invalid_amount":
    case "negative_amount":
    case "amount_scale_exceeded":
    case "amount_too_large":
    case "invalid_amount_format":
      return { amount: error.message };
    default:
      return null;
  }
}

export async function createReferenceAction(
  _previous: CreateFormState,
  formData: FormData,
): Promise<CreateFormState> {
  const values = {
    business_id: String(formData.get("business_id") ?? ""),
    external_reference: String(formData.get("external_reference") ?? ""),
    amount: String(formData.get("amount") ?? ""),
    description: String(formData.get("description") ?? ""),
    expires_at: String(formData.get("expires_at") ?? ""),
  };

  const parsed = createReferenceSchema.safeParse(values);
  if (!parsed.success) {
    return { status: "error", fieldErrors: fieldErrorsOf(parsed.error), values };
  }

  try {
    const created = await createPaymentReference({
      business_id: parsed.data.business_id,
      external_reference: parsed.data.external_reference,
      amount: parsed.data.amount,
      description: parsed.data.description || undefined,
      // The browser reports a local instant; the API works in UTC.
      expires_at: new Date(parsed.data.expires_at).toISOString(),
    });

    revalidatePath("/payment-references");
    return { status: "success", createdId: created.id };
  } catch (error) {
    if (isApiError(error)) {
      const fieldErrors = apiErrorToFieldErrors(error);
      if (fieldErrors) {
        return { status: "error", fieldErrors, values };
      }
      return { status: "error", message: error.message, values };
    }
    return {
      status: "error",
      message: "No se pudo crear la referencia. Inténtalo de nuevo.",
      values,
    };
  }
}

/**
 * Cancels a reference. A 409 is not a failure of the request but the API stating a
 * rule, so its message is passed through verbatim.
 */
export async function cancelReferenceAction(id: number): Promise<ActionResult> {
  try {
    const cancelled = await cancelPaymentReference(id);
    revalidatePath("/payment-references");
    revalidatePath(`/payment-references/${id}`);
    return { ok: true, message: `${cancelled.external_reference} cancelada.` };
  } catch (error) {
    if (isApiError(error)) {
      return { ok: false, message: error.message };
    }
    return { ok: false, message: "No se pudo cancelar la referencia." };
  }
}

/** Posts a fake provider notification for this reference. Review convenience only. */
export async function simulatePaymentAction(id: number): Promise<ActionResult> {
  try {
    const reference = await getPaymentReference(id);
    await simulateProviderPayment(reference);
    revalidatePath("/payment-references");
    revalidatePath(`/payment-references/${id}`);
    return { ok: true, message: "Webhook recibido. La referencia quedó pagada." };
  } catch (error) {
    if (isApiError(error)) {
      const hint =
        error.status === 401
          ? " El backend está exigiendo firma HMAC: deja WEBHOOK_SECRET vacío para usar el simulador."
          : "";
      return { ok: false, message: error.message + hint };
    }
    return { ok: false, message: "No se pudo simular el pago." };
  }
}
