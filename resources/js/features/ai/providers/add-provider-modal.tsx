/*
 * Adding a provider, in two steps: pick one from a grid of cards, then
 * give what it needs. Most take an API key; a ChatGPT subscription signs
 * in instead. dimmi's own models are never in the grid, since they come
 * with the workspace. A provider this install cannot serve shows, greyed,
 * with the reason.
 */
import { RiArrowLeftSLine } from "@remixicon/react";
import { useState } from "react";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import type { AiProviderDescriptor } from "@/lib/api/ai";
import { cx } from "@/utils/cx";
import { ProviderLogo } from "../logos";
import { providerInfo, providerName } from "../provider-info";
import { ChatGPTConnect } from "./chatgpt-connect";
import { ProviderForm } from "./provider-form";

/*
 * ProviderCard is one provider in the grid.
 */
function ProviderCard({ descriptor, onPick }: { descriptor: AiProviderDescriptor; onPick: () => void }) {
  const info = providerInfo[descriptor.id];

  return (
    <button
      type="button"
      onClick={onPick}
      disabled={!descriptor.available}
      className={cx(
        "flex min-h-[112px] w-full flex-col items-start gap-3 rounded-2xl bg-background-secondary-default p-4 text-start outline-none",
        "transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        descriptor.available ? "cursor-pointer hover:bg-background-secondary-hover" : "cursor-not-allowed opacity-60",
      )}
    >
      <ProviderLogo provider={descriptor.id} />
      <span className="flex flex-col gap-0.5">
        <span className="text-body-medium text-text-primary">{providerName(descriptor.id)}</span>
        <span className="text-body-2-regular text-text-secondary">
          {descriptor.available ? (info?.blurb ?? descriptor.name) : "Not available on this install."}
        </span>
      </span>
    </button>
  );
}

/*
 * ProviderGrid is the first step: every provider that can be added.
 */
function ProviderGrid({
  providers,
  onPick,
}: {
  providers: AiProviderDescriptor[];
  onPick: (descriptor: AiProviderDescriptor) => void;
}) {
  return (
    <ModalBody className="pb-1">
      <ul aria-label="Providers" className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {providers.map((descriptor) => (
          <li key={descriptor.id} className="flex">
            <ProviderCard descriptor={descriptor} onPick={() => onPick(descriptor)} />
          </li>
        ))}
      </ul>
    </ModalBody>
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
  const choices = providers.filter((descriptor) => descriptor.id !== "dimmi");

  const close = () => {
    setPicked(null);
    onClose();
  };
  const back = () => setPicked(null);

  return (
    <Modal
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          close();
        }
      }}
      size="lg"
    >
      <ModalHeader
        title={picked ? `Connect ${providerName(picked.id)}` : "Add a provider"}
        description={
          picked
            ? "dimmi checks the connection before it saves."
            : "Choose where the agent's models come from. Usage is billed by the provider."
        }
      >
        {picked ? (
          <LinkButton
            variant="secondary"
            size="small"
            leadingIcon={RiArrowLeftSLine}
            onClick={back}
            className="mt-3 -ms-1 self-start"
          >
            All providers
          </LinkButton>
        ) : null}
      </ModalHeader>
      {picked === null ? <ProviderGrid providers={choices} onPick={setPicked} /> : null}
      {picked?.auth_type === "oauth" ? (
        <ChatGPTConnect
          workspace={workspace}
          descriptor={picked}
          onCancel={close}
          onDone={() => {
            setPicked(null);
            onClose();
          }}
        />
      ) : null}
      {picked && picked.auth_type !== "oauth" ? (
        <ProviderForm key={picked.id} workspace={workspace} descriptor={picked} onCancel={close} onDone={close} />
      ) : null}
    </Modal>
  );
}
