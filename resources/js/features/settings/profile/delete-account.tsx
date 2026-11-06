/*
 * Deleting the account, at once and for good. The dialog first says what
 * stands in the way and what goes with it, then asks for the address to be
 * typed, and after a while away the password too.
 */
import { RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { fieldError } from "@/features/auth/form-errors";
import { type DeletionWorkspace, deleteAccount, deletionPreviewQuery } from "@/lib/api/account";

import { Danger } from "../controls";
import { Row, Section } from "../layout";
import { useRecentAuth } from "../recent-auth";

function Workspaces({ title, workspaces }: { title: string; workspaces: DeletionWorkspace[] }) {
  if (workspaces.length === 0) {
    return null;
  }

  return (
    <div className="flex flex-col gap-1">
      <p className="text-body-2-regular text-text-secondary">{title}</p>
      <ul className="flex list-disc flex-col gap-1 ps-5 text-body-2-medium text-text-primary">
        {workspaces.map((workspace) => (
          <li key={workspace.id}>{workspace.name}</li>
        ))}
      </ul>
    </div>
  );
}

export function DeleteAccount({ email, hasPassword }: { email: string; hasPassword: boolean }) {
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const { run, prompt } = useRecentAuth(hasPassword);
  const preview = useQuery({ ...deletionPreviewQuery, enabled: open, staleTime: 0 });
  const remove = useMutation({
    mutationFn: () => run(() => deleteAccount(typed)),
    onSuccess: (done) => {
      if (done) {
        window.location.assign("/login");
      }
    },
  });
  const failure = fieldError(remove.error, "confirm_email", ["confirm_email"], "confirm_email");
  const blocking = preview.data?.blocking ?? [];
  const blocked = blocking.length > 0;
  const matches = typed.trim().toLowerCase() === email.toLowerCase();

  const close = () => {
    setTyped("");
    remove.reset();
    setOpen(false);
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    remove.mutate();
  };

  return (
    <Section title="Delete account" description="Removes you from every workspace, straight away and for good.">
      <Row label="Delete your account" description="Workspaces you alone own must be handed over or deleted first.">
        <Danger onPress={() => setOpen(true)}>Delete account</Danger>
      </Row>
      <Modal size="sm" isOpen={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
        <form onSubmit={submit}>
          <ModalHeader
            icon={RiDeleteBinLine}
            tone="danger"
            title="Delete your account?"
            description="This cannot be undone. Your messages stay, shown as a former teammate."
          />
          <ModalBody className="flex flex-col gap-4">
            <Workspaces title="Hand over or delete these workspaces first:" workspaces={blocking} />
            <Workspaces
              title="Nobody else works in these, so they are deleted too:"
              workspaces={preview.data?.will_be_deleted ?? []}
            />
            {blocked ? null : (
              <Input
                label="Your email to confirm"
                autoComplete="off"
                autoFocus
                inputDir="ltr"
                value={typed}
                onChange={(next) => {
                  setTyped(next);
                  remove.reset();
                }}
                isInvalid={Boolean(failure)}
                errorMessage={failure}
              />
            )}
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" variant="danger" loading={remove.isPending} disabled={blocked || !matches}>
              Delete account
            </Button>
          </ModalFooter>
        </form>
      </Modal>
      {prompt}
    </Section>
  );
}
