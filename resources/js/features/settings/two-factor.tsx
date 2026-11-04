/*
 * Two-factor authentication on the security page: every second factor
 * the person holds, an authenticator app and the passkeys on their
 * devices, each removable, passkeys also renamable; adding either, which
 * turns two-factor on and hands out recovery codes the first time;
 * making new recovery codes; and turning it all off. Secrets show once,
 * in the dialog that made them, and are gone when it closes.
 */
import { RiKey2Line, RiSmartphoneLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { meQuery } from "@/lib/api/auth";
import {
  type AuthFactor,
  addAuthenticator,
  authFactorsQuery,
  finishPasskey,
  type NewAuthenticator,
  regenerateRecoveryCodes,
  removeFactor,
  renameFactor,
  startPasskey,
  turnOffTwoFactor,
} from "@/lib/api/auth-factors";
import { passkeysSupported } from "@/lib/passkeys";

import { Action, Danger } from "./controls";
import { Empty, Glyph, Item, Row, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";
import { CodesDialog, NameDialog, SetupDialog } from "./two-factor-dialogs";
import { isLastFactor, remainingLabel, twoFactorState, usedLabel } from "./two-factor-state";

const dates = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" });

type Dialog =
  | { kind: "setup"; factor: NewAuthenticator }
  | { kind: "codes"; codes: string[] }
  | { kind: "add-passkey" }
  | { kind: "rename"; factor: AuthFactor }
  | { kind: "remove"; factor: AuthFactor }
  | { kind: "regenerate" }
  | { kind: "off" }
  | null;

/*
 * removalText says what removing the factor does: the last one turns
 * two-factor off with it.
 */
function removalText(factors: AuthFactor[], factor: AuthFactor) {
  const title = factor.type === "passkey" ? "Remove this passkey?" : "Remove this authenticator?";

  if (isLastFactor(factors, factor)) {
    return {
      title,
      description:
        "It is your last second factor, so two-factor authentication turns off and your recovery codes stop working.",
    };
  }

  return { title, description: "You can no longer use it to sign in." };
}

export function TwoFactorSection({ hasPassword }: { hasPassword: boolean }) {
  const queryClient = useQueryClient();
  const { data: factors = [], isPending } = useQuery(authFactorsQuery);
  const { recovery, on } = twoFactorState(factors);
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

  return (
    <Section
      title="Two-factor authentication"
      description="A second step at every sign-in: a code from an authenticator app or a passkey on your device."
    >
      {failure ? (
        <div className="py-2.5 pe-2.5">
          <Notification status="error" title={failure} dismissible={false} role="alert" className="w-full" />
        </div>
      ) : null}
      {isPending ? (
        <Empty>Loading…</Empty>
      ) : (
        <Methods
          factors={factors}
          onSetup={() =>
            attempt(async () => {
              const factor = await addAuthenticator();
              setDialog({ kind: "setup", factor });
            })
          }
          onOpen={setDialog}
        />
      )}
      {on && recovery ? (
        <Row label="Recovery codes" description={remainingLabel(recovery.remaining ?? 0, recovery.total ?? 0)}>
          <Action onPress={() => setDialog({ kind: "regenerate" })}>Regenerate</Action>
        </Row>
      ) : (
        <Row label="Recovery codes" description="One-time codes for when your devices are not at hand." muted>
          <span className="text-body-2-regular text-text-tertiary">After a second factor is added</span>
        </Row>
      )}
      {on ? (
        <Row label="Turn off two-factor" description="Removes your authenticator, passkeys and recovery codes.">
          <Danger onPress={() => setDialog({ kind: "off" })}>Turn off</Danger>
        </Row>
      ) : null}
      <Dialogs
        dialog={dialog}
        factors={factors}
        attempt={attempt}
        refresh={refresh}
        onOpen={setDialog}
        onClose={() => setDialog(null)}
      />
      {prompt}
    </Section>
  );
}

/*
 * Methods lists the authenticator app, or offers to add one, then each
 * passkey with what can be done with it, and the way to add another.
 */
function Methods({
  factors,
  onSetup,
  onOpen,
}: {
  factors: AuthFactor[];
  onSetup: () => void;
  onOpen: (dialog: Dialog) => void;
}) {
  const { authenticator, passkeys } = twoFactorState(factors);

  return (
    <>
      {authenticator ? (
        <Item
          mark={<Glyph icon={RiSmartphoneLine} />}
          title={authenticator.name}
          detail={usedLabel(authenticator, dates)}
        >
          <Action onPress={() => onOpen({ kind: "remove", factor: authenticator })}>Remove</Action>
        </Item>
      ) : (
        <Empty action={<Action onPress={onSetup}>Add authenticator</Action>}>No authenticator app added</Empty>
      )}
      {passkeys.map((passkey) => (
        <Item
          key={passkey.id}
          mark={<Glyph icon={RiKey2Line} />}
          title={passkey.name}
          detail={usedLabel(passkey, dates)}
        >
          <Action onPress={() => onOpen({ kind: "rename", factor: passkey })}>Rename</Action>
          <Action onPress={() => onOpen({ kind: "remove", factor: passkey })}>Remove</Action>
        </Item>
      ))}
      {passkeysSupported() ? (
        <Row
          label="Passkeys"
          description={
            passkeys.length > 0
              ? "Add one for each device you sign in from."
              : "Sign in with your fingerprint, face or screen lock instead of a code."
          }
        >
          <Action onPress={() => onOpen({ kind: "add-passkey" })}>Add passkey</Action>
        </Row>
      ) : (
        <Row label="Passkeys" description="This browser cannot use passkeys." muted />
      )}
    </>
  );
}

/*
 * Dialogs shows the open dialog and runs what it confirms.
 */
function Dialogs({
  dialog,
  factors,
  attempt,
  refresh,
  onOpen,
  onClose,
}: {
  dialog: Dialog;
  factors: AuthFactor[];
  attempt: (action: () => Promise<unknown>) => Promise<void>;
  refresh: () => Promise<unknown>;
  onOpen: (dialog: Dialog) => void;
  onClose: () => void;
}) {
  const removal = dialog?.kind === "remove" ? removalText(factors, dialog.factor) : null;

  const addPasskey = (name: string) => {
    onClose();
    void attempt(async () => {
      const active = await finishPasskey(await startPasskey(name));
      const codes = active.recovery_codes?.codes;
      onOpen(codes ? { kind: "codes", codes } : null);
    });
  };

  return (
    <>
      {dialog?.kind === "setup" ? (
        <SetupDialog
          factor={dialog.factor}
          onActivated={(codes) => {
            void refresh();
            onOpen(codes ? { kind: "codes", codes } : null);
          }}
          onClose={() => {
            onClose();
            void refresh();
          }}
        />
      ) : null}
      {dialog?.kind === "add-passkey" ? (
        <NameDialog
          title="Add a passkey"
          description="Name it after the device it lives on, then follow your browser's prompt."
          submitLabel="Continue"
          optional
          onSubmit={addPasskey}
          onClose={onClose}
        />
      ) : null}
      {dialog?.kind === "rename" ? (
        <NameDialog
          title="Rename passkey"
          description="A name helps you tell your passkeys apart."
          initial={dialog.factor.name}
          submitLabel="Save"
          onSubmit={(name) => {
            onClose();
            void attempt(() => renameFactor(dialog.factor.id, name));
          }}
          onClose={onClose}
        />
      ) : null}
      {dialog?.kind === "codes" ? <CodesDialog codes={dialog.codes} onClose={onClose} /> : null}
      <AlertDialog
        isOpen={dialog?.kind === "remove"}
        onOpenChange={(open) => !open && onClose()}
        title={removal?.title ?? ""}
        description={removal?.description ?? ""}
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
        onOpenChange={(open) => !open && onClose()}
        variant="default"
        title="Make new recovery codes?"
        description="Your current recovery codes stop working at once."
        confirmLabel="Make new codes"
        onConfirm={() =>
          void attempt(async () => {
            const set = await regenerateRecoveryCodes();
            onOpen({ kind: "codes", codes: set.codes });
          })
        }
      />
      <AlertDialog
        isOpen={dialog?.kind === "off"}
        onOpenChange={(open) => !open && onClose()}
        title="Turn off two-factor authentication?"
        description="Signing in will need only your password or a link again. Your apps on other devices are signed out."
        confirmLabel="Turn off"
        onConfirm={() => void attempt(turnOffTwoFactor)}
      />
    </>
  );
}
