/*
 * The prompt for the password when a change needs a recent proof of who
 * is at the keyboard. A user without a password is told to sign in again
 * instead.
 */
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { fieldError } from "@/features/auth/form-errors";
import { confirmPassword } from "@/lib/api/auth-factors";

export function ConfirmPasswordDialog({
  isOpen,
  hasPassword,
  onCancel,
  onConfirmed,
}: {
  isOpen: boolean;
  hasPassword: boolean;
  onCancel: () => void;
  onConfirmed: () => void;
}) {
  const [password, setPassword] = useState("");
  const confirm = useMutation({
    mutationFn: () => confirmPassword(password),
    onSuccess: () => {
      setPassword("");
      onConfirmed();
    },
  });
  const passwordError = fieldError(confirm.error, "password", ["password", "method"], "password");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    confirm.mutate();
  };

  return (
    <Modal
      size="sm"
      isOpen={isOpen}
      onOpenChange={(next) => {
        if (!next) {
          confirm.reset();
          setPassword("");
          onCancel();
        }
      }}
    >
      {hasPassword ? (
        <form onSubmit={submit}>
          <ModalHeader title="Confirm it is you" description="Enter your password to make this change." />
          <ModalBody>
            <Input
              name="password"
              type="password"
              label="Password"
              autoComplete="current-password"
              autoFocus
              isRequired
              validationBehavior="aria"
              value={password}
              onChange={(typed) => {
                setPassword(typed);
                confirm.reset();
              }}
              isInvalid={Boolean(passwordError)}
              errorMessage={passwordError}
            />
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={onCancel}>
              Cancel
            </Button>
            <Button type="submit" loading={confirm.isPending} disabled={password === ""}>
              Confirm
            </Button>
          </ModalFooter>
        </form>
      ) : (
        <>
          <ModalHeader
            title="Sign in again to continue"
            description="This change needs a recent sign-in. Sign out, sign in again, and come back here."
          />
          <ModalFooter>
            <Button onClick={onCancel}>Close</Button>
          </ModalFooter>
        </>
      )}
    </Modal>
  );
}
