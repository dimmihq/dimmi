import { createFileRoute, notFound } from "@tanstack/react-router";

import { SignaturePage } from "@/features/email/signatures/signature-page";
import {
  type SignatureScope,
  type SignatureSearch,
  signatureScopes,
} from "@/features/email/signatures/signature-target";

export const Route = createFileRoute("/d/settings/signatures/$scope")({
  beforeLoad: ({ params }) => {
    if (!signatureScopes.includes(params.scope as SignatureScope)) {
      throw notFound();
    }
  },
  validateSearch: (search: Record<string, unknown>): SignatureSearch => ({
    ...(typeof search.address === "string" ? { address: search.address } : {}),
    ...(typeof search.signature === "string" ? { signature: search.signature } : {}),
  }),
  component: SignatureRoute,
});

function SignatureRoute() {
  const { scope } = Route.useParams();
  const search = Route.useSearch();

  return (
    <SignaturePage
      key={`${scope}:${search.address ?? ""}:${search.signature ?? ""}`}
      scope={scope as SignatureScope}
      search={search}
    />
  );
}
