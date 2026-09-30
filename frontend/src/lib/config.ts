/**
 * Runtime configuration, read once so no component reaches into process.env.
 */

/**
 * Where the Go API lives.
 *
 * API_URL wins and is read at runtime, which is what lets one built image talk to
 * http://api:8080 inside a compose network and to a real host in a deployment.
 * NEXT_PUBLIC_API_URL is the fallback for local development, where it is also the
 * value a client component would see if one ever needed it.
 *
 * Every read happens on the server, so the URL does not have to reach the browser.
 */
export const API_URL = (process.env.API_URL ?? process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080").replace(
  /\/+$/,
  "",
);

/**
 * Base of the shareable payment link shown on the detail panel. The platform does not
 * host a payment page, so the link is a placeholder built from the external
 * reference: it comes from the design, not from the domain.
 */
export const PAYMENT_LINK_BASE = (process.env.NEXT_PUBLIC_PAYMENT_LINK_BASE ?? "https://pay.example.com/r").replace(/\/+$/, "");

/**
 * Shows the control that posts a fake provider notification to /webhooks/payments.
 * It is a review convenience and only succeeds while the backend runs with
 * WEBHOOK_SECRET empty, which the UI says out loud.
 */
export const WEBHOOK_SIMULATOR_ENABLED = (process.env.NEXT_PUBLIC_ENABLE_WEBHOOK_SIMULATOR ?? "true") !== "false";

/** How many rows a page of the listing holds. The API caps it at 100. */
export const PAGE_SIZE = 20;
