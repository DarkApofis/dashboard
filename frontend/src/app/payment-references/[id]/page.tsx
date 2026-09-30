import Link from "next/link";
import { notFound } from "next/navigation";

import { ButtonLink } from "@/components/atoms/ButtonLink";
import { Icon } from "@/components/atoms/Icon";
import { Toast } from "@/components/molecules/Toast";
import { ReferenceDetailCard } from "@/components/organisms/ReferenceDetailCard";
import { DetailTemplate } from "@/components/templates/DetailTemplate";
import { indexBusinesses, listBusinesses } from "@/features/businesses/api";
import { getPaymentReference } from "@/features/payment-references/api";
import { PAYMENT_LINK_BASE, WEBHOOK_SIMULATOR_ENABLED } from "@/lib/config";
import { isApiError } from "@/lib/http-client";

/**
 * The detail screen.
 *
 * Everything here is read-only and server-rendered; only the action zone, which can
 * cancel the reference or poll for a payment, ships JavaScript.
 */
export const dynamic = "force-dynamic";

export default async function PaymentReferenceDetailPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const [{ id }, query] = await Promise.all([params, searchParams]);

  const numericId = Number.parseInt(id, 10);
  if (!Number.isFinite(numericId) || numericId <= 0) {
    notFound();
  }

  const reference = await getPaymentReference(numericId).catch((error: unknown) => {
    // A missing reference is a 404 page, not an error boundary: the URL is simply wrong.
    if (isApiError(error) && error.isNotFound) {
      notFound();
    }
    throw error;
  });

  const businesses = await listBusinesses();
  const businessName = indexBusinesses(businesses).get(reference.business_id) ?? "—";

  const justCreated = query.created === "1";

  return (
    <>
      <DetailTemplate
        breadcrumb={
          <div style={{ position: "absolute", left: 24, marginTop: 4 }}>
            <ButtonLink href="/payment-references" variant="ghost">
              ← Referencias
            </ButtonLink>
          </div>
        }
      >
        <ReferenceDetailCard
          reference={reference}
          businessName={businessName}
          paymentLink={`${PAYMENT_LINK_BASE}/${reference.external_reference}`}
          simulatorEnabled={WEBHOOK_SIMULATOR_ENABLED}
          closeSlot={
            <Link href="/payment-references" className="btn btn-icon btn-ghost" aria-label="Cerrar">
              <Icon name="close" size={18} />
            </Link>
          }
        />
      </DetailTemplate>
      {justCreated ? <Toast message={`201 Created · ${reference.external_reference}`} /> : null}
    </>
  );
}
