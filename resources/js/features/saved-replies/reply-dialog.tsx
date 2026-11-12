/*
 * Adding or changing a saved reply in a modal: its title, the shortcut
 * found by typing # in the composer, its Markdown text with variables
 * picked from a list, who it is for, and the macro actions it applies
 * when it is sent. Only people who manage shared replies can share one.
 * The server's reason shows when it refuses.
 */
import { RiGroupLine, RiUserLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { variableMarkdown, variables } from "@/features/broadcasts/writer/variables";
import { failureText } from "@/features/settings/teams";
import {
  createSavedReply,
  type SavedReply,
  type SavedReplyActions,
  savedRepliesKey,
  updateSavedReply,
} from "@/lib/api/saved-replies";

import { ActionsPicker } from "./actions-picker";
import { contextValues } from "./render";

const known = Object.keys(contextValues({}));
const offered = variables.filter((variable) => known.includes(variable.key));

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
  const save = useMutation({
    mutationFn: () => {
      const fields = {
        title: title.trim(),
        body,
        shortcut: shortcut.trim() || (reply ? null : undefined),
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

  return (
    <Modal
      size="lg"
      className="h-[min(760px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader
        title={reply ? `Edit ${reply.title}` : "New saved reply"}
        description="Insert it from the composer's add menu, or type # and its shortcut."
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          className="flex flex-col gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (ready) {
              save.mutate();
            }
          }}
        >
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-[1fr_180px]">
            <Input label="Title" placeholder="Refund on its way" autoFocus value={title} onChange={setTitle} />
            <Input label="Shortcut" placeholder="refund" inputDir="ltr" value={shortcut} onChange={setShortcut} />
          </div>
          <div className="flex flex-col gap-1.5">
            <Textarea
              label="Text"
              hint="Markdown. Variables fill in with the customer's details when the reply is inserted."
              rows={6}
              value={body}
              onChange={setBody}
            />
            <Select
              aria-label="Insert a variable"
              placeholder="Insert a variable"
              size="sm"
              className="self-start"
              selectedKey={null}
              onSelectionChange={(key) => {
                const variable = offered.find((candidate) => candidate.key === key);
                if (variable) {
                  setBody(`${body}${variableMarkdown(variable.key, variable.fallback)}`);
                }
              }}
            >
              {offered.map((variable) => (
                <SelectItem key={variable.key} id={variable.key} textValue={variable.label}>
                  {variable.label}
                </SelectItem>
              ))}
            </Select>
          </div>
          {sharing ? (
            <div className="flex flex-col gap-1">
              <Label>Who can use it</Label>
              <RadioGroup
                aria-label="Who can use it"
                value={shared ? "shared" : "personal"}
                onChange={(next) => setShared(next === "shared")}
                isDisabled={!canShare}
              >
                <RadioCard value="personal" icon={RiUserLine} title="Only me" description="In your composer only." />
                <RadioCard
                  value="shared"
                  icon={RiGroupLine}
                  title="Everyone"
                  description="The whole team can use it."
                />
              </RadioGroup>
            </div>
          ) : null}
          <div className="flex flex-col gap-1">
            <span className="text-headline-medium text-text-primary">When it is sent</span>
            <p className="text-body-2-regular text-text-tertiary">
              Turn the reply into a macro: these apply to the conversation as the reply goes out.
            </p>
          </div>
          <ActionsPicker workspace={workspace} value={actions} onChange={setActions} />
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
