/*
 * The dialogs of the two-factor settings: setting up an authenticator
 * app, naming a passkey, and showing new recovery codes once. Secrets
 * show only in the dialog that made them and are gone when it closes.
 */
import { RiCheckLine, RiFileCopyLine, RiKey2Line, RiShieldCheckLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { HintText } from "@/components/base/input/hint-text";
import { Input, InputBase, TextField } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { InputOtp } from "@/components/base/input-otp/input-otp";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { fieldError } from "@/features/auth/form-errors";
import { activateFactor, type NewAuthenticator, qrImage } from "@/lib/api/auth-factors";

import { useCopy } from "./use-copy";

/*
 * NameDialog asks for a passkey's name: before the device makes a new
 * one, or to rename one. An empty name for a new passkey keeps the
 * default.
 */
export function NameDialog({
  title,
  description,
  initial = "",
  submitLabel,
  optional = false,
  onSubmit,
  onClose,
}: {
  title: string;
  description: string;
  initial?: string;
  submitLabel: string;
  optional?: boolean;
  onSubmit: (name: string) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(initial);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSubmit(name);
  };

  return (
    <Modal size="sm" isOpen onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit}>
        <ModalHeader icon={RiKey2Line} title={title} description={description} />
        <ModalBody>
          <Input
            name="name"
            label="Name"
            placeholder="Work laptop"
            maxLength={60}
            autoFocus
            isRequired={!optional}
            validationBehavior="aria"
            value={name}
            onChange={setName}
          />
        </ModalBody>
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={!optional && name.trim() === ""}>
            {submitLabel}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

export function SetupDialog({
  factor,
  onActivated,
  onClose,
}: {
  factor: NewAuthenticator;
  onActivated: (codes: string[] | null) => void;
  onClose: () => void;
}) {
  const [code, setCode] = useState("");
  const activate = useMutation({
    mutationFn: (typed: string) => activateFactor(factor.id, typed),
    onSuccess: (active) => onActivated(active.recovery_codes?.codes ?? null),
    onError: () => setCode(""),
  });
  const codeError = fieldError(activate.error, "code", ["code"], "code");
  const check = (typed: string) => {
    if (!activate.isPending) {
      activate.mutate(typed);
    }
  };
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    check(code);
  };

  return (
    <Modal size="sm" isOpen onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit}>
        <ModalHeader
          icon={RiShieldCheckLine}
          title="Add an authenticator app"
          description="Scan the QR code with an app such as 1Password, Google Authenticator or Authy, then enter the code it shows."
        />
        <ModalBody className="flex flex-col items-center gap-5">
          <img
            src={qrImage(factor.setup.qr_svg)}
            alt="QR code to scan with your authenticator app"
            className="size-56 rounded-2xl bg-background-primary-default p-3"
          />
          <SetupKey secret={factor.setup.secret} />
          <div className="flex w-full flex-col items-center gap-1.5">
            <span className="text-body-medium text-text-secondary">Code from the app</span>
            <InputOtp
              value={code}
              onChange={(typed) => {
                setCode(typed);
                activate.reset();
              }}
              onComplete={check}
              length={6}
              groupEvery={3}
              isInvalid={Boolean(codeError)}
              aria-label="Code from the app"
              className="justify-center"
            />
            {codeError ? (
              <HintText isInvalid role="alert">
                {codeError}
              </HintText>
            ) : null}
          </div>
        </ModalBody>
        <ModalFooter className="mt-6">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={activate.isPending || code.length < 6}>
            {activate.isPending ? "Checking…" : "Turn on"}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

/*
 * SetupKey stays behind a link until the person cannot scan the code,
 * then shows the key on one line with a button that copies it.
 */
function SetupKey({ secret }: { secret: string }) {
  const [shown, setShown] = useState(false);
  const { copied, copy } = useCopy();

  if (!shown) {
    return (
      <LinkButton variant="secondary" size="xs" onClick={() => setShown(true)}>
        Can't scan the code?
      </LinkButton>
    );
  }

  return (
    <TextField value={secret} isReadOnly className="w-full">
      <Label>Enter this setup key in the app instead</Label>
      <InputBase
        dir="ltr"
        data-testid="totp-secret"
        className="font-mono text-body-2-regular"
        trailingAddon={
          <Button
            variant="ghost"
            size="xs"
            iconOnly
            leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
            aria-label={copied ? "Copied" : "Copy setup key"}
            onClick={() => void copy(secret)}
          />
        }
      />
    </TextField>
  );
}

export function CodesDialog({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const { copied, copy } = useCopy();

  return (
    <Modal size="sm" isOpen isDismissable={false} onOpenChange={(open) => !open && onClose()}>
      <ModalHeader
        icon={RiShieldCheckLine}
        title="Save your recovery codes"
        description="Each code signs you in once if your phone is not at hand. They show only now, so keep them somewhere safe."
        showCloseButton={false}
      />
      <ModalBody>
        <ol
          aria-label="Recovery codes"
          dir="ltr"
          className="grid grid-cols-2 gap-x-6 gap-y-2 rounded-xl bg-background-tertiary-default p-4 font-mono text-body-regular text-text-primary"
        >
          {codes.map((code) => (
            <li key={code} className="select-all">
              {code}
            </li>
          ))}
        </ol>
      </ModalBody>
      <ModalFooter>
        <Button
          variant="secondary"
          leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
          onClick={() => void copy(codes.join("\n"))}
        >
          {copied ? "Copied" : "Copy codes"}
        </Button>
        <Button onClick={onClose}>I saved them</Button>
      </ModalFooter>
    </Modal>
  );
}
