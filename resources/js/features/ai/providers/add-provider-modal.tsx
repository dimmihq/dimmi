/*
 * Adding a provider, in two steps inside one narrow dialog of a fixed
 * height, the same as picking a workflow's trigger: first a list of
 * providers under their headings, each with its mark, a line on what it
 * offers and a chevron, then what the picked one needs. Most take an API
 * key; a ChatGPT subscription signs in instead. The steps slide into one
 * another the way the person is heading, and the dialog never changes
 * size between them. dimmi's own models and Engy are never offered. A
 * provider this install cannot serve shows, greyed, with the reason.
 * Each opening starts again from the list.
 */
import { RiArrowLeftSLine, RiArrowRightSLine } from "@remixicon/react";
import { useState } from "react";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import { Swap } from "@/features/inbox/motion";
import type { AiProviderDescriptor } from "@/lib/api/ai";
import { cx } from "@/utils/cx";
import { ProviderLogo } from "../logos";
import { providerInfo, providerName } from "../provider-info";
import { ChatGPTConnect } from "./chatgpt-connect";
import { providerGroups } from "./choices";
import { ProviderForm } from "./provider-form";

/*
 * Choice is one provider's row in the list.
 */
function Choice({ descriptor, onPick }: { descriptor: AiProviderDescriptor; onPick: () => void }) {
  const info = providerInfo[descriptor.id];

  return (
    <button
      type="button"
      onClick={onPick}
      disabled={!descriptor.available}
      className={cx(
        "flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none",
        "transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        descriptor.available ? "cursor-pointer hover:bg-background-secondary-hover" : "cursor-not-allowed opacity-60",
      )}
    >
      <ProviderLogo provider={descriptor.id} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-regular text-text-primary">{providerName(descriptor.id)}</span>
        <span className="text-body-2-regular text-text-secondary">
          {descriptor.available ? (info?.blurb ?? descriptor.name) : "Not available on this install."}
        </span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

/*
 * ProviderList is the first step: every provider that can be added,
 * under its heading.
 */
function ProviderList({
  providers,
  onPick,
}: {
  providers: AiProviderDescriptor[];
  onPick: (descriptor: AiProviderDescriptor) => void;
}) {
  return (
    <>
      <ModalHeader
        title="Add a provider"
        description="Choose where the agent's models come from. You can add more later."
      />
      <ModalBody>
        <div className="flex flex-col gap-5 pb-2">
          {providerGroups(providers).map((group) => (
            <section key={group.id} aria-label={group.name} className="flex flex-col gap-2">
              <div className="flex flex-col">
                <h3 className="text-body-2-medium text-text-primary">{group.name}</h3>
                <p className="text-body-2-regular text-text-secondary">{group.detail}</p>
              </div>
              <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
                {group.providers.map((descriptor) => (
                  <Choice key={descriptor.id} descriptor={descriptor} onPick={() => onPick(descriptor)} />
                ))}
              </div>
            </section>
          ))}
        </div>
      </ModalBody>
    </>
  );
}

/*
 * Connect is the second step: what the picked provider needs, with the
 * way back to the list.
 */
function Connect({
  workspace,
  descriptor,
  onBack,
  onClose,
}: {
  workspace: string;
  descriptor: AiProviderDescriptor;
  onBack: () => void;
  onClose: () => void;
}) {
  return (
    <>
      <ModalHeader
        title={`Connect ${providerName(descriptor.id)}`}
        description="dimmi checks the connection before it saves."
      >
        <LinkButton
          variant="secondary"
          size="small"
          leadingIcon={RiArrowLeftSLine}
          onClick={onBack}
          className="mt-3 -ms-1 self-start rtl:[&_svg]:-scale-x-100"
        >
          All providers
        </LinkButton>
      </ModalHeader>
      {descriptor.auth_type === "oauth" ? (
        <ChatGPTConnect workspace={workspace} descriptor={descriptor} onCancel={onClose} onDone={onClose} />
      ) : (
        <ProviderForm workspace={workspace} descriptor={descriptor} onCancel={onClose} onDone={onClose} />
      )}
    </>
  );
}

/*
 * AddProviderModal is the whole flow.
 */
export function AddProviderModal({
  workspace,
  providers,
  isOpen,
  onClose,
}: {
  workspace: string;
  providers: AiProviderDescriptor[];
  isOpen: boolean;
  onClose: () => void;
}) {
  const [picked, setPicked] = useState<AiProviderDescriptor | null>(null);
  const [wasOpen, setWasOpen] = useState(isOpen);

  if (isOpen !== wasOpen) {
    setWasOpen(isOpen);
    if (isOpen) {
      setPicked(null);
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      size="md"
      className="relative h-[min(640px,calc(100dvh-3rem))]"
    >
      <Swap view={picked?.id ?? "list"} forward={picked !== null} className="flex min-h-0 flex-1 flex-col gap-4">
        {picked ? (
          <Connect workspace={workspace} descriptor={picked} onBack={() => setPicked(null)} onClose={onClose} />
        ) : (
          <ProviderList providers={providers} onPick={setPicked} />
        )}
      </Swap>
    </Modal>
  );
}
