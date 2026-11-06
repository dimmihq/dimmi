/*
 * Connecting a ChatGPT subscription instead of a key, inside the Add
 * provider modal. The device code comes first and large, with a copy
 * button and a button that opens OpenAI's page, while the step waits for
 * the approval on its own. A link swaps to the browser sign-in, where the
 * person opens OpenAI's page in a new tab and pastes back the address it
 * ended on. Once connected the modal closes and the new connection shows
 * in the list.
 */
import { RiArrowRightUpLine, RiErrorWarningLine, RiFileCopyLine } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { ModalBody, ModalFooter } from "@/components/base/modal/modal";
import { Spinner } from "@/components/base/spinner/spinner";
import { toast } from "@/components/base/toast/toast";
import { useCopy } from "@/features/settings/use-copy";
import { type AiProviderDescriptor, aiKey } from "@/lib/api/ai";

import { ProviderMark } from "../provider-mark";
import { type Phase, type SignInMethod, useChatGPTSignIn } from "./use-chatgpt-sign-in";

const alert =
  "flex items-start gap-2 rounded-xl bg-notification-error-background p-3 text-body-2-regular text-notification-error-foreground";

function open(url: string) {
  window.open(url, "_blank", "noopener,noreferrer");
}

function Step({ n, children }: { n: number; children: string }) {
  return (
    <li className="flex items-start gap-3">
      <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-background-tertiary-default text-caption-1-semibold text-text-secondary">
        {n}
      </span>
      <span className="pt-0.5 text-body-regular text-text-primary">{children}</span>
    </li>
  );
}

/*
 * DeviceStep shows the code to type on OpenAI's page and waits.
 */
function DeviceStep({ phase, onBrowser }: { phase: Extract<Phase, { kind: "device" }>; onBrowser: () => void }) {
  const { copied, copy } = useCopy();
  const { url, user_code } = phase.start;

  return (
    <div className="flex flex-col gap-4">
      <ol className="flex flex-col gap-3">
        <Step n={1}>Open ChatGPT and sign in to the account with your plan.</Step>
        <Step n={2}>Enter this code when it asks for one.</Step>
      </ol>
      <div className="flex items-center justify-between gap-3 rounded-2xl bg-background-secondary-default p-4">
        <span className="font-mono text-title-1-medium text-text-primary tracking-[0.2em]">
          <span className="sr-only">Your code: </span>
          {user_code}
        </span>
        <Button variant="secondary" size="small" leadingIcon={RiFileCopyLine} onClick={() => void copy(user_code)}>
          {copied ? "Copied" : "Copy"}
        </Button>
      </div>
      <Button trailingIcon={RiArrowRightUpLine} onClick={() => open(url)} className="self-start">
        Open ChatGPT
      </Button>
      <p role="status" className="flex items-center gap-2 text-body-2-regular text-text-secondary">
        <Spinner size="xs" aria-hidden />
        Waiting for you to approve…
      </p>
      <LinkButton variant="secondary" size="small" onClick={onBrowser} className="self-start">
        Use browser sign-in instead
      </LinkButton>
    </div>
  );
}

/*
 * BrowserStep opens OpenAI's page and takes back the address it ended
 * on.
 */
function BrowserStep({
  phase,
  address,
  problem,
  onAddress,
  onDevice,
}: {
  phase: Extract<Phase, { kind: "browser" }>;
  address: string;
  problem: string | null;
  onAddress: (address: string) => void;
  onDevice: () => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      <ol className="flex flex-col gap-3">
        <Step n={1}>Open the sign-in page and approve dimmi's access to your plan.</Step>
        <Step n={2}>Your browser then lands on a page that does not load. Copy its address and paste it below.</Step>
      </ol>
      <Button
        variant="secondary"
        trailingIcon={RiArrowRightUpLine}
        onClick={() => open(phase.start.url)}
        className="self-start"
      >
        Open the sign-in page
      </Button>
      <Input
        label="Paste the address your browser ended on"
        placeholder={`${phase.start.redirect_uri}?code=…`}
        value={address}
        onChange={onAddress}
        autoComplete="off"
        isInvalid={problem !== null}
        errorMessage={problem ?? undefined}
      />
      <LinkButton variant="secondary" size="small" onClick={onDevice} className="self-start">
        Use a code instead
      </LinkButton>
    </div>
  );
}

function Failed({
  phase,
  onRetry,
}: {
  phase: Extract<Phase, { kind: "failed" }>;
  onRetry: (method: SignInMethod) => void;
}) {
  const deviceOff = phase.code === "chatgpt_device_disabled";

  return (
    <div className="flex flex-col gap-3">
      <p role="alert" className={alert}>
        <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0" />
        <span>{phase.message}</span>
      </p>
      {phase.code === "chatgpt_unavailable" ? null : (
        <Button
          variant="secondary"
          size="small"
          onClick={() => onRetry(deviceOff ? "browser" : phase.method)}
          className="self-start"
        >
          {deviceOff ? "Use browser sign-in" : "Start again"}
        </Button>
      )}
    </div>
  );
}

function Starting() {
  return (
    <p role="status" className="flex items-center gap-2 py-6 text-body-2-regular text-text-secondary">
      <Spinner size="xs" aria-hidden />
      Starting the sign-in…
    </p>
  );
}

/*
 * ChatGPTConnect is the modal step for a ChatGPT subscription.
 */
export function ChatGPTConnect({
  workspace,
  descriptor,
  onCancel,
  onDone,
}: {
  workspace: string;
  descriptor: AiProviderDescriptor;
  onCancel: () => void;
  onDone: () => void;
}) {
  const client = useQueryClient();
  const [address, setAddress] = useState("");
  const [problem, setProblem] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const { phase, begin, paste } = useChatGPTSignIn(workspace, descriptor.available, () => {
    void client.invalidateQueries({ queryKey: aiKey(workspace) });
    toast.success("ChatGPT connected", { description: "Its models are ready to choose for the agent." });
    onDone();
  });

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (address.trim() === "") {
      setProblem("Paste the full address from your browser's address bar.");
      return;
    }
    setBusy(true);
    setProblem(await paste(address));
    setBusy(false);
  };
  const restart = (method: SignInMethod) => {
    setAddress("");
    setProblem(null);
    void begin(method);
  };

  return (
    <form className="contents" onSubmit={submit} noValidate>
      <ModalBody className="flex flex-col gap-5 pb-1">
        <div className="flex items-center gap-3 rounded-2xl bg-background-secondary-default p-3">
          <ProviderMark provider="chatgpt" size="lg" />
          <div className="flex min-w-0 flex-col">
            <span className="text-body-medium text-text-primary">ChatGPT subscription</span>
            <span className="text-body-2-regular text-text-secondary">
              Answers count against your ChatGPT plan, not a metered key.
            </span>
          </div>
        </div>
        {descriptor.available ? null : (
          <p className="rounded-xl bg-background-secondary-default p-3 text-body-2-regular text-text-secondary">
            ChatGPT sign-in is not turned on for this install. Ask whoever runs dimmi to enable it, or connect an OpenAI
            key instead.
          </p>
        )}
        {descriptor.available && phase.kind === "starting" ? <Starting /> : null}
        {phase.kind === "device" ? <DeviceStep phase={phase} onBrowser={() => restart("browser")} /> : null}
        {phase.kind === "browser" ? (
          <BrowserStep
            phase={phase}
            address={address}
            problem={problem}
            onAddress={(next) => {
              setAddress(next);
              setProblem(null);
            }}
            onDevice={() => restart("device")}
          />
        ) : null}
        {phase.kind === "failed" ? <Failed phase={phase} onRetry={restart} /> : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onCancel} disabled={busy}>
          Cancel
        </Button>
        {phase.kind === "browser" ? (
          <Button type="submit" loading={busy}>
            Connect
          </Button>
        ) : null}
      </ModalFooter>
    </form>
  );
}
