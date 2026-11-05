/*
 * The address the person signs in with. Moving it takes a recent proof of
 * who they are and a link opened from the new mailbox, so until then the
 * row says where the link went and offers to send it again. Asking again,
 * for this address or another, replaces the link before it.
 */
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { fieldError } from "@/features/auth/form-errors";
import { type EmailChange, requestEmailChange } from "@/lib/api/account";

import { Action } from "../controls";
import { Row } from "../layout";
import { useRecentAuth } from "../recent-auth";

export function EmailRow({ email, hasPassword }: { email: string; hasPassword: boolean }) {
  const [open, setOpen] = useState(false);
  const [typed, setTyped] = useState("");
  const [pending, setPending] = useState<{ change: EmailChange; address: string } | null>(null);
  const { run, prompt } = useRecentAuth(hasPassword);

  const send = useMutation({
    mutationFn: async (address: string) => {
      let change: EmailChange | null = null;
      const done = await run(async () => {
        change = await requestEmailChange(address);
      });

      return done && change ? { change, address } : null;
    },
    onSuccess: (sent) => {
      if (sent) {
        setPending(sent);
        setOpen(false);
        setTyped("");
      }
    },
  });
  const failure = fieldError(send.error, "email", ["email"], "email");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    send.mutate(typed);
  };

  const close = () => {
    send.reset();
    setOpen(false);
  };

  return (
    <>
      <Row label="Email" description="Changing it sends a link to the new address first.">
        <bdi className="block min-w-0 max-w-full truncate text-body-2-regular text-text-secondary sm:max-w-64">
          {email}
        </bdi>
        <Action onPress={() => setOpen(true)}>{pending ? "Use another address" : "Change"}</Action>
      </Row>
      {pending ? (
        <Row
          label="Waiting for the new address"
          description={`Open the link we sent to ${pending.change.email} to finish. It works for 24 hours.`}
        >
          <Action onPress={() => send.mutate(pending.address)}>Resend</Action>
        </Row>
      ) : null}
      <Modal size="sm" isOpen={open} onOpenChange={(next) => (next ? setOpen(true) : close())}>
        <form onSubmit={submit}>
          <ModalHeader
            title="Change your email"
            description="We send a link to the new address, and tell this one. Nothing changes until the link is opened."
          />
          <ModalBody>
            <Input
              name="email"
              type="email"
              label="New email"
              autoComplete="email"
              autoFocus
              isRequired
              validationBehavior="aria"
              inputDir="ltr"
              value={typed}
              onChange={(next) => {
                setTyped(next);
                send.reset();
              }}
              isInvalid={Boolean(failure)}
              errorMessage={failure}
            />
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button type="submit" disabled={typed === "" || send.isPending}>
              {send.isPending ? "Sending…" : "Send link"}
            </Button>
          </ModalFooter>
        </form>
      </Modal>
      {prompt}
    </>
  );
}
