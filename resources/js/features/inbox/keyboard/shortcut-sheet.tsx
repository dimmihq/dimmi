/*
 * The shortcut sheet, opened with ? or from the command menu: every move
 * that has a key, grouped as the registry groups them, with a link to
 * the practice page.
 */
import { Link } from "@tanstack/react-router";

import { Kbd } from "@/components/base/kbd/kbd";
import { Modal, ModalBody, ModalHeader } from "@/components/base/modal/modal";

import { allActions } from "../actions/all-actions";
import type { ActionGroup } from "../actions/registry";

const groups: ActionGroup[] = ["Conversation", "Navigation", "Help"];

export function ShortcutSheet({ isOpen, onOpenChange }: { isOpen: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Modal isOpen={isOpen} onOpenChange={onOpenChange} size="lg">
      <ModalHeader title="Keyboard shortcuts" description="Work the inbox without the mouse." />
      <ModalBody className="flex flex-col gap-5">
        {groups.map((group) => (
          <section key={group} aria-label={group} className="flex flex-col gap-1">
            <h3 className="text-caption-1-medium text-text-tertiary">{group}</h3>
            <dl className="flex flex-col">
              {allActions
                .filter((a) => a.group === group && a.keys)
                .map((a) => (
                  <div key={a.id} className="flex min-h-8 items-center justify-between gap-3">
                    <dt className="text-body-2-regular text-text-primary">{a.label}</dt>
                    <dd className="flex gap-1">
                      {(a.keys ?? []).map((key) => (
                        <Kbd key={key}>{key}</Kbd>
                      ))}
                    </dd>
                  </div>
                ))}
            </dl>
          </section>
        ))}
        <Link
          to="/d/shortcuts"
          onClick={() => onOpenChange(false)}
          className="self-start text-body-2-medium text-text-primary underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          Practise the shortcuts
        </Link>
      </ModalBody>
    </Modal>
  );
}
