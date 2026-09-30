import { ButtonLink } from "@/components/atoms/ButtonLink";
import { EmptyState } from "@/components/molecules/EmptyState";

export default function ReferenceNotFound() {
  return (
    <EmptyState
      title="Referencia no encontrada"
      description="No existe una referencia de pago con ese identificador."
      action={
        <ButtonLink href="/payment-references" variant="secondary">
          Volver a las referencias
        </ButtonLink>
      }
    />
  );
}
