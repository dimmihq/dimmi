/*
 * Adding or changing a saved reply, in a modal the size of the one that
 * starts a workflow. Its title and the shortcut a reply inserts it with
 * after a #; its text, written in the same editor as a reply, with the
 * same formatting and variables picked from the toolbar; who it is for;
 * and the macro actions it applies when it is sent. Only people who
 * manage shared replies can share one. The server's reason shows when it
 * refuses.
 */
import { RiBracesLine, RiGroupLine, RiUserLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useEffect, useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { variableMarkdown, variables } from "@/features/broadcasts/writer/variables";
import { ComposerField } from "@/features/inbox/composer/composer-field";
import { preloadEditor } from "@/features/inbox/composer/load-editor";
import { failureText } from "@/features/settings/teams";
import {
  createSavedReply,
  type SavedReply,
  type SavedReplyActions,
  savedRepliesKey,
  updateSavedReply,
} from "@/lib/api/saved-replies";

import { ActionsPicker } from "./actions-picker";
import { cleanVariables, contextValues } from "./render";

const known = Object.keys(contextValues({}));
const offered = variables.filter((variable) => known.includes(variable.key));

const noop = () => undefined;

/*
 * Section is one titled part of the form.
 */
function Section({ title, detail, children }: { title: string; detail?: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-2.5">
      <div className="flex flex-col">
        <h3 className="text-body-2-medium text-text-primary">{title}</h3>
        {detail ? <p className="text-body-2-regular text-text-secondary">{detail}</p> : null}
      </div>
      {children}
    </section>
  );
}

/*
 * VariableMenu offers the customer details a reply fills in.
 */
function VariableMenu({ onPick }: { onPick: (markdown: string) => void }) {
  const [open, setOpen] = useState(false);

  return (
    <Dropdown isOpen={open} onOpenChange={setOpen}>
      <DropdownTrigger className="flex h-[30px] shrink-0 cursor-pointer items-center gap-1 rounded-full ps-2 pe-2.5 text-body-2-medium text-text-secondary outline-none transition-colors duration-150 ease hover:bg-background-secondary-hover hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring">
        <RiBracesLine aria-hidden className="size-4" />
        Variable
      </DropdownTrigger>
      <DropdownPopover aria-label="Insert a variable" placement="bottom end">
        {offered.map((variable) => (
          <DropdownItem
            key={variable.key}
            onSelect={() => {
              onPick(variableMarkdown(variable.key, variable.fallback));
              setOpen(false);
            }}
          >
            {variable.label}
          </DropdownItem>
        ))}
      </DropdownPopover>
    </Dropdown>
  );
}

export function ReplyDialog({
  workspace,
  reply,
  canShare,
  onClose,
}: {
  workspace: string;
  reply: SavedReply | null;
  canShare: boolean;
  onClose: () => void;
}) {
  const formId = useId();
  const client = useQueryClient();
  const [title, setTitle] = useState(reply?.title ?? "");
  const [shortcut, setShortcut] = useState(reply?.shortcut ?? "");
  const [body, setBody] = useState(reply?.body ?? "");
  const [shared, setShared] = useState(reply?.shared ?? false);
  const [actions, setActions] = useState<SavedReplyActions>(reply?.actions ?? { tag_ids: [] });
  const [slot, setSlot] = useState<HTMLElement | null>(null);
  useEffect(preloadEditor, []);
  const save = useMutation({
    mutationFn: () => {
      const fields = {
        title: title.trim(),
        body: cleanVariables(body),
        shortcut: shortcut.trim().replace(/^#/, "") || (reply ? null : undefined),
        shared,
        actions,
      };
      return reply ? updateSavedReply(workspace, reply.id, fields) : createSavedReply(workspace, fields);
    },
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: savedRepliesKey(workspace) });
      onClose();
    },
  });
  const ready = title.trim() !== "" && body.trim() !== "";
  const sharing = canShare || (reply?.shared ?? false);
  const submit = () => {
    if (ready && !save.isPending) {
      save.mutate();
    }
  };

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader
        title={reply ? `Edit ${reply.title}` : "New saved reply"}
        description="Insert it from a reply's add menu, or type # and its shortcut."
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          className="flex flex-col gap-6"
          onSubmit={(event) => {
            event.preventDefault();
            submit();
          }}
        >
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_160px]">
            <Input label="Title" placeholder="Refund on its way" autoFocus value={title} onChange={setTitle} />
            <Input label="Shortcut" placeholder="refund" inputDir="ltr" value={shortcut} onChange={setShortcut} />
          </div>
          <Section title="Reply" detail="Variables fill in with the customer's details when the reply is inserted.">
            <div className="flex flex-col gap-2 rounded-2xl bg-background-secondary-default p-3">
              <ComposerField
                value={body}
                label="Saved reply text"
                placeholder="Write the reply your team will send…"
                focusKey={reply?.id ?? "new"}
                onChange={setBody}
                onSubmit={submit}
                onBlur={noop}
                onFiles={noop}
                toolbarSlot={slot}
              />
              <div className="flex flex-wrap items-center gap-1 border-t border-separator-border pt-2">
                <span ref={setSlot} className="contents" />
                <span className="ms-auto">
                  <VariableMenu onPick={(markdown) => setBody((was) => `${was}${markdown}`)} />
                </span>
              </div>
            </div>
          </Section>
          {sharing ? (
            <Section title="Who can use it">
              <RadioGroup
                aria-label="Who can use it"
                value={shared ? "shared" : "personal"}
                onChange={(next) => setShared(next === "shared")}
                isDisabled={!canShare}
              >
                <RadioCard value="personal" icon={RiUserLine} title="Only me" description="In your replies only." />
                <RadioCard
                  value="shared"
                  icon={RiGroupLine}
                  title="Everyone"
                  description="The whole team can use it."
                />
              </RadioGroup>
            </Section>
          ) : null}
          <Section
            title="When it is sent"
            detail="Turn the reply into a macro: these apply to the conversation as it goes out."
          >
            <ActionsPicker workspace={workspace} value={actions} onChange={setActions} />
          </Section>
          {save.error ? (
            <p role="alert" className="text-body-2-regular text-text-error-primary">
              {failureText(save.error)}
            </p>
          ) : null}
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form={formId} loading={save.isPending} disabled={!ready}>
          {reply ? "Save" : "Create saved reply"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
