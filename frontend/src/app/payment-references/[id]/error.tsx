"use client";

import { useEffect } from "react";

import { BlueprintFrame } from "@/components/atoms/BlueprintFrame";
import { Button } from "@/components/atoms/Button";
import { ButtonLink } from "@/components/atoms/ButtonLink";
import { Heading } from "@/components/atoms/Heading";
import { Text } from "@/components/atoms/Text";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    console.error("[payment-reference detail] render failed", error);
  }, [error]);

  return (
    <BlueprintFrame style={{ padding: "32px 28px", display: "flex", flexDirection: "column", gap: 10 }}>
      <Heading level={2}>No se pudo cargar la referencia</Heading>
      <Text muted>Comprueba que el API esté en marcha y vuelve a intentarlo.</Text>
      <Text size="sm" muted technical>
        {error.message}
      </Text>
      <div style={{ display: "flex", gap: 8 }}>
        <Button variant="primary" blueprint onClick={reset}>
          Reintentar
        </Button>
        <ButtonLink href="/payment-references" variant="secondary">
          Volver
        </ButtonLink>
      </div>
    </BlueprintFrame>
  );
}
