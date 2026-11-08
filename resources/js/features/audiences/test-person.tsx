/*
 * Test a person: pick someone, optionally say what page, language,
 * device and country their browser would report, and see every
 * condition with the value they actually have and whether it held.
 * Nothing is written; the test never acts as them.
 */
import { useInfiniteQuery, useMutation } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Chip, type ChipColor } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import {
  type AudienceEvaluation,
  type AudienceRules,
  type BrowserContext,
  evaluateAudience,
  type LeafResult,
} from "@/lib/api/audiences";
import { peopleQuery } from "@/lib/api/customers";

import { personName } from "../customers/names";
import { fieldName, opNames } from "./rules";

const verdicts: Record<AudienceEvaluation["result"], [string, ChipColor]> = {
  match: ["Matches", "lime"],
  no_match: ["Does not match", "rose"],
  depends_on_page: ["Depends on the page", "yellow"],
};

const devices: [NonNullable<BrowserContext["device"]>, string][] = [
  ["desktop", "Desktop"],
  ["tablet", "Tablet"],
  ["mobile", "Mobile"],
];

const marks: Record<LeafResult["result"], string> = { true: "Yes", false: "No", unknown: "Unknown" };

function actualText(actual: unknown): string {
  if (actual === null || actual === undefined) {
    return "no value";
  }
  if (Array.isArray(actual)) {
    return actual.length === 0 ? "none" : actual.join(", ");
  }

  return String(actual);
}

function Leaves({ leaves, names }: { leaves: LeafResult[]; names: Record<string, string> }) {
  return (
    <ul className="flex flex-col gap-2">
      {leaves.map((leaf) => (
        <li key={leaf.path} className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 flex-col">
            <span className="text-body-2-regular text-text-primary">
              {fieldName(leaf.field)} {opNames[leaf.op] ?? leaf.op}
            </span>
            <span className="truncate text-caption-1-regular text-text-tertiary">
              Has {actualText(Array.isArray(leaf.actual) ? leaf.actual.map((a) => names[a] ?? a) : leaf.actual)}
              {leaf.trust === "claimed" ? " · could be claimed by the browser" : ""}
            </span>
          </div>
          <span className="shrink-0 text-body-2-medium text-text-secondary">{marks[leaf.result]}</span>
        </li>
      ))}
    </ul>
  );
}

export function TestPerson({
  workspace,
  rules,
  names,
}: {
  workspace: string;
  rules: AudienceRules;
  names: Record<string, string>;
}) {
  const [search, setSearch] = useState("");
  const [contact, setContact] = useState<string | null>(null);
  const [page, setPage] = useState("");
  const [device, setDevice] = useState<BrowserContext["device"] | "">("");
  const q = useDeferredValue(search.trim());
  const people = useInfiniteQuery({ ...peopleQuery(workspace, { q: q || undefined }), enabled: workspace !== "" });
  const found = people.data?.pages.flatMap((p) => p.items).slice(0, 20) ?? [];
  const test = useMutation({
    mutationFn: (id: string) =>
      evaluateAudience(workspace, rules, id, { page_url: page.trim() || undefined, device: device || undefined }),
  });
  const verdict = test.data ? verdicts[test.data.result] : null;

  return (
    <div className="flex flex-col gap-3">
      <Input label="Person" size="small" placeholder="Search by name or email" value={search} onChange={setSearch} />
      <Select
        aria-label="Person to test"
        size="sm"
        placeholder={found.length === 0 ? "No one found" : "Pick a person"}
        selectedKey={contact}
        onSelectionChange={(key) => setContact(key === null ? null : String(key))}
      >
        {found.map((p) => (
          <SelectItem key={p.id} id={p.id} textValue={personName(p)}>
            {personName(p)}
          </SelectItem>
        ))}
      </Select>
      <Input
        label="On page"
        size="small"
        inputDir="ltr"
        placeholder="https://app.example.com/billing"
        value={page}
        onChange={setPage}
      />
      <Select
        label="Device"
        size="sm"
        placeholder="Any device"
        selectedKey={device || null}
        onSelectionChange={(key) => setDevice(key === null ? "" : (String(key) as BrowserContext["device"]))}
      >
        {devices.map(([id, name]) => (
          <SelectItem key={id} id={id}>
            {name}
          </SelectItem>
        ))}
      </Select>
      <div>
        <Button
          size="small"
          variant="secondary"
          disabled={!contact}
          loading={test.isPending}
          onClick={() => contact && test.mutate(contact)}
        >
          Test
        </Button>
      </div>
      {test.error ? <p className="text-body-2-regular text-text-error-primary">{test.error.message}</p> : null}
      {test.data && verdict ? (
        <div className="flex flex-col gap-3">
          <div className="flex items-center gap-2">
            <span className="truncate text-body-medium text-text-primary">{personName(test.data.contact)}</span>
            <Chip color={verdict[1]}>{verdict[0]}</Chip>
          </div>
          <Leaves leaves={test.data.leaves} names={names} />
        </div>
      ) : null}
    </div>
  );
}
