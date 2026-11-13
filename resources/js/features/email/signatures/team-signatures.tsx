/*
 * The team's signature for one address: it closes replies from the
 * address when the teammate has none of their own, and every reply the
 * agent sends from it. It lives on the address's own page, shown to
 * whoever may change everyone's signatures. Turning it on or off waits
 * for the address page's save bar.
 */
import { useQuery } from "@tanstack/react-query";

import { Section } from "@/features/settings/layout";
import type { EmailAddress } from "@/lib/api/email";
import { teamSignaturesQuery } from "@/lib/api/email-signatures";

import { SignatureCard, type SignatureToggles } from "./signature-card";

export function TeamSignature({
  workspace,
  address,
  toggles,
}: {
  workspace: string;
  address: EmailAddress;
  toggles: SignatureToggles;
}) {
  const { data: signatures } = useQuery(teamSignaturesQuery(workspace));

  if (!signatures) {
    return null;
  }
  const signature = signatures.find((s) => s.email_address_id === address.id);

  return (
    <Section
      title="Signature"
      description="Closes replies from this address when the teammate has no signature, and every reply the agent sends from it."
    >
      <SignatureCard
        workspace={workspace}
        label="Team signature"
        signature={signature}
        scope="address"
        search={{ address: address.id }}
        toggles={toggles}
      />
    </Section>
  );
}
