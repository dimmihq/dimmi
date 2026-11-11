/*
 * One signature where it is managed: a small preview of how it reads,
 * a way to its own page to write it, whether replies use it, and
 * removing it, which lets the next signature in line sign replies. The
 * card carries the anchor the signature page returns to.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";

import { Button, buttonStyles } from "@/components/base/buttons/button";
import { Failure } from "@/features/email/setup/failure";
import { Toggle } from "@/features/settings/controls";
import { Row } from "@/features/settings/layout";
import { deleteSignature, type EmailSignature, signaturesKey, updateSignature } from "@/lib/api/email-signatures";
import { cx } from "@/utils/cx";

import { SignatureFrame } from "./signature-frame";
import { useSignaturePreview } from "./signature-preview";
import { anchorOf, type SignatureScope, type SignatureSearch } from "./signature-target";

/*
 * lowerFirst writes a label for the middle of a sentence.
 */
function lowerFirst(label: string): string {
  return label.charAt(0).toLowerCase() + label.slice(1);
}

export function SignatureCard({
  workspace,
  label,
  signature,
  scope,
  search = {},
}: {
  workspace: string;
  label: string;
  signature?: EmailSignature;
  scope: SignatureScope;
  search?: SignatureSearch;
}) {
  const queryClient = useQueryClient();
  const preview = useSignaturePreview(
    workspace,
    signature?.body ?? "",
    signature?.email_address_id ?? search.address ?? undefined,
  );
  const refresh = () => queryClient.invalidateQueries({ queryKey: signaturesKey(workspace) });
  const toggle = useMutation({
    mutationFn: (enabled: boolean) => updateSignature(workspace, signature?.id ?? "", { enabled }),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: () => deleteSignature(workspace, signature?.id ?? ""),
    onSuccess: refresh,
  });
  const html = signature ? preview.data?.html : undefined;

  return (
    <div id={anchorOf(scope, search.signature)} className="flex scroll-mt-6 flex-col">
      <div className="flex flex-col gap-3 py-3 pe-3">
        {html ? (
          <SignatureFrame title={`${label} preview`} html={html} className="h-24" />
        ) : (
          <p className="text-body-2-regular text-text-tertiary">
            {signature ? "This signature shows nothing in a reply yet." : "No signature yet."}
          </p>
        )}
        <Failure error={toggle.error ?? remove.error} />
        <div className="flex gap-2">
          <Link
            to="/d/settings/signatures/$scope"
            params={{ scope }}
            search={search}
            aria-label={`${signature ? "Edit" : "Add"} ${lowerFirst(label)}`}
            className={cx(buttonStyles.base, buttonStyles.size.small, buttonStyles.variant.secondary)}
          >
            <span className={buttonStyles.label.small}>{signature ? "Edit signature" : "Add signature"}</span>
          </Link>
          {signature ? (
            <Button variant="secondary" size="small" loading={remove.isPending} onClick={() => remove.mutate()}>
              Remove
            </Button>
          ) : null}
        </div>
      </div>
      {signature ? (
        <Row label="Use in replies" description="Turned off, replies use the next signature in line.">
          <Toggle label={`Use ${label.toLowerCase()}`} on={signature.enabled} onChange={(on) => toggle.mutate(on)} />
        </Row>
      ) : null}
    </div>
  );
}
