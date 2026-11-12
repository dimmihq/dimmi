/*
 * The add-step menu at the end of every path in the builder: a small
 * plus that opens BoardUI's menu panel with the step kinds offered,
 * grouped as messages, logic, dimmi, team actions and tickets; a group with
 * nothing offered is left out.
 */
import { RiAddLine } from "@remixicon/react";
import { Button as AriaButton, Dialog, DialogTrigger, Popover } from "react-aria-components";

import { MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import { cx } from "@/utils/cx";

import { KindMark } from "./parts";
import { stepLook } from "./step-kinds";
import type { StepKind } from "./workflows";

const groups: [string, StepKind[]][] = [
  ["Messages", ["message", "buttons", "collect", "reply_time"]],
  ["Logic", ["branch", "classify", "wait", "run_workflow"]],
  ["dimmi", ["dimmi"]],
  [
    "Team",
    [
      "assign",
      "unassign",
      "tag",
      "untag",
      "priority",
      "note",
      "update_contact",
      "snooze",
      "close",
      "apply_sla",
      "http_request",
    ],
  ],
  ["Tickets", ["create_ticket", "set_ticket_state", "set_ticket_fields", "escalate", "add_to_tracker"]],
];

export function AddStep({ kinds, onAdd }: { kinds: StepKind[]; onAdd: (kind: StepKind) => void }) {
  return (
    <DialogTrigger>
      <AriaButton
        aria-label="Add a step"
        className={cx(
          "flex size-7 cursor-pointer items-center justify-center rounded-full border border-border-button-default",
          "bg-background-primary-default text-foreground-icon-secondary shadow-xs outline-none transition-colors duration-150",
          "hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        )}
      >
        <RiAddLine aria-hidden className="size-4" />
      </AriaButton>
      <Popover placement="bottom" offset={8} className={cx(MENU_POPOVER_SURFACE, "w-[260px] p-2")}>
        <Dialog aria-label="Add a step" className="flex flex-col gap-2 outline-none">
          {({ close }) =>
            groups
              .map(([title, group]): [string, StepKind[]] => [title, group.filter((kind) => kinds.includes(kind))])
              .filter(([, group]) => group.length > 0)
              .map(([title, group]) => (
                <div key={title} className="flex flex-col gap-1">
                  <span className="ps-2 pt-1 text-body-2-medium text-text-secondary">{title}</span>
                  {group.map((kind) => {
                    const look = stepLook[kind];

                    return (
                      <button
                        key={kind}
                        type="button"
                        onClick={() => {
                          onAdd(kind);
                          close();
                        }}
                        className="flex w-full cursor-pointer items-center gap-2 rounded-2lg px-2 py-1.5 text-start text-body-medium text-text-primary outline-none transition-colors hover:bg-background-primary-hover focus-visible:bg-background-primary-hover"
                      >
                        <KindMark icon={look.icon} tint={look.tint} size="sm" />
                        {look.name}
                      </button>
                    );
                  })}
                </div>
              ))
          }
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
