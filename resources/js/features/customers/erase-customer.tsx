/*
 * Erasing a customer, from the actions of their row. They disappear at once and are
 * wiped when the grace period ends, unless someone cancels from the
 * erasures list first. Skipping the grace period wipes them as soon as
 * the server gets to it, and asks for the password when the last proof
 * of who is at the keyboard is too old.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { AlertDialog } from "@/components/base/modal/modal";
import { useRecentAuth } from "@/features/settings/recent-auth";
import { failureText } from "@/features/settings/teams";
import { useAccount } from "@/features/settings/use-account";
import { type Contact, customerKeys, eraseContact } from "@/lib/api/customers";

import { personName, useWorkspace } from "./names";

/*
 * EraseDialog asks before erasing the customer and closes once they are
 * gone or the member changes their mind.
 */
export function EraseDialog({ contact, onClose }: { contact: Contact; onClose: () => void }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const navigate = useNavigate();
  const me = useAccount();
  const { run, prompt } = useRecentAuth(me.hasPassword);
  const [now, setNow] = useState(false);
  const [failure, setFailure] = useState<Error | null>(null);
  const name = personName(contact);

  const erase = async () => {
    setFailure(null);
    try {
      const done = await run(() => eraseContact(workspace, contact.id, now));
      if (!done) {
        return;
      }
      onClose();
      client.removeQueries({ queryKey: customerKeys.contact(workspace, contact.id) });
      await navigate({ to: "/d/customers/erasures" });
      void client.invalidateQueries({ queryKey: ["workspace", workspace] });
    } catch (error) {
      setFailure(error instanceof Error ? error : new Error(String(error)));
      throw error;
    }
  };

  return (
    <>
      <AlertDialog
        isOpen
        onOpenChange={(open) => (open ? undefined : onClose())}
        title={`Erase ${name}?`}
        description="They disappear from the inbox now. Their messages, files and details are wiped after 7 days; until then you can cancel."
        confirmLabel="Erase customer"
        onConfirm={erase}
      >
        <div className="flex flex-col gap-2">
          <Checkbox size="sm" isSelected={now} onChange={setNow}>
            Wipe now instead of in 7 days
          </Checkbox>
          {failure ? <p className="text-body-2-regular text-text-error-primary">{failureText(failure)}</p> : null}
        </div>
      </AlertDialog>
      {prompt}
    </>
  );
}
