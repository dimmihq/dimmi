/*
 * Connecting a ChatGPT subscription instead of a key: the person signs
 * in at OpenAI, which sends them back to dimmi with a one-time code, and
 * dimmi trades that code for the subscription's tokens on the server.
 * The browser never holds the tokens. This step only starts the sign-in;
 * the page OpenAI returns to finishes it.
 */
import { RiArrowRightUpLine, RiErrorWarningLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { ModalBody, ModalFooter } from "@/components/base/modal/modal";
import { type AiProviderDescriptor, startChatGPTConnect } from "@/lib/api/ai";

import { ProviderMark } from "../provider-mark";
import { refusal } from "../use-agent";

const steps = [
  "Sign in to your ChatGPT account at OpenAI.",
  "Approve dimmi's access to your plan.",
  "OpenAI sends you back here with the connection ready.",
];

/*
 * ChatGPTConnect explains the sign-in and starts it.
 */
export function ChatGPTConnect({
  workspace,
  descriptor,
  onCancel,
}: {
  workspace: string;
  descriptor: AiProviderDescriptor;
  onCancel: () => void;
}) {
  const start = useMutation({
    mutationFn: () => startChatGPTConnect(workspace),
    onSuccess: ({ authorize_url }) => window.location.assign(authorize_url),
  });

  return (
    <>
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
        {descriptor.available ? (
          <ol className="flex flex-col gap-3">
            {steps.map((step, index) => (
              <li key={step} className="flex items-center gap-3">
                <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-background-tertiary-default text-caption-1-semibold text-text-secondary">
                  {index + 1}
                </span>
                <span className="text-body-regular text-text-primary">{step}</span>
              </li>
            ))}
          </ol>
        ) : (
          <p className="rounded-xl bg-background-secondary-default p-3 text-body-2-regular text-text-secondary">
            ChatGPT sign-in is not turned on for this install. Ask whoever runs dimmi to enable it, or connect an OpenAI
            key instead.
          </p>
        )}
        {start.error ? (
          <p
            role="alert"
            className="flex items-start gap-2 rounded-xl bg-notification-error-background p-3 text-body-2-regular text-notification-error-foreground"
          >
            <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0" />
            <span>{refusal(start.error, "The sign-in could not start. Try again.")}</span>
          </p>
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          trailingIcon={RiArrowRightUpLine}
          disabled={!descriptor.available}
          loading={start.isPending || start.isSuccess}
          onClick={() => start.mutate()}
        >
          Sign in with ChatGPT
        </Button>
      </ModalFooter>
    </>
  );
}
