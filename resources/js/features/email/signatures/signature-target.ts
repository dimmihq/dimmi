/*
 * Which signature the signature page writes, and where it was opened
 * from: the teammate's own on their profile, the team's for one address
 * on that address's page, or a teammate's on the email settings page.
 * Leaving the page returns to that place, scrolled to the signature.
 */
import { useQuery } from "@tanstack/react-query";

import { emailAddressesQuery } from "@/lib/api/email";
import {
  type EmailSignature,
  mySignaturesQuery,
  type NewSignature,
  personalSignaturesQuery,
  teamSignaturesQuery,
} from "@/lib/api/email-signatures";
import { membersQuery } from "@/lib/api/workspace";

export type SignatureScope = "mine" | "address" | "teammate";

export const signatureScopes: SignatureScope[] = ["mine", "address", "teammate"];

/*
 * SignatureSearch names the address of a team signature, or the
 * teammate's signature being changed.
 */
export interface SignatureSearch {
  address?: string;
  signature?: string;
}

/*
 * SignatureTarget is one signature to write: its name, the signature
 * when there is one, what a new one is kept as, and the address its
 * preview signs for. found is false once the lists load without it.
 */
export interface SignatureTarget {
  loading: boolean;
  found: boolean;
  title: string;
  signature?: EmailSignature;
  scope: Omit<NewSignature, "body">;
  address?: string;
}

/*
 * anchorOf is the place on the page the signature was opened from.
 */
export function anchorOf(scope: SignatureScope, signature?: string): string {
  return scope === "teammate" && signature ? `signature-${signature}` : "signature";
}

function useMine(workspace: string, enabled: boolean): SignatureTarget {
  const { data, isPending } = useQuery({ ...mySignaturesQuery(workspace), enabled });

  return {
    loading: isPending,
    found: true,
    title: "Your signature",
    signature: data?.find((signature) => signature.email_address_id === null),
    scope: { kind: "personal" },
  };
}

function useAddress(workspace: string, enabled: boolean, address = ""): SignatureTarget {
  const signatures = useQuery({ ...teamSignaturesQuery(workspace), enabled });
  const addresses = useQuery({ ...emailAddressesQuery(workspace), enabled });
  const found = addresses.data?.find((a) => a.id === address);

  return {
    loading: signatures.isPending || addresses.isPending,
    found: found !== undefined,
    title: found ? `Signature for ${found.address}` : "Team signature",
    signature: signatures.data?.find((signature) => signature.email_address_id === address),
    scope: { kind: "address", email_address_id: address },
    address,
  };
}

function useTeammate(workspace: string, enabled: boolean, id = ""): SignatureTarget {
  const signatures = useQuery({ ...personalSignaturesQuery(workspace), enabled });
  const members = useQuery({ ...membersQuery(workspace), enabled, retry: false });
  const signature = signatures.data?.find((s) => s.id === id);
  const name = members.data?.find((m) => m.id === signature?.member_id)?.user.name ?? "a teammate";

  return {
    loading: signatures.isPending,
    found: signature !== undefined,
    title: `Signature of ${name}`,
    signature,
    scope: { kind: "personal" },
    address: signature?.email_address_id ?? undefined,
  };
}

/*
 * useSignatureTarget reads the signature the page was opened for.
 */
export function useSignatureTarget(workspace: string, scope: SignatureScope, search: SignatureSearch): SignatureTarget {
  const mine = useMine(workspace, scope === "mine");
  const address = useAddress(workspace, scope === "address", search.address);
  const teammate = useTeammate(workspace, scope === "teammate", search.signature);

  return { mine, address, teammate }[scope];
}
