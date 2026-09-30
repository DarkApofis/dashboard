import { z } from "zod";

/**
 * Form validation, kept in step with the domain rules the Go entity enforces
 * (internal/domain/paymentref). Validating here is a courtesy that saves a round
 * trip; the API remains the authority, and its errors are surfaced too.
 */

const MAX_EXTERNAL_REFERENCE = 64;
const MAX_DESCRIPTION = 500;
/** NUMERIC(14,2): twelve integer digits and two decimals. */
const MAX_AMOUNT = 999_999_999_999.99;

const AMOUNT_PATTERN = /^\d{1,12}([.,]\d{1,2})?$/;

export const createReferenceSchema = z.object({
  business_id: z.coerce
    .number({ invalid_type_error: "Selecciona una empresa." })
    .int("Selecciona una empresa.")
    .positive("Selecciona una empresa."),

  external_reference: z
    .string()
    .trim()
    .min(1, "Requerido.")
    .max(MAX_EXTERNAL_REFERENCE, `Máximo ${MAX_EXTERNAL_REFERENCE} caracteres.`)
    .refine((value) => !/\s/.test(value), "No puede contener espacios."),

  // Kept as a string end to end so the value never passes through a float.
  amount: z
    .string()
    .trim()
    .min(1, "Requerido.")
    .refine((value) => AMOUNT_PATTERN.test(value), "Usa un número con hasta 2 decimales.")
    .transform((value) => value.replace(",", "."))
    .refine((value) => Number(value) > 0, "Debe ser mayor a 0.")
    .refine((value) => Number(value) <= MAX_AMOUNT, "Excede el máximo admitido."),

  description: z
    .string()
    .trim()
    .max(MAX_DESCRIPTION, `Máximo ${MAX_DESCRIPTION} caracteres.`)
    .optional(),

  expires_at: z
    .string()
    .trim()
    .min(1, "Requerido.")
    .refine((value) => !Number.isNaN(new Date(value).getTime()), "Fecha inválida.")
    .refine((value) => new Date(value).getTime() > Date.now(), "Debe ser una fecha futura."),
});

export type CreateReferenceInput = z.infer<typeof createReferenceSchema>;

/** The form fields, so an error map cannot name a field that does not exist. */
export type CreateReferenceField = keyof CreateReferenceInput;

/** Flattens a zod error into one message per field. */
export function fieldErrorsOf(error: z.ZodError): Partial<Record<CreateReferenceField, string>> {
  const result: Partial<Record<CreateReferenceField, string>> = {};
  for (const issue of error.issues) {
    const field = issue.path[0];
    if (typeof field === "string" && !(field in result)) {
      result[field as CreateReferenceField] = issue.message;
    }
  }
  return result;
}
