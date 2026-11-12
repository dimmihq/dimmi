/*
 * Deleting a help center that is not the default, for owners. Its
 * articles and its custom domain go with it, and messengers that showed
 * it show the default help center instead, so the owner types its name
 * before the button works. The dashboard then works on the default one.
 */
import { RiDeleteBinLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { deleteHelpCenter } from "@/lib/api/help-center-sites";
import { type HelpCenter, helpCentersQuery } from "@/lib/api/help-centers";

import { Danger } from "./controls";
import { Row, Section } from "./layout";

export function DeleteHelpCenter({ workspace, center }: { workspace: string; center: HelpCenter }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const remove = useMutation({
    mutationFn: () => deleteHelpCenter(workspace, center.id, typed),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey }),
  });
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
      <Row label="Delete help center" description="Owners only. Its articles and domain are deleted with it.">
        <Danger onPress={() => setOpen(true)}>Delete help center</Danger>
      </Row>
      <Modal size="sm" isOpen={open} onOpenChange={(next) => (next ? null : close())}>
        <form onSubmit={submit}>
          <ModalHeader
            icon={RiDeleteBinLine}
            tone="danger"
            title={`Delete ${center.name}?`}
            description="Its articles and custom domain are deleted. Messengers showing it show your default help center."
          />
          <ModalBody className="flex flex-col gap-4">
            {remove.error ? (
              <Notification status="error" title={remove.error.message} dismissible={false} role="alert" />
            ) : null}
            <Input
              label="Help center name to confirm"
              hint={
                <>
                  Type <span className="font-mono text-text-primary">{center.name}</span> to confirm.
                </>
              }
              autoComplete="off"
              autoFocus
              value={typed}
              onChange={(next) => {
                setTyped(next);
                remove.reset();
              }}
            />
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" variant="danger" loading={remove.isPending} disabled={typed.trim() !== center.name}>
              Delete help center
            </Button>
          </ModalFooter>
        </form>
      </Modal>
    </Section>
  );
}
