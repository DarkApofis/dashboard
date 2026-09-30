import { Suspense } from "react";

import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";
import { ButtonLink } from "@/components/atoms/ButtonLink";
import { Icon } from "@/components/atoms/Icon";
import { Spinner } from "@/components/atoms/Spinner";
import { Text } from "@/components/atoms/Text";
import { EmptyState } from "@/components/molecules/EmptyState";
import { Pagination } from "@/components/molecules/Pagination";
import { PageHeader } from "@/components/organisms/PageHeader";
import { ReferencesFilters } from "@/components/organisms/ReferencesFilters";
import { ReferencesTable } from "@/components/organisms/ReferencesTable";
import { RequestHintLine } from "@/components/organisms/RequestHintLine";
import { StatsStrip } from "@/components/organisms/StatsStrip";
import { ListTemplate } from "@/components/templates/ListTemplate";
import { indexBusinesses, listBusinesses } from "@/features/businesses/api";
import { countReferencesByStatus, listPaymentReferences, toQueryString } from "@/features/payment-references/api";
import { parseStatus } from "@/features/payment-references/status";
import type { ReferenceQuery } from "@/features/payment-references/types";
import { PAGE_SIZE } from "@/lib/config";

/**
 * The listing.
 *
 * A Server Component: the filters arrive in the URL, the data is fetched on the server,
 * and the browser receives HTML. There is no client-side cache to invalidate and no
 * state library, which is the whole reason the filters live in the query string.
 *
 * The loading state is a Suspense boundary inside the page rather than a loading.tsx
 * next to it. A route-level loading.tsx flushes the response before the page resolves,
 * which pins the status at 200 and makes notFound() in the sibling detail route unable
 * to answer 404. Streaming from here keeps the header instant, gives the table a
 * skeleton, and leaves the status codes honest.
 */
export const dynamic = "force-dynamic";

type SearchParams = Record<string, string | string[] | undefined>;

function first(value: string | string[] | undefined): string | undefined {
  return Array.isArray(value) ? value[0] : value;
}

function toPositiveInt(value: string | undefined): number | undefined {
  if (!value) {
    return undefined;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

function buildQuery(params: SearchParams): ReferenceQuery {
  return {
    status: parseStatus(first(params.status)),
    businessId: toPositiveInt(first(params.business_id)),
    externalReference: first(params.external_reference)?.trim() || undefined,
    limit: PAGE_SIZE,
    offset: Math.max(Number.parseInt(first(params.offset) ?? "0", 10) || 0, 0),
  };
}

export default async function PaymentReferencesPage({ searchParams }: { searchParams: Promise<SearchParams> }) {
  const params = await searchParams;
  const query = buildQuery(params);

  return (
    <ListTemplate
      header={
        <PageHeader
          kicker="Referencias de pago"
          title="Cobros generados"
          action={
            <ButtonLink href="/payment-references/new" variant="primary" blueprint>
              <Icon name="plus" />
              Nueva referencia
            </ButtonLink>
          }
        />
      }
      table={
        <Suspense key={JSON.stringify(query)} fallback={<ListSkeleton />}>
          <ReferencesSection query={query} params={params} />
        </Suspense>
      }
      footer={<RequestHintLine request={`GET /payment-references${toQueryString(query)}`} />}
    />
  );
}

/**
 * Everything that needs data. Kept in the route file rather than promoted to an
 * organism, because organisms in this codebase receive data as props and never fetch.
 */
async function ReferencesSection({ query, params }: { query: ReferenceQuery; params: SearchParams }) {
  const [page, businesses, counts] = await Promise.all([
    listPaymentReferences(query),
    listBusinesses(),
    countReferencesByStatus({ businessId: query.businessId, externalReference: query.externalReference }),
  ]);

  const isFiltered = Boolean(query.status || query.businessId || query.externalReference);

  const hrefForOffset = (next: number) => {
    const search = new URLSearchParams();
    for (const [key, value] of Object.entries(params)) {
      const single = first(value);
      if (single && key !== "offset") {
        search.set(key, single);
      }
    }
    if (next > 0) {
      search.set("offset", String(next));
    }
    const rendered = search.toString();
    return rendered ? `/payment-references?${rendered}` : "/payment-references";
  };

  return (
    <>
      <StatsStrip counts={counts} filtered={isFiltered} />

      <ReferencesFilters businesses={businesses} counts={counts} shown={page.items.length} total={page.total} />

      <ReferencesTable
        references={page.items}
        businessNames={indexBusinesses(businesses)}
        emptyState={
          <EmptyState
            title={isFiltered ? "Sin resultados" : "Todavía no hay referencias"}
            description={
              isFiltered
                ? "No hay referencias que coincidan con los filtros actuales."
                : "Crea la primera referencia de pago para empezar a cobrar."
            }
            action={
              isFiltered ? (
                <ButtonLink href="/payment-references" variant="secondary">
                  Limpiar filtros
                </ButtonLink>
              ) : (
                <ButtonLink href="/payment-references/new" variant="primary" blueprint>
                  <Icon name="plus" />
                  Nueva referencia
                </ButtonLink>
              )
            }
          />
        }
      />

      <Pagination total={page.total} limit={page.limit} offset={page.offset} hrefForOffset={hrefForOffset} />
    </>
  );
}

/** What the viewer sees while the section above is still resolving on the server. */
function ListSkeleton() {
  return (
    <BlueprintFrame
      style={{ display: "flex", alignItems: "center", gap: 10, padding: "48px 24px", justifyContent: "center" }}
      aria-busy="true"
    >
      <Spinner large />
      <Text muted>Cargando referencias…</Text>
    </BlueprintFrame>
  );
}
