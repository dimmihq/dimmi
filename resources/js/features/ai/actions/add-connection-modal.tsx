/*
 * Adding a connection, in two steps inside one narrow dialog of a fixed
 * height, the same as adding an AI provider or picking a workflow's
 * trigger: first the kinds of connection, each with its mark, a line on
 * what it is and a chevron, and under them how dimmi keeps actions safe;
 * then the picked kind's details. The steps slide into one another the
 * way the person is heading and the dialog never changes size. A start
 * from the empty page opens straight on the details of its kind, with
 * the way back to the list. Changing a connection opens the same dialog
 * on its details alone.
 */
import {
  RiArrowLeftSLine,
  RiArrowRightSLine,
  RiKey2Line,
  RiShieldCheckLine,
  RiShieldUserLine,
  RiUserFollowLine,
} from "@remixicon/react";
import { useState } from "react";

import { LinkButton } from "@/components/base/buttons/link-button";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";
import { Swap } from "@/features/inbox/motion";

import { type ActionConnection, CONNECTION_KINDS, type ConnectionKind } from "./api";
import { ConnectionForm } from "./connection-form";
import { kindInfo } from "./connection-values";
import { ConnectionMark } from "./parts";

const safety = [
  { icon: RiShieldUserLine, text: "Customer details come only from a verified sign-in, never from the chat." },
  { icon: RiUserFollowLine, text: "Anything that changes data asks the customer, and if you want, a teammate." },
  { icon: RiKey2Line, text: "Secrets are encrypted and never reach the model, logs or traces." },
];

function Choice({ kind, onPick }: { kind: ConnectionKind; onPick: () => void }) {
  const info = kindInfo[kind];

  return (
    <button
      type="button"
      onClick={onPick}
      className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <ConnectionMark kind={kind} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-regular text-text-primary">{info.name}</span>
        <span className="text-body-2-regular text-text-secondary">{info.blurb}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

function KindList({ onPick }: { onPick: (kind: ConnectionKind) => void }) {
  return (
    <>
      <ModalHeader
        title="Add a connection"
        description="Where dimmi's actions call. One connection serves as many actions as you need."
      />
      <ModalBody>
        <div className="flex flex-col gap-5 pb-2">
          <section aria-label="Connect" className="flex flex-col gap-2">
            <div className="flex flex-col">
              <h3 className="text-body-2-medium text-text-primary">Your systems</h3>
              <p className="text-body-2-regular text-text-secondary">
                Billing, accounts, orders: wherever answers live.
              </p>
            </div>
            <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
              {CONNECTION_KINDS.map((kind) => (
                <Choice key={kind} kind={kind} onPick={() => onPick(kind)} />
              ))}
            </div>
          </section>
          <section aria-label="How actions stay safe" className="flex flex-col gap-2">
            <div className="flex flex-col">
              <h3 className="flex items-center gap-1.5 text-body-2-medium text-text-primary">
                <RiShieldCheckLine aria-hidden className="size-4 text-foreground-icon-secondary" />
                How actions stay safe
              </h3>
            </div>
            <ul className="flex flex-col gap-2.5 rounded-2xl bg-background-secondary-default p-3">
              {safety.map((item) => (
                <li key={item.text} className="flex items-start gap-2.5 text-body-2-regular text-text-secondary">
                  <item.icon aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-tertiary" />
                  {item.text}
                </li>
              ))}
            </ul>
          </section>
        </div>
      </ModalBody>
    </>
  );
}

function Details({
  workspace,
  kind,
  connection,
  onBack,
  onClose,
}: {
  workspace: string;
  kind: ConnectionKind;
  connection?: ActionConnection;
  onBack?: () => void;
  onClose: () => void;
}) {
  const info = kindInfo[kind];

  return (
    <>
      <ModalHeader
        title={connection ? `Change ${connection.name}` : `Connect ${kind === "mcp" ? "an" : "a"} ${info.name}`}
        description={
          kind === "mcp"
            ? "dimmi connects, lists the server's tools and turns each into a draft action."
            : "dimmi tests the connection when it saves."
        }
      >
        {onBack ? (
          <LinkButton
            variant="secondary"
            size="small"
            leadingIcon={RiArrowLeftSLine}
            onClick={onBack}
            className="mt-3 -ms-1 self-start rtl:[&_svg]:-scale-x-100"
          >
            All connections
          </LinkButton>
        ) : null}
      </ModalHeader>
      <ConnectionForm workspace={workspace} kind={kind} connection={connection} onCancel={onClose} onDone={onClose} />
    </>
  );
}

/*
 * AddConnectionModal is the whole flow. start opens on a kind's details;
 * connection opens on an existing connection's details to change them.
 */
export function AddConnectionModal({
  workspace,
  isOpen,
  start = null,
  connection,
  onClose,
}: {
  workspace: string;
  isOpen: boolean;
  start?: ConnectionKind | null;
  connection?: ActionConnection;
  onClose: () => void;
}) {
  const [picked, setPicked] = useState<ConnectionKind | null>(start);
  const [wasOpen, setWasOpen] = useState(isOpen);

  if (isOpen !== wasOpen) {
    setWasOpen(isOpen);
    if (isOpen) {
      setPicked(connection?.kind ?? start);
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
      <Swap view={picked ?? "list"} forward={picked !== null} className="flex min-h-0 flex-1 flex-col gap-4">
        {picked ? (
          <Details
            workspace={workspace}
            kind={picked}
            connection={connection}
            onBack={connection ? undefined : () => setPicked(null)}
            onClose={onClose}
          />
        ) : (
          <KindList onPick={setPicked} />
        )}
      </Swap>
    </Modal>
  );
}
