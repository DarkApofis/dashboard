import { redirect } from "next/navigation";

/** The dashboard has one screen, so the root goes straight to it. */
export default function HomePage() {
  redirect("/payment-references");
}
