/*
 * Every teammate's own signatures, for whoever may change everyone's:
 * an admin can fix a teammate's signature, turn it off or remove it, and
 * the teammate's next reply carries the change. A teammate is shown by
 * name when the members list is readable, else plainly as a teammate.
 */
import { useQuery } from "@tanstack/react-query";

import { Section } from "@/features/settings/layout";
import type { EmailAddress } from "@/lib/api/email";
import { personalSignaturesQuery } from "@/lib/api/email-signatures";
import { membersQuery } from "@/lib/api/workspace";

import { SignatureEditor } from "./signature-editor";

export function TeammateSignatures({
  workspace,
  email,
  addresses,
}: {
  workspace: string;
  email: string;
  addresses: EmailAddress[];
}) {
  const { data: signatures } = useQuery(personalSignaturesQuery(workspace));
  const { data: members } = useQuery({ ...membersQuery(workspace), retry: false });

  const others = (signatures ?? []).filter((signature) => {
    const member = members?.find((m) => m.id === signature.member_id);
    return member?.user.email !== email;
  });

  if (others.length === 0) {
    return null;
  }

  return (
    <Section title="Teammates' signatures" description="What each teammate's replies close with.">
      {others.map((signature) => {
        const name = members?.find((m) => m.id === signature.member_id)?.user.name ?? "a teammate";
        const address = addresses.find((a) => a.id === signature.email_address_id)?.address;

        return (
          <SignatureEditor
            key={signature.id}
            workspace={workspace}
            label={address ? `Signature of ${name} for ${address}` : `Signature of ${name}`}
            signature={signature}
            scope={{ kind: "personal" }}
          />
        );
      })}
    </Section>
  );
}
