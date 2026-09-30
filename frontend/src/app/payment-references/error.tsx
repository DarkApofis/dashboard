"use client";

import { useEffect } from "react";

import { Button } from "@/components/atoms/Button";
import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";
import { Heading } from "@/components/atoms/Heading";
import { Text } from "@/components/atoms/Text";

/**
 * The error state of the listing.
 *
 * The likeliest cause by far is the API not being up, so the message says that instead
 * of showing a stack trace, and offers a retry that re-runs the server render.
 */
export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error("[payment-references] render failed", error);
  }, [error]);

  return (
    <BlueprintFrame style={{ padding: "32px 28px", display: "flex", flexDirection: "column", gap: 10 }}>
      <Heading level={2}>No se pudieron cargar las referencias</Heading>
      <Text muted>
        Comprueba que el API esté en marcha en la dirección configurada en NEXT_PUBLIC_API_URL y vuelve a intentarlo.
      </Text>
      <Text size="sm" muted technical>
        {error.message}
      </Text>
      <div>
        <Button variant="primary" blueprint onClick={reset}>
          Reintentar
        </Button>
      </div>
    </BlueprintFrame>
  );
}
