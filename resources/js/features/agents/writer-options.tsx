/*
 * The rows of the guidance writer's menus: a block for the slash, and a
 * chip for @, grouped the way a procedure step groups them.
 */
import {
  RiArticleLine,
  RiFlashlightLine,
  RiListOrdered2,
  RiPriceTag3Line,
  RiTeamLine,
  RiTicketLine,
  RiUserLine,
} from "@remixicon/react";

import type { SlashCommand } from "@/features/help-center/writer/slash-commands";
import type { BindingKind } from "@/lib/api/procedures";

import type { ChipOption } from "../automation/procedure-writer/procedure-suggestions";

const icons: Record<BindingKind, typeof RiFlashlightLine> = {
  action: RiFlashlightLine,
  attribute: RiUserLine,
  article: RiArticleLine,
  handoff: RiTeamLine,
  tag: RiPriceTag3Line,
  ticket_type: RiTicketLine,
  sub_procedure: RiListOrdered2,
};

export function SlashOption({ command }: { command: SlashCommand }) {
  const Icon = command.icon;

  return (
    <>
      <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border-button-default bg-background-primary-default">
        <Icon aria-hidden className="size-4 text-foreground-icon-secondary" />
      </span>
      <span className="flex min-w-0 flex-col">
        <span className="truncate text-body-medium text-text-primary">{command.label}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">{command.hint}</span>
      </span>
    </>
  );
}

export function ChipOptionRow({ option }: { option: ChipOption }) {
  const Icon = icons[option.kind];

  return (
    <span className="flex min-w-0 flex-1 flex-col">
      {option.first ? (
        <span className="pb-1 text-caption-1-semibold text-text-tertiary uppercase">{option.group}</span>
      ) : null}
      <span className="flex min-w-0 items-center gap-2">
        <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-body-medium text-text-primary">{option.label}</span>
          <span className="truncate text-body-2-regular text-text-tertiary">{option.hint}</span>
        </span>
      </span>
    </span>
  );
}
