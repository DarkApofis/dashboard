"use client";

import Link from "next/link";

import { Button } from "@/components/atoms/Button";
import { Heading } from "@/components/atoms/Heading";
import { Icon } from "@/components/atoms/Icon";
import { Input } from "@/components/atoms/Input";
import { Select } from "@/components/atoms/Select";
import { Spinner } from "@/components/atoms/Spinner";
import { FormField } from "@/components/molecules/FormField";
import type { Business } from "@/features/businesses/types";
import { useCreateReference } from "@/features/payment-references/hooks";

import styles from "./ReferenceForm.module.css";

export interface ReferenceFormProps {
  businesses: Business[];
  /** Prefilled expiry, computed on the server so the markup is deterministic. */
  defaultExpiresAt: string;
  /** Where the cancel link goes back to. */
  cancelHref: string;
}

/**
 * The create form.
 *
 * Submitting runs a Server Action, so the form works the same whether or not the
 * JavaScript has loaded, and the API URL never reaches the browser. Field errors come
 * from two places and render identically: the zod schema, and the API rejecting the
 * request (a duplicate external_reference is only knowable there).
 */
export function ReferenceForm({ businesses, defaultExpiresAt, cancelHref }: ReferenceFormProps) {
  const { state, formAction, isPending } = useCreateReference();
  const errors = state.fieldErrors ?? {};
  const values = state.values ?? {};

  return (
    <form
      action={formAction}
      className="dialog blueprint"
      style={{ width: "min(520px, 100%)", gap: 14, padding: 22 }}
      // Validation is the schema's job and the API's, and both render through
      // FormField. Native constraint validation would pop an unstyled browser bubble
      // and stop the submission before the action ever runs.
      noValidate
    >
      <i className="corner tl" aria-hidden="true" />
      <i className="corner tr" aria-hidden="true" />
      <i className="corner bl" aria-hidden="true" />
      <i className="corner br" aria-hidden="true" />

      <div className={styles.header}>
        <div>
          <Heading level={6} size="kicker">
            POST /payment-references
          </Heading>
          <div className="dialog-title" style={{ fontSize: 24 }}>
            Nueva referencia de pago
          </div>
        </div>
        <Link href={cancelHref} className="btn btn-icon btn-ghost" aria-label="Cerrar">
          <Icon name="close" size={18} />
        </Link>
      </div>

      {state.message ? (
        <div className={styles.summaryError} role="alert">
          {state.message}
        </div>
      ) : null}

      <FormField htmlFor="business_id" label="Empresa (business_id)" error={errors.business_id}>
        <Select
          id="business_id"
          name="business_id"
          defaultValue={values.business_id ?? String(businesses[0]?.id ?? "")}
          invalid={Boolean(errors.business_id)}
        >
          {businesses.map((business) => (
            <option key={business.id} value={business.id}>
              {business.name}
            </option>
          ))}
        </Select>
      </FormField>

      <div className={styles.pair}>
        <FormField htmlFor="external_reference" label="external_reference" error={errors.external_reference}>
          <Input
            id="external_reference"
            name="external_reference"
            placeholder="INV-10024"
            maxLength={64}
            defaultValue={values.external_reference ?? ""}
            invalid={Boolean(errors.external_reference)}
          />
        </FormField>

        <FormField htmlFor="amount" label="amount (COP)" error={errors.amount}>
          <Input
            id="amount"
            name="amount"
            type="text"
            inputMode="decimal"
            placeholder="125000.50"
            style={{ fontVariantNumeric: "tabular-nums" }}
            defaultValue={values.amount ?? ""}
            invalid={Boolean(errors.amount)}
          />
        </FormField>
      </div>

      <FormField htmlFor="description" label="description" error={errors.description}>
        <Input
          id="description"
          name="description"
          placeholder="Factura 10024"
          maxLength={500}
          defaultValue={values.description ?? ""}
          invalid={Boolean(errors.description)}
        />
      </FormField>

      <FormField
        htmlFor="expires_at"
        label="expires_at"
        error={errors.expires_at}
        hint="En tu zona horaria; se envía al API en UTC."
      >
        <Input
          id="expires_at"
          name="expires_at"
          type="datetime-local"
          defaultValue={values.expires_at ?? defaultExpiresAt}
          invalid={Boolean(errors.expires_at)}
        />
      </FormField>

      <div className="dialog-actions">
        <Link href={cancelHref} className="btn btn-secondary">
          Cancelar
        </Link>
        <Button type="submit" variant="primary" blueprint disabled={isPending}>
          {isPending ? <Spinner label="Creando" /> : null}
          Crear referencia
        </Button>
      </div>
    </form>
  );
}
