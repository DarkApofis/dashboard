"use client";

import { useActionState, useCallback, useEffect, useState, useTransition } from "react";
import { useRouter, useSearchParams } from "next/navigation";

import { PAGE_SIZE } from "@/lib/config";

import { cancelReferenceAction, createReferenceAction, simulatePaymentAction } from "./actions";
import { initialCreateFormState, type ActionResult } from "./form-state";
import type { PaymentReferenceStatus } from "./types";

/**
 * The client-side half of the feature.
 *
 * Reading data is not here: pages are Server Components that fetch on the server, so
 * there is no client cache to keep in sync and no state library to install. What does
 * need the client is the URL (the filters live there, so a filtered view is a
 * shareable link), the mutations, and the polling that keeps a pending detail fresh.
 */

export interface ReferenceFilterValues {
  status: PaymentReferenceStatus | "all";
  businessId: string;
  externalReference: string;
}

/**
 * Reads the filters from the URL and writes them back to it. The URL is the single
 * source of truth: reloading, sharing or going back all reproduce the same view.
 * Changing a filter resets the offset, because page 3 of a different filter is
 * meaningless.
 */
export function useReferenceFilters() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [isPending, startTransition] = useTransition();

  const values: ReferenceFilterValues = {
    status: (searchParams.get("status") as PaymentReferenceStatus | null) ?? "all",
    businessId: searchParams.get("business_id") ?? "all",
    externalReference: searchParams.get("external_reference") ?? "",
  };

  const push = useCallback(
    (mutate: (params: URLSearchParams) => void) => {
      const params = new URLSearchParams(searchParams.toString());
      mutate(params);
      params.delete("offset");
      const query = params.toString();
      startTransition(() => {
        router.push(query ? `/payment-references?${query}` : "/payment-references");
      });
    },
    [router, searchParams],
  );

  const setStatus = useCallback(
    (status: PaymentReferenceStatus | "all") => {
      push((params) => {
        if (status === "all") {
          params.delete("status");
        } else {
          params.set("status", status);
        }
      });
    },
    [push],
  );

  const setBusiness = useCallback(
    (businessId: string) => {
      push((params) => {
        if (businessId === "all") {
          params.delete("business_id");
        } else {
          params.set("business_id", businessId);
        }
      });
    },
    [push],
  );

  const setExternalReference = useCallback(
    (value: string) => {
      push((params) => {
        const trimmed = value.trim();
        if (trimmed) {
          params.set("external_reference", trimmed);
        } else {
          params.delete("external_reference");
        }
      });
    },
    [push],
  );

  const clear = useCallback(() => {
    startTransition(() => {
      router.push("/payment-references");
    });
  }, [router]);

  return { values, isPending, setStatus, setBusiness, setExternalReference, clear };
}

/** Builds the href for a page of the listing, preserving every active filter. */
export function usePaginationHref() {
  const searchParams = useSearchParams();

  return useCallback(
    (offset: number) => {
      const params = new URLSearchParams(searchParams.toString());
      if (offset > 0) {
        params.set("offset", String(offset));
      } else {
        params.delete("offset");
      }
      const query = params.toString();
      return query ? `/payment-references?${query}` : "/payment-references";
    },
    [searchParams],
  );
}

export { PAGE_SIZE };

/**
 * Drives the create form. On success it navigates to the new reference, which is also
 * where the confirmation toast is shown.
 */
export function useCreateReference() {
  const router = useRouter();
  const [state, formAction, isPending] = useActionState(createReferenceAction, initialCreateFormState);

  useEffect(() => {
    if (state.status === "success" && state.createdId) {
      router.push(`/payment-references/${state.createdId}?created=1`);
    }
  }, [router, state]);

  return { state, formAction, isPending };
}

/** Runs the cancellation and reports what the API answered. */
export function useCancelReference(id: number) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [result, setResult] = useState<ActionResult | null>(null);

  const cancel = useCallback(() => {
    startTransition(async () => {
      const outcome = await cancelReferenceAction(id);
      setResult(outcome);
      if (outcome.ok) {
        router.refresh();
      }
    });
  }, [id, router]);

  const dismiss = useCallback(() => setResult(null), []);

  return { cancel, dismiss, isPending, result };
}

/** Posts the simulated provider notification. */
export function useSimulatePayment(id: number) {
  const router = useRouter();
  const [isPending, startTransition] = useTransition();
  const [result, setResult] = useState<ActionResult | null>(null);

  const simulate = useCallback(() => {
    startTransition(async () => {
      const outcome = await simulatePaymentAction(id);
      setResult(outcome);
      if (outcome.ok) {
        router.refresh();
      }
    });
  }, [id, router]);

  const dismiss = useCallback(() => setResult(null), []);

  return { simulate, dismiss, isPending, result };
}

/**
 * Re-renders the server component on an interval while the reference can still change.
 *
 * A payment arrives through a webhook, not through anything the viewer does, so a
 * pending detail would otherwise sit there stale. Polling stops the moment the state
 * is terminal, and the tab being hidden pauses it.
 */
export function useAutoRefresh(enabled: boolean, intervalMs = 10_000) {
  const router = useRouter();

  useEffect(() => {
    if (!enabled) {
      return;
    }
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") {
        router.refresh();
      }
    }, intervalMs);
    return () => clearInterval(timer);
  }, [enabled, intervalMs, router]);
}
