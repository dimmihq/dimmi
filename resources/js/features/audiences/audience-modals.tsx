/*
 * What can be done to an audience, each in a modal opened from its row:
 * editing its rule the way the people list is filtered, with how many
 * people it matches as it changes; renaming it; testing one person
 * against it; freezing who it matches now into a tag; and deleting it,
 * which an audience something live uses refuses, naming what to change.
 * A new audience is named and given its rule in one modal, and only made
 * on Save. Members who may only read see the rule without controls.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Textarea } from "@/components/base/textarea/textarea";
import { usePermission } from "@/features/settings/inboxes";
import {
  type Audience,
  type AudiencePreview,
  audienceFieldsQuery,
  audiencesQuery,
  deleteAudience,
  freezeAudience,
} from "@/lib/api/audiences";
import { formatNumber } from "@/lib/i18n/format";

import { RuleGroups } from "./rule-groups";
import { summary } from "./rules";
import { listSummary } from "./tagging";
import { TestPerson } from "./test-person";
import { useAudienceDraft } from "./use-audience-draft";
import { useLookups } from "./use-lookups";

/*
 * modalHeight gives every audience modal the new workflow modal's
 * height, a little wider so a condition fits on one row, so moving
 * between them never jumps.
 */
const modalHeight = "h-[min(640px,calc(100dvh-3rem))]";

function matchCount(preview: AudiencePreview | undefined, previewing: boolean): string {
  if (!preview) {
    return previewing ? "Counting…" : "";
  }
  const people = preview.count === 1 ? "1 person" : `${formatNumber(preview.count)} people`;

  return `Matches ${preview.approximate ? "about " : ""}${people}`;
}

type Draft = ReturnType<typeof useAudienceDraft>;

function Details({ draft, focus }: { draft: Draft; focus: boolean }) {
  return (
    <div className="flex flex-col gap-3">
      <Input label="Name" value={draft.name} onChange={draft.setName} autoFocus={focus} />
      <Textarea
        label="What it is for"
        placeholder="Paying customers on the Pro plan"
        value={draft.description}
        onChange={draft.setDescription}
      />
    </div>
  );
}

/*
 * Rule is the modal's rule: the filters, or a note when the rule nests
 * deeper than they show.
 */
function Rule({ workspace, draft, own }: { workspace: string; draft: Draft; own: string }) {
  const fields = useQuery(audienceFieldsQuery(workspace)).data ?? [];
  const { lookups } = useLookups(workspace, own);
  if (!draft.form) {
    return (
      <p className="text-body-2-regular text-text-secondary">
        This rule was written through the API and nests deeper than the filters show.
      </p>
    );
  }

  return <RuleGroups form={draft.form} fields={fields} lookups={lookups} onChange={draft.setForm} />;
}

function Footer({
  draft,
  count,
  canManage,
  create,
  onClose,
}: {
  draft: Draft;
  count: string;
  canManage: boolean;
  create: boolean;
  onClose: () => void;
}) {
  const ready = (create || draft.changed) && draft.form !== null && draft.name.trim() !== "";

  return (
    <ModalFooter className="sm:justify-between">
      <span className="text-body-2-regular text-text-secondary">{count}</span>
      <span className="flex flex-col-reverse gap-2 sm:flex-row">
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        {canManage ? (
          <Button
            loading={draft.save.isPending}
            disabled={!ready}
            onClick={() => draft.save.mutate(undefined, { onSuccess: onClose })}
          >
            {create ? "Create audience" : "Save"}
          </Button>
        ) : null}
      </span>
    </ModalFooter>
  );
}

/*
 * AudienceModal edits an audience: its name for a new one or a rename,
 * its rule for a new one or a change of rule.
 */
export function AudienceModal({
  workspace,
  audience,
  mode,
  onClose,
}: {
  workspace: string;
  audience: Audience;
  mode: "new" | "rules" | "details";
  onClose: () => void;
}) {
  const canManage = usePermission(workspace, "audiences:manage");
  const draft = useAudienceDraft(workspace, audience, onClose);
  const withRules = mode !== "details";
  const titles = { new: "New audience", rules: `${draft.server.name}: rule`, details: `Rename ${draft.server.name}` };
  const problems = draft.problems.rest;

  return (
    <Modal size="wide" className={modalHeight} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title={titles[mode]} />
      <ModalBody className="pb-2">
        <fieldset disabled={!canManage} className="min-w-0">
          <div className="flex flex-col gap-6">
            {mode === "rules" ? null : <Details draft={draft} focus />}
            {withRules ? <Rule workspace={workspace} draft={draft} own={audience.id} /> : null}
          </div>
        </fieldset>
        {problems.length > 0 ? (
          <p className="pt-3 text-body-2-regular text-text-error-primary">{problems.join(" ")}</p>
        ) : null}
      </ModalBody>
      <Footer
        draft={draft}
        count={withRules ? matchCount(draft.preview, draft.previewing) : ""}
        canManage={canManage}
        create={mode === "new"}
        onClose={onClose}
      />
    </Modal>
  );
}

/*
 * TestModal checks one person against the audience's rule.
 */
export function TestModal({
  workspace,
  audience,
  onClose,
}: {
  workspace: string;
  audience: Audience;
  onClose: () => void;
}) {
  const { names } = useLookups(workspace, audience.id);

  return (
    <Modal size="wide" className={modalHeight} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title={`Test ${audience.name}`} description={summary(audience.rules, names)} />
      <ModalBody className="pb-4">
        <TestPerson workspace={workspace} rules={audience.rules} names={names} />
      </ModalBody>
    </Modal>
  );
}

/*
 * FreezeModal tags everyone the audience matches now, so the group stays
 * as it is whoever joins or leaves the audience later.
 */
export function FreezeModal({
  workspace,
  audience,
  onClose,
}: {
  workspace: string;
  audience: Audience;
  onClose: () => void;
}) {
  const formId = useId();
  const [tag, setTag] = useState("");
  const freeze = useMutation({ mutationFn: () => freezeAudience(workspace, audience.id, tag.trim()) });

  return (
    <Modal size="wide" className={modalHeight} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={`Freeze ${audience.name}`}
        description="Tag everyone who matches now. The tag stays as it is when the audience changes."
      />
      <ModalBody>
        <form
          id={formId}
          className="flex flex-col gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            if (tag.trim() !== "") {
              freeze.mutate();
            }
          }}
        >
          <Input label="Tag" placeholder="Pro on 1 December" autoFocus value={tag} onChange={setTag} maxLength={50} />
          {freeze.data ? <p className="text-body-2-regular text-text-primary">{listSummary(freeze.data)}</p> : null}
          {freeze.error ? <p className="text-body-2-regular text-text-error-primary">{freeze.error.message}</p> : null}
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          {freeze.data ? "Done" : "Cancel"}
        </Button>
        {freeze.data ? null : (
          <Button type="submit" form={formId} loading={freeze.isPending} disabled={tag.trim() === ""}>
            Freeze
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

/*
 * DeleteModal deletes the audience, or says what still uses it.
 */
export function DeleteModal({
  workspace,
  audience,
  onClose,
}: {
  workspace: string;
  audience: Audience;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const remove = useMutation({
    mutationFn: () => deleteAudience(workspace, audience.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: audiencesQuery(workspace).queryKey });
      onClose();
    },
  });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Delete ${audience.name}?`}
      confirmLabel="Delete audience"
      onConfirm={() => remove.mutateAsync()}
    >
      <p className="text-body-2-regular text-text-secondary">
        {remove.error
          ? remove.error.message
          : "Workflow drafts that name it will need another audience before they publish."}
      </p>
    </AlertDialog>
  );
}
