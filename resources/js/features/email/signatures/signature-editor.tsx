/*
 * Writing one signature: its text, a preview of how it closes a reply,
 * and whether replies use it. The preview comes from the server, so it
 * shows the placeholders filled in exactly as a reply will, and sits in
 * a sandboxed frame because it is mail, not part of this page. The frame
 * has an origin of its own, so nothing in it reaches the page, and it
 * inherits the page's content security policy, which runs no script
 * without the page's nonce; the server also strips the signature down to
 * text, emphasis and links. Saving
 * keeps a new signature or changes the one there; removing it lets the
 * next signature in line sign replies.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Textarea } from "@/components/base/textarea/textarea";
import { Failure } from "@/features/email/setup/failure";
import { Toggle } from "@/features/settings/controls";
import { Row } from "@/features/settings/layout";
import {
  createSignature,
  deleteSignature,
  type EmailSignature,
  type NewSignature,
  renderSignature,
  signaturesKey,
  updateSignature,
} from "@/lib/api/email-signatures";

const placeholderHint =
  "Bold, italic and links. Use {{member_name}}, {{member_title}}, {{address_name}} or {{company_name}}.";

/*
 * usePreview renders the text as a reply shows it, a moment after typing
 * stops changing it.
 */
function usePreview(workspace: string, body: string, address?: string) {
  const settled = useDeferredValue(body.trim());

  return useQuery({
    queryKey: [...signaturesKey(workspace), "rendering", settled, address ?? ""],
    queryFn: () => renderSignature(workspace, settled, address),
    enabled: settled !== "",
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function SignatureEditor({
  workspace,
  label,
  signature,
  scope,
}: {
  workspace: string;
  label: string;
  signature?: EmailSignature;
  scope: Omit<NewSignature, "body">;
}) {
  const queryClient = useQueryClient();
  const [body, setBody] = useState(signature?.body ?? "");
  const preview = usePreview(workspace, body, scope.email_address_id);
  const refresh = () => queryClient.invalidateQueries({ queryKey: signaturesKey(workspace) });

  const save = useMutation({
    mutationFn: () =>
      signature ? updateSignature(workspace, signature.id, { body }) : createSignature(workspace, { ...scope, body }),
    onSuccess: refresh,
  });
  const toggle = useMutation({
    mutationFn: (enabled: boolean) => updateSignature(workspace, signature?.id ?? "", { enabled }),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: () => deleteSignature(workspace, signature?.id ?? ""),
    onSuccess: async () => {
      setBody("");
      await refresh();
    },
  });

  const html = body.trim() !== "" ? preview.data?.html : undefined;

  return (
    <>
      <div className="flex flex-col gap-3 py-3 pe-3">
        <Textarea label={label} hint={placeholderHint} rows={4} maxLength={2000} value={body} onChange={setBody} />
        {html ? (
          <iframe
            title={`${label} preview`}
            sandbox="allow-scripts"
            srcDoc={html}
            className="h-28 w-full rounded-xl border border-border-button-default bg-background-primary-default"
          />
        ) : null}
        <Failure error={preview.error ?? save.error ?? toggle.error ?? remove.error} />
        <div className="flex gap-2">
          <Button size="small" disabled={save.isPending || body.trim() === ""} onClick={() => save.mutate()}>
            Save signature
          </Button>
          {signature ? (
            <Button variant="secondary" size="small" disabled={remove.isPending} onClick={() => remove.mutate()}>
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
    </>
  );
}
