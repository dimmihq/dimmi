/*
 * Deleting the workspace, for its owners only. The workspace goes to the
 * trash, where it can be restored for 30 days, but everyone in it loses
 * access at once, so the owner types the workspace's address before the
 * button works and, after a while away, their password too.
 */
import { RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { fieldError } from "@/features/auth/form-errors";
import { callerQuery, deleteWorkspace } from "@/lib/api/workspace";

import { Danger } from "./controls";
import { Row, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";
import { useAccount } from "./use-account";

/*
 * DeleteWorkspace is the danger zone of the workspace page: the delete
 * button and the dialog it opens. Anyone but an owner sees nothing.
 */
export function DeleteWorkspace({ id, name, slug }: { id: string; name: string; slug: string }) {
  const me = useAccount();
  const { data: caller } = useQuery(callerQuery(id));
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const { run, prompt } = useRecentAuth(me.hasPassword);

  const remove = useMutation({
    mutationFn: () => run(() => deleteWorkspace(id, typed)),
    onSuccess: (done) => {
      if (done) {
        window.location.assign("/d");
      }
    },
  });
  const slugError = fieldError(remove.error, "confirm_slug", ["confirm_slug"], "confirm_slug");

  if (caller?.member?.role !== "owner") {
    return null;
  }

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
    <Section title="Danger zone">
      <Row label="Delete workspace" description="Owners only. It can be restored for 30 days.">
        <Danger onPress={() => setOpen(true)}>Delete workspace</Danger>
      </Row>
      <Modal size="sm" isOpen={open} onOpenChange={(next) => (next ? null : close())}>
        <form onSubmit={submit}>
          <ModalHeader
            icon={RiDeleteBinLine}
            tone="danger"
            title={`Delete ${name}?`}
            description="It can be restored for 30 days. Everyone in it loses access straight away."
          />
          <ModalBody>
            <Input
              label="Workspace address to confirm"
              hint={
                <>
                  Type <span className="font-mono text-text-primary">{slug}</span> to confirm.
                </>
              }
              autoComplete="off"
              autoFocus
              inputDir="ltr"
              value={typed}
              onChange={(next) => {
                setTyped(next);
                remove.reset();
              }}
              isInvalid={Boolean(slugError)}
              errorMessage={slugError}
            />
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" variant="danger" disabled={typed !== slug || remove.isPending}>
              {remove.isPending ? "Deleting…" : "Delete workspace"}
            </Button>
          </ModalFooter>
        </form>
      </Modal>
      {prompt}
    </Section>
  );
}
