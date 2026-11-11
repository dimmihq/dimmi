/*
 * Writing one signature on a page of its own: the writer beside the
 * reply it closes, or above it on a narrow screen, so every change shows
 * at once as the customer will read it. Saving keeps the signature and
 * returns to the place the page was opened from; leaving with changes
 * unsaved asks first, so a signature is never lost by a stray click.
 */
import { RiArrowLeftSLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Failure } from "@/features/email/setup/failure";
import { useAccount } from "@/features/settings/use-account";
import { Panel, ScrollArea } from "@/features/shell/page";
import { createSignature, signaturesKey, updateSignature } from "@/lib/api/email-signatures";

import { SignatureFrame } from "./signature-frame";
import { useSignaturePreview } from "./signature-preview";
import {
  anchorOf,
  type SignatureScope,
  type SignatureSearch,
  type SignatureTarget,
  useSignatureTarget,
} from "./signature-target";
import { SignatureWriter } from "./signature-writer";

/*
 * useLeave returns to the page the signature was opened from, at the
 * signature.
 */
function useLeave(scope: SignatureScope, search: SignatureSearch) {
  const navigate = useNavigate();
  const hash = anchorOf(scope, search.signature);

  return () => {
    if (scope === "address" && search.address) {
      return navigate({ to: "/d/settings/email/addresses/$address", params: { address: search.address }, hash });
    }

    return navigate({ to: "/d/settings/$section", params: { section: scope === "mine" ? "profile" : "email" }, hash });
  };
}

/*
 * useUnsavedGuard asks before leaving with changes unsaved, unless the
 * page is leaving because they were just saved.
 */
function useUnsavedGuard(dirty: boolean) {
  const saved = useRef(false);
  const blocker = useBlocker({
    shouldBlockFn: () => dirty && !saved.current,
    enableBeforeUnload: () => dirty && !saved.current,
    withResolver: true,
  });

  return { blocker, markSaved: () => (saved.current = true) };
}

function Writer({
  workspace,
  target,
  scope,
  search,
}: {
  workspace: string;
  target: SignatureTarget;
  scope: SignatureScope;
  search: SignatureSearch;
}) {
  const queryClient = useQueryClient();
  const initial = target.signature?.body ?? "";
  const [body, setBody] = useState(initial);
  const dirty = body.trim() !== initial.trim();
  const preview = useSignaturePreview(workspace, body, target.address);
  const leave = useLeave(scope, search);
  const guard = useUnsavedGuard(dirty);
  const save = useMutation({
    mutationFn: () =>
      target.signature
        ? updateSignature(workspace, target.signature.id, { body })
        : createSignature(workspace, { ...target.scope, body }),
    onSuccess: async () => {
      guard.markSaved();
      await queryClient.invalidateQueries({ queryKey: signaturesKey(workspace) });
      toast.success("Signature saved.");
      await leave();
    },
  });
  const html = body.trim() !== "" ? (preview.data?.html ?? "") : "";

  return (
    <Panel label={target.title} ground className="h-full w-full flex-1">
      <header className="flex min-h-12 shrink-0 items-center gap-2 ps-2.5 pe-2.5 pt-[7px]">
        <button
          type="button"
          aria-label="Back"
          onClick={() => void leave()}
          className="flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
        </button>
        <h1 className="min-w-0 flex-1 truncate text-body-medium text-text-primary">{target.title}</h1>
        <Button
          size="small"
          loading={save.isPending}
          disabled={!dirty || body.trim() === ""}
          onClick={() => save.mutate()}
        >
          Save signature
        </Button>
      </header>
      <ScrollArea ground>
        <div className="mx-auto grid w-full max-w-6xl gap-8 px-6 pt-8 pb-16 lg:grid-cols-2 lg:pt-12">
          <div className="order-2 flex min-w-0 flex-col gap-2 lg:order-1">
            <h2 className="text-body-2-medium text-text-secondary">Signature</h2>
            <SignatureWriter value={initial} label={target.title} onChange={setBody} />
            <p className="text-body-2-regular text-text-tertiary">
              Bold, italic and links. Type / or pick Detail to add your name, job title, the address or the company.
            </p>
            <Failure error={preview.error ?? save.error} />
          </div>
          <div className="order-1 flex min-w-0 flex-col gap-2 lg:order-2">
            <h2 className="text-body-2-medium text-text-secondary">In a reply</h2>
            <SignatureFrame title={`${target.title} preview`} html={html} reply className="h-96" />
          </div>
        </div>
      </ScrollArea>
      <AlertDialog
        isOpen={guard.blocker.status === "blocked"}
        onOpenChange={(open) => (open ? undefined : guard.blocker.reset?.())}
        variant="default"
        title="Leave without saving?"
        description="The changes to this signature are lost."
        confirmLabel="Leave"
        cancelLabel="Keep writing"
        onConfirm={() => guard.blocker.proceed?.()}
      />
    </Panel>
  );
}

function Notice({ children }: { children: string }) {
  return (
    <Panel label="Signature" ground className="h-full flex-1">
      <div className="flex h-full items-center justify-center text-body-2-regular text-text-tertiary">{children}</div>
    </Panel>
  );
}

export function SignaturePage({ scope, search }: { scope: SignatureScope; search: SignatureSearch }) {
  const workspace = useAccount().workspace.id;
  const target = useSignatureTarget(workspace, scope, search);

  if (target.loading) {
    return <Notice>Loading…</Notice>;
  }
  if (!target.found) {
    return <Notice>This signature does not exist or you cannot change it.</Notice>;
  }

  return <Writer workspace={workspace} target={target} scope={scope} search={search} />;
}
