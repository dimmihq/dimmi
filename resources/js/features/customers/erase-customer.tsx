/*
 * Erasing a customer from their page. They disappear at once and are
 * wiped when the grace period ends, unless someone cancels from the
 * erasures list first. Skipping the grace period wipes them as soon as
 * the server gets to it, and asks for the password when the last proof
 * of who is at the keyboard is too old.
 */
import { RiDeleteBin6Line } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { AlertDialog } from "@/components/base/modal/modal";
import { useRecentAuth } from "@/features/settings/recent-auth";
import { failureText } from "@/features/settings/teams";
import { useAccount } from "@/features/settings/use-account";
import { type Contact, customerKeys, eraseContact } from "@/lib/api/customers";

import { personName, useWorkspace } from "./names";

export function EraseCustomer({ contact }: { contact: Contact }) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const navigate = useNavigate();
  const me = useAccount();
  const { run, prompt } = useRecentAuth(me.hasPassword);
  const [open, setOpen] = useState(false);
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
      setOpen(false);
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
      <Button variant="danger-secondary" size="small" leadingIcon={RiDeleteBin6Line} onClick={() => setOpen(true)}>
        Erase
      </Button>
      <AlertDialog
        isOpen={open}
        onOpenChange={setOpen}
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
