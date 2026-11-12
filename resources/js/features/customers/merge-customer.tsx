/*
 * Merging another record of the same person into this customer. The
 * dialog suggests the people who look alike, with why, and finds anyone
 * else by name, email or phone. Confirming moves the other's
 * conversations, identities, tags and activity here; this customer's
 * details win and the other only fills what is missing. The other record
 * then points here, so links to it still open this customer. A merge
 * cannot be undone, and two users the product identified under different
 * ids cannot be merged.
 */
import { RiMergeCellsHorizontal, RiSearchLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { type Contact, duplicatesQuery, mergeContact, peoplePageQuery } from "@/lib/api/customers";

import { duplicateReasons } from "./duplicate-reasons";
import { personName, useWorkspace } from "./names";

interface Candidate {
  id: string;
  name: string;
  detail: string;
}

/*
 * useCandidates lists who may be merged in: the suggested duplicates,
 * then the people matching the search, each once and never the customer
 * themselves.
 */
function useCandidates(workspace: string, contact: Contact, search: string): Candidate[] {
  const { data: duplicates = [] } = useQuery(duplicatesQuery(workspace, contact.id));
  const q = search.trim();
  const { data: found } = useQuery({ ...peoplePageQuery(workspace, { q }, 1), enabled: q !== "" });
  const out: Candidate[] = duplicates.map((d) => ({
    id: d.contact.id,
    name: personName(d.contact),
    detail: `Suggested: ${d.reasons.map((r) => duplicateReasons[r] ?? r).join(", ")}`,
  }));
  for (const person of q ? (found?.items ?? []) : []) {
    if (person.id !== contact.id && !out.some((c) => c.id === person.id)) {
      out.push({ id: person.id, name: personName(person), detail: person.email ?? person.phone ?? person.kind });
    }
  }

  return out;
}

export function MergeContactDialog({
  contact,
  preselected,
  onClose,
}: {
  contact: Contact;
  preselected?: string;
  onClose: () => void;
}) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const query = useDeferredValue(search);
  const [source, setSource] = useState<string | null>(preselected ?? null);
  const [failure, setFailure] = useState<string | null>(null);
  const candidates = useCandidates(workspace, contact, query);
  const name = personName(contact);

  const merge = async () => {
    if (!source) {
      return;
    }
    setFailure(null);
    try {
      await mergeContact(workspace, contact.id, source);
    } catch (error) {
      setFailure(error instanceof Error ? error.message : "The customers could not be merged.");
      throw error;
    }
    toast.success(`Merged into ${name}`);
    await client.invalidateQueries({ queryKey: ["workspace", workspace] });
  };

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      isDismissable
      variant="default"
      icon={RiMergeCellsHorizontal}
      size="md"
      title={`Merge into ${name}`}
      description={`Their conversations, identities, tags and activity move to ${name}. ${name}'s details win; the other only fills what is missing. This cannot be undone.`}
      confirmLabel="Merge"
      pendingLabel="Merging…"
      isConfirmDisabled={!source}
      onConfirm={merge}
    >
      <div className="flex flex-col gap-3">
        <Input
          aria-label="Find a customer"
          placeholder="Find by name, email or phone"
          leadingIcon={RiSearchLine}
          value={search}
          onChange={setSearch}
        />
        <div className="flex max-h-72 flex-col overflow-y-auto">
          {candidates.length === 0 ? (
            <p className="text-body-2-regular text-text-secondary">
              {query.trim() ? "Nobody else matches." : "No likely duplicates. Search for the other record."}
            </p>
          ) : (
            <RadioGroup aria-label="Customer to merge in" value={source} onChange={setSource} className="gap-2">
              {candidates.map((c) => (
                <RadioCard key={c.id} value={c.id} title={c.name} description={c.detail} />
              ))}
            </RadioGroup>
          )}
        </div>
        {failure ? <p className="text-body-2-regular text-text-error-primary">{failure}</p> : null}
      </div>
    </AlertDialog>
  );
}
