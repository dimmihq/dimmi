/*
 * Handing the server a raw email the way a receiving provider would,
 * through the test inbound source that server.ts opens with this same
 * secret.
 */
import { type APIRequestContext, expect } from "@playwright/test";

const inboundSecret = "e2e-inbound-secret";

/*
 * rawEmail writes a plain text email from the customer to the address.
 */
export function rawEmail(from: string, to: string, subject: string, body: string): string {
  const id = `${Date.now()}.${Math.random().toString(36).slice(2)}@customer.test`;

  return [
    `From: Bob Stone <${from}>`,
    `To: ${to}`,
    `Subject: ${subject}`,
    "Date: Tue, 3 Nov 2026 10:00:00 +0000",
    `Message-ID: <${id}>`,
    "MIME-Version: 1.0",
    "Content-Type: text/plain; charset=utf-8",
    "",
    body,
    "",
  ].join("\r\n");
}

/*
 * deliverEmail posts the raw email to the forwarding address.
 */
export async function deliverEmail(request: APIRequestContext, raw: string, to: string) {
  const response = await request.post(`/webhooks/test/inbound?to=${encodeURIComponent(to)}`, {
    headers: { Authorization: `Bearer ${inboundSecret}`, "Content-Type": "message/rfc822" },
    data: raw,
  });
  expect(response.status(), await response.text()).toBe(202);
}
