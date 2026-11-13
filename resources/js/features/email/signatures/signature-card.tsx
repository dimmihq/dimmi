/*
 * One signature where it is managed: a small preview of how it reads,
 * a way to its own page to write it, whether replies use it, and
 * removing it, which lets the next signature in line sign replies. The
 * card carries the anchor the signature page returns to. Whether replies
 * use it is held in the page's draft and sent with the page's save bar;
 * removing is an action of its own and happens at once.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

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

/*
 * useSignatureToggles holds the signatures turned on or off but not saved
 * yet, by id. send writes each change; clear forgets them all, after a
 * save or on Discard. A removed signature's change is forgotten with it.
 */
export function useSignatureToggles(workspace: string) {
  const [pending, setPending] = useState<Record<string, boolean>>({});

  return {
    dirty: Object.keys(pending).length > 0,
    enabledOf: (signature: EmailSignature) => pending[signature.id] ?? signature.enabled,
    set: (signature: EmailSignature, on: boolean) =>
      setPending(({ [signature.id]: _, ...rest }) =>
        on === signature.enabled ? rest : { ...rest, [signature.id]: on },
      ),
    forget: (id: string) => setPending(({ [id]: _, ...rest }) => rest),
    send: () =>
      Promise.all(Object.entries(pending).map(([id, enabled]) => updateSignature(workspace, id, { enabled }))),
    clear: () => setPending({}),
  };
}

export type SignatureToggles = ReturnType<typeof useSignatureToggles>;

export function SignatureCard({
  workspace,
  label,
  signature,
  scope,
  search = {},
  toggles,
}: {
  workspace: string;
  label: string;
  signature?: EmailSignature;
  scope: SignatureScope;
  search?: SignatureSearch;
  toggles: SignatureToggles;
}) {
  const queryClient = useQueryClient();
  const preview = useSignaturePreview(
    workspace,
    signature?.body ?? "",
    signature?.email_address_id ?? search.address ?? undefined,
  );
  const refresh = () => queryClient.invalidateQueries({ queryKey: signaturesKey(workspace) });
  const remove = useMutation({
    mutationFn: () => deleteSignature(workspace, signature?.id ?? ""),
    onSuccess: () => {
      if (signature) {
        toggles.forget(signature.id);
      }
      return refresh();
    },
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
        <Failure error={remove.error} />
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
          <Toggle
            key={`enabled-${toggles.enabledOf(signature)}`}
            label={`Use ${label.toLowerCase()}`}
            on={toggles.enabledOf(signature)}
            onChange={(on) => toggles.set(signature, on)}
          />
        </Row>
      ) : null}
    </div>
  );
}
