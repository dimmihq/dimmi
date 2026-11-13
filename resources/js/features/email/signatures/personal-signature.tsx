/*
 * The teammate's own signature, closing their email replies from every
 * address. Shown to whoever may keep one; an address's own signature
 * signs their replies while they have none. Turning it on or off waits
 * for the profile page's save bar, which holds this draft.
 */
import { useQuery } from "@tanstack/react-query";

import { Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { mySignaturesQuery } from "@/lib/api/email-signatures";
import { callerQuery } from "@/lib/api/workspace";

import type { PersonalSignatureDraft } from "./personal-signature-draft";
import { SignatureCard } from "./signature-card";

export function PersonalSignature({ draft }: { draft: PersonalSignatureDraft }) {
  const workspace = useAccount().workspace.id;
  const { data: caller } = useQuery(callerQuery(workspace));
  const permissions = caller?.member?.permissions ?? [];
  const allowed =
    permissions.includes("email_signatures:manage_own") || permissions.includes("email_signatures:manage_any");
  const { data: signatures } = useQuery({ ...mySignaturesQuery(workspace), enabled: allowed });

  if (!allowed || !signatures) {
    return null;
  }

  const general = signatures.find((signature) => signature.email_address_id === null);

  return (
    <Section
      title="Email signature"
      description="Closes your email replies. Without one, replies use the signature of the address they come from."
    >
      <SignatureCard
        workspace={workspace}
        label="Your signature"
        signature={general}
        scope="mine"
        toggles={draft.toggles}
      />
    </Section>
  );
}
