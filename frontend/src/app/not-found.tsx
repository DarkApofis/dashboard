import { ButtonLink } from "@/components/atoms/ButtonLink";
import { EmptyState } from "@/components/molecules/EmptyState";

export default function NotFound() {
  return (
    <EmptyState
      title="Página no encontrada"
      description="La ruta que buscas no existe en este dashboard."
      action={
        <ButtonLink href="/payment-references" variant="secondary">
          Ir a las referencias
        </ButtonLink>
      }
    />
  );
}
