/*
 * The first step of a new workflow: picking what starts it. One narrow
 * dialog lists every trigger, group after group, each group under its
 * name and what it covers, and each trigger with its icon in the
 * group's colour, a short example of what to do with it, and a chevron,
 * as picking one moves on to the new workflow. Each fits one line on
 * wider screens and wraps on phones rather than hiding words. Searching keeps
 * only the triggers that match, still in their groups, and Enter picks
 * the first one, while the dialog keeps its height so it never jumps as
 * the list narrows. Picking a trigger opens the builder on an empty
 * workflow with it, which it then keeps.
 */
import { RiArrowRightSLine, RiSearchLine } from "@remixicon/react";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";

import { KindMark } from "./parts";
import { type TriggerKind, tintOf, triggerGroups, triggers, triggersIn } from "./triggers";

function Choice({ kind, onPick }: { kind: TriggerKind; onPick: (kind: TriggerKind) => void }) {
  const trigger = triggers[kind];

  return (
    <button
      type="button"
      onClick={() => onPick(kind)}
      className="flex w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 py-2 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <KindMark icon={trigger.icon} tint={tintOf(kind)} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-regular text-text-primary">{trigger.name}</span>
        <span className="text-body-2-regular text-text-secondary">{trigger.detail}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

function Groups({ query, onPick }: { query: string; onPick: (kind: TriggerKind) => void }) {
  const found = triggerGroups
    .map((group) => ({ ...group, kinds: triggersIn(group.id, query) }))
    .filter((group) => group.kinds.length > 0);

  if (found.length === 0) {
    return <p className="py-8 text-center text-body-regular text-text-secondary">No trigger matches “{query}”</p>;
  }

  return (
    <div className="flex flex-col gap-5 pb-2">
      {found.map((group) => (
        <section key={group.id} aria-label={group.name} className="flex flex-col gap-2">
          <div className="flex flex-col">
            <h3 className="text-body-2-medium text-text-primary">{group.name}</h3>
            <p className="text-body-2-regular text-text-secondary">{group.detail}</p>
          </div>
          <div className="flex flex-col gap-0.5 rounded-2xl bg-background-secondary-default p-1">
            {group.kinds.map((kind) => (
              <Choice key={kind} kind={kind} onPick={onPick} />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

export function TriggerPicker({
  isOpen,
  onOpenChange,
  onPick,
}: {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  onPick: (kind: TriggerKind) => void;
}) {
  const [query, setQuery] = useState("");
  const first = triggerGroups.flatMap((group) => triggersIn(group.id, query.trim()))[0];

  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange} size="md" className="h-[min(640px,calc(100dvh-3rem))]">
      <ModalHeader
        title="What starts this workflow?"
        description="Pick what kicks it off. You can't change this later."
      />
      <div className="shrink-0 px-6">
        <Input
          aria-label="Search triggers"
          placeholder="Search triggers"
          leadingIcon={RiSearchLine}
          value={query}
          onChange={setQuery}
          autoFocus
          onKeyDown={(event) => {
            if (event.key === "Enter" && first) {
              onPick(first);
            }
          }}
        />
      </div>
      <ModalBody>
        <Groups query={query.trim()} onPick={onPick} />
      </ModalBody>
    </Modal>
  );
}
