/*
 * Changing or setting the person's password. Someone with a password types
 * it; someone without proves who they are first, if the server asks. Every
 * other browser and app is signed out. Access tokens keep working, so the
 * dialog ends by saying how many do and where to review them.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { fieldError } from "@/features/auth/form-errors";
import { ButtonLink } from "@/features/navigation/links";
import { changePassword, type PasswordChange } from "@/lib/api/account";

import { Action } from "./controls";
import { Row, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";

function PasswordResult({ remaining, onClose }: { remaining: number; onClose: () => void }) {
  return (
    <>
      <ModalHeader
        title="Password saved"
        description={
          remaining === 0
            ? "Every other browser and app was signed out."
            : `Every other browser and app was signed out. ${remaining === 1 ? "1 API token still works" : `${remaining} API tokens still work`}.`
        }
      />
      <ModalFooter className="mt-6">
        {remaining > 0 ? (
          <ButtonLink to="/d/settings/tokens" variant="secondary">
            Review tokens
          </ButtonLink>
        ) : null}
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </>
  );
}

function PasswordDialog({
  isOpen,
  hasPassword,
  onClose,
}: {
  isOpen: boolean;
  hasPassword: boolean;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [current, setCurrent] = useState("");
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const [result, setResult] = useState<PasswordChange | null>(null);
  const { run, prompt } = useRecentAuth(hasPassword);

  const save = useMutation({
    mutationFn: async () => {
      let change: PasswordChange | null = null;
      await run(async () => {
        change = await changePassword(password, hasPassword ? current : undefined);
      });

      return change;
    },
    onSuccess: async (change) => {
      if (change) {
        setResult(change);
        await queryClient.invalidateQueries({ queryKey: ["sessions"] });
      }
    },
  });
  const mismatch = again !== "" && again !== password ? "The passwords do not match." : undefined;
  const fields = ["current_password", "password"];
  const currentError = fieldError(save.error, "current_password", fields, "password");
  const passwordError = fieldError(save.error, "password", fields, "password");

  const close = () => {
    setCurrent("");
    setPassword("");
    setAgain("");
    setResult(null);
    save.reset();
    if (result) {
      void queryClient.invalidateQueries({ queryKey: ["me"] });
    }
    onClose();
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate();
  };

  return (
    <Modal size="sm" isOpen={isOpen} onOpenChange={(next) => (next ? null : close())}>
      {result ? (
        <PasswordResult remaining={result.tokens_remaining} onClose={close} />
      ) : (
        <form onSubmit={submit}>
          <ModalHeader
            title={hasPassword ? "Change your password" : "Set a password"}
            description="Every other browser and app is signed out."
          />
          <ModalBody className="flex flex-col gap-4">
            {hasPassword ? (
              <Input
                name="current_password"
                type="password"
                label="Current password"
                autoComplete="current-password"
                autoFocus
                value={current}
                onChange={setCurrent}
                isInvalid={Boolean(currentError)}
                errorMessage={currentError}
              />
            ) : null}
            <Input
              name="password"
              type="password"
              label="New password"
              autoComplete="new-password"
              value={password}
              onChange={setPassword}
              isInvalid={Boolean(passwordError)}
              errorMessage={passwordError}
            />
            <Input
              name="password_again"
              type="password"
              label="Repeat new password"
              autoComplete="new-password"
              value={again}
              onChange={setAgain}
              isInvalid={Boolean(mismatch)}
              errorMessage={mismatch}
            />
          </ModalBody>
          <ModalFooter className="mt-6">
            <Button variant="secondary" onClick={close}>
              Cancel
            </Button>
            <Button
              type="submit"
              loading={save.isPending}
              disabled={password === "" || password !== again || (hasPassword && current === "")}
            >
              Save password
            </Button>
          </ModalFooter>
        </form>
      )}
      {prompt}
    </Modal>
  );
}

/*
 * PasswordSection is the password row of the security page and the dialog
 * it opens.
 */
export function PasswordSection({ hasPassword }: { hasPassword: boolean }) {
  const [open, setOpen] = useState(false);

  return (
    <Section title="Password">
      <Row
        label="Password"
        description={
          hasPassword ? "Changing it signs you out everywhere else." : "You sign in with a link or a provider."
        }
      >
        <Action onPress={() => setOpen(true)}>{hasPassword ? "Change password" : "Set a password"}</Action>
      </Row>
      <PasswordDialog isOpen={open} hasPassword={hasPassword} onClose={() => setOpen(false)} />
    </Section>
  );
}
