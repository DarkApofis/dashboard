import { ReferenceForm } from "@/components/organisms/ReferenceForm";
import { DialogTemplate } from "@/components/templates/DialogTemplate";
import { listBusinesses } from "@/features/businesses/api";
import { toDateTimeLocalValue } from "@/lib/format";

/**
 * The create screen.
 *
 * The design opened this as a modal over the listing. It is a route here, so the form
 * can be linked to and reloaded, and it keeps the dialog treatment: the same framed
 * panel at the same width.
 */
export const dynamic = "force-dynamic";

/** A week out, which is the default the design suggested. */
const DEFAULT_EXPIRY_DAYS = 7;

export default async function NewPaymentReferencePage() {
  const businesses = await listBusinesses();

  const defaultExpiry = new Date();
  defaultExpiry.setDate(defaultExpiry.getDate() + DEFAULT_EXPIRY_DAYS);
  defaultExpiry.setHours(23, 59, 0, 0);

  return (
    <DialogTemplate dismissHref="/payment-references" dismissLabel="← Volver a las referencias">
      <ReferenceForm
        businesses={businesses}
        defaultExpiresAt={toDateTimeLocalValue(defaultExpiry.toISOString())}
        cancelHref="/payment-references"
      />
    </DialogTemplate>
  );
}
