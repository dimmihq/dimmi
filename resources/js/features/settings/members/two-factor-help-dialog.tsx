/*
 * An admin helping a teammate who cannot sign in. Help with two-factor
 * mails the teammate a link that turns off the second factor they lost;
 * the admin never turns it off for them, and the teammate still needs
 * their password. Clear sign-in lock lifts the lock failed guesses set on
 * their account. Sending the link needs a recent proof of who the admin
 * is, asked for first when it is too old.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog, Modal, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { clearSignInLock, type Member, startTwoFactorReset } from "@/lib/api/workspace";

import { useRecentAuth } from "../recent-auth";
import type { Help } from "./sign-in-help";

function TwoFactorHelpDialog({
  workspace,
  member,
  hasPassword,
  onClose,
}: {
  workspace: string;
  member: Member | null;
  hasPassword: boolean;
  onClose: () => void;
}) {
  const { run, prompt } = useRecentAuth(hasPassword);
  const send = useMutation({
    mutationFn: (to: Member) => run(() => startTwoFactorReset(workspace, to.id)),
  });
  const name = member?.user.name ?? "";

  const close = () => {
    send.reset();
    onClose();
  };

  return (
    <Modal size="sm" isOpen={member !== null} onOpenChange={(open) => (open ? null : close())}>
      {send.data ? (
        <>
          <ModalHeader
            title="Link sent"
            description={`${name} has 30 minutes to open it from ${member?.user.email ?? "their email"}.`}
          />
          <ModalFooter className="mt-6">
            <Button onClick={close}>Done</Button>
          </ModalFooter>
        </>
      ) : (
        <>
          <ModalHeader
            title={`Help ${name} with two-factor`}
            description={`We email ${name} a link that turns off two-factor authentication. They need their password to use it, then set up a new authenticator. Every browser and app they use is signed out.`}
          />
          {send.error ? (
            <div className="mt-4">
              <Notification
                status="error"
                title={send.error.message}
                dismissible={false}
                role="alert"
                className="w-full"
              />
            </div>
          ) : null}
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button onClick={() => member && send.mutate(member)} loading={send.isPending}>
              Send reset link
            </Button>
          </ModalFooter>
        </>
      )}
      {prompt}
    </Modal>
  );
}

function SignInLockDialog({
  workspace,
  member,
  onClose,
}: {
  workspace: string;
  member: Member | null;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const clear = useMutation({
    mutationFn: (locked: Member) => clearSignInLock(workspace, locked.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "members"] }),
  });
  const name = member?.user.name ?? "";

  return (
    <AlertDialog
      isOpen={member !== null}
      onOpenChange={(open) => (open ? null : onClose())}
      variant="default"
      icon={false}
      title={`Clear ${name}'s sign-in lock?`}
      description={`Too many wrong passwords locked ${name} out for a while. Clear it only if you know it was them.`}
      confirmLabel="Clear lock"
      onConfirm={() => (member ? clear.mutateAsync(member) : undefined)}
    />
  );
}

/*
 * SignInHelpDialogs shows the dialog of the way to help the admin picked.
 */
export function SignInHelpDialogs({
  workspace,
  help,
  hasPassword,
  onClose,
}: {
  workspace: string;
  help: Help | null;
  hasPassword: boolean;
  onClose: () => void;
}) {
  return (
    <>
      <TwoFactorHelpDialog
        workspace={workspace}
        member={help?.kind === "two-factor" ? help.member : null}
        hasPassword={hasPassword}
        onClose={onClose}
      />
      <SignInLockDialog workspace={workspace} member={help?.kind === "lock" ? help.member : null} onClose={onClose} />
    </>
  );
}
