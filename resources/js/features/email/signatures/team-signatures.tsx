/*
 * The team's signature for each address: it closes replies from the
 * address when the teammate has none of their own, and every reply the
 * agent sends from it. Shown to whoever may change everyone's
 * signatures.
 */
import { useQuery } from "@tanstack/react-query";

import { Section } from "@/features/settings/layout";
import type { EmailAddress } from "@/lib/api/email";
import { teamSignaturesQuery } from "@/lib/api/email-signatures";

import { SignatureEditor } from "./signature-editor";

export function TeamSignatures({ workspace, addresses }: { workspace: string; addresses: EmailAddress[] }) {
  const { data: signatures } = useQuery(teamSignaturesQuery(workspace));

  if (!signatures || addresses.length === 0) {
    return null;
  }

  return (
    <Section
      title="Team signatures"
      description="Close replies from an address when the teammate has no signature, and every reply the agent sends."
    >
      {addresses.map((address) => {
        const signature = signatures.find((s) => s.email_address_id === address.id);

        return (
          <SignatureEditor
            key={`${address.id}-${signature?.id ?? "new"}`}
            workspace={workspace}
            label={`Signature for ${address.address}`}
            signature={signature}
            scope={{ kind: "address", email_address_id: address.id }}
          />
        );
      })}
    </Section>
  );
}
