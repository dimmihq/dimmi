/*
 * Two-factor authentication on the security page: adding an
 * authenticator app, which turns it on and hands out recovery codes,
 * making new recovery codes, and turning it off. Secrets show once, in
 * the dialog that made them, and are gone when it closes.
 */
import { RiCheckLine, RiFileCopyLine, RiShieldCheckLine, RiSmartphoneLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { HintText } from "@/components/base/input/hint-text";
import { InputBase, TextField } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { InputOtp } from "@/components/base/input-otp/input-otp";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { fieldError } from "@/features/auth/form-errors";
import { meQuery } from "@/lib/api/auth";
import {
  type AuthFactor,
  activateFactor,
  addAuthenticator,
  authFactorsQuery,
  type NewAuthenticator,
  qrImage,
  regenerateRecoveryCodes,
  removeFactor,
  turnOffTwoFactor,
} from "@/lib/api/auth-factors";

import { Action, Danger } from "./controls";
import { Empty, Glyph, Item, Row, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";
import { remainingLabel, twoFactorState } from "./two-factor-state";
import { useCopy } from "./use-copy";

const dates = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });

type Dialog =
  | { kind: "setup"; factor: NewAuthenticator }
  | { kind: "codes"; codes: string[] }
  | { kind: "remove"; factor: AuthFactor }
  | { kind: "regenerate" }
  | { kind: "off" }
  | null;

export function TwoFactorSection({ hasPassword }: { hasPassword: boolean }) {
  const queryClient = useQueryClient();
  const { data: factors = [], isPending } = useQuery(authFactorsQuery);
  const { authenticator, recovery, on } = twoFactorState(factors);
  const [dialog, setDialog] = useState<Dialog>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const { run, prompt } = useRecentAuth(hasPassword);

  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: authFactorsQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: meQuery.queryKey }),
    ]);

  const attempt = async (action: () => Promise<unknown>) => {
    setFailure(null);

    try {
      await run(action);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "Something went wrong. Try again.");
    }

    await refresh();
  };

  const setup = () =>
    attempt(async () => {
      const factor = await addAuthenticator();
      setDialog({ kind: "setup", factor });
    });

  return (
    <Section title="Two-factor authentication" description="A code from an authenticator app at every sign-in.">
      {failure ? (
        <div className="py-2.5 pe-2.5">
          <Notification status="error" title={failure} dismissible={false} role="alert" className="w-full" />
        </div>
      ) : null}
      {isPending ? (
        <Empty>Loading…</Empty>
      ) : authenticator ? (
        <Item
          mark={<Glyph icon={RiSmartphoneLine} />}
          title={authenticator.name}
          detail={`Added ${dates.format(new Date(authenticator.confirmed_at ?? authenticator.created_at))}`}
        >
          <Action onPress={() => setDialog({ kind: "remove", factor: authenticator })}>Remove</Action>
        </Item>
      ) : (
        <Empty action={<Action onPress={setup}>Add authenticator</Action>}>No authenticator added</Empty>
      )}
      {on && recovery ? (
        <Row label="Recovery codes" description={remainingLabel(recovery.remaining ?? 0, recovery.total ?? 0)}>
          <Action onPress={() => setDialog({ kind: "regenerate" })}>Regenerate</Action>
        </Row>
      ) : (
        <Row label="Recovery codes" description="One-time codes for when your phone is not at hand." muted>
          <span className="text-body-2-regular text-text-tertiary">After an authenticator is added</span>
        </Row>
      )}
      {on ? (
        <Row label="Turn off two-factor" description="Removes your authenticator and recovery codes.">
          <Danger onPress={() => setDialog({ kind: "off" })}>Turn off</Danger>
        </Row>
      ) : null}

      {dialog?.kind === "setup" ? (
        <SetupDialog
          factor={dialog.factor}
          onActivated={(codes) => {
            void refresh();
            setDialog(codes ? { kind: "codes", codes } : null);
          }}
          onClose={() => {
            setDialog(null);
            void refresh();
          }}
        />
      ) : null}
      {dialog?.kind === "codes" ? <CodesDialog codes={dialog.codes} onClose={() => setDialog(null)} /> : null}
      <AlertDialog
        isOpen={dialog?.kind === "remove"}
        onOpenChange={(open) => !open && setDialog(null)}
        title="Remove this authenticator?"
        description="Two-factor authentication turns off and your recovery codes stop working."
        confirmLabel="Remove"
        onConfirm={() => {
          if (dialog?.kind === "remove") {
            const id = dialog.factor.id;
            void attempt(() => removeFactor(id));
          }
        }}
      />
      <AlertDialog
        isOpen={dialog?.kind === "regenerate"}
        onOpenChange={(open) => !open && setDialog(null)}
        variant="default"
        title="Make new recovery codes?"
        description="Your current recovery codes stop working at once."
        confirmLabel="Make new codes"
        onConfirm={() =>
          void attempt(async () => {
            const set = await regenerateRecoveryCodes();
            setDialog({ kind: "codes", codes: set.codes });
          })
        }
      />
      <AlertDialog
        isOpen={dialog?.kind === "off"}
        onOpenChange={(open) => !open && setDialog(null)}
        title="Turn off two-factor authentication?"
        description="Signing in will need only your password or a link again. Your apps on other devices are signed out."
        confirmLabel="Turn off"
        onConfirm={() => void attempt(turnOffTwoFactor)}
      />
      {prompt}
    </Section>
  );
}

function SetupDialog({
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

function CodesDialog({ codes, onClose }: { codes: string[]; onClose: () => void }) {
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
