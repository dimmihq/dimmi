/*
 * Finding the customer a teammate speaks for, by name or email, among
 * the workspace's people.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Input } from "@/components/base/input/input";
import { peoplePageQuery } from "@/lib/api/customers";
import { cx } from "@/utils/cx";

export interface Picked {
  id: string;
  label: string;
}

export function CustomerPicker({
  workspace,
  picked,
  onPick,
}: {
  workspace: string;
  picked: Picked | null;
  onPick: (picked: Picked | null) => void;
}) {
  const [q, setQ] = useState("");
  const people = useQuery({
    ...peoplePageQuery(workspace, { q: q || undefined }, 1),
    enabled: workspace !== "" && q !== "",
  });

  return (
    <div className="flex flex-col gap-2">
      <Input label="Customer" placeholder="Search by name or email" value={q} onChange={setQ} />
      {picked ? (
        <p className="flex items-center justify-between rounded-xl bg-background-secondary-default px-3 py-2 text-body-2-regular">
          <span className="truncate">{picked.label}</span>
          <button
            type="button"
            className="text-body-2-medium text-text-secondary hover:text-text-primary"
            onClick={() => onPick(null)}
          >
            Clear
          </button>
        </p>
      ) : null}
      {q && !picked ? (
        <ul
          aria-label="Matching customers"
          className="flex max-h-48 flex-col overflow-auto rounded-xl border border-border-button-default"
        >
          {(people.data?.items ?? []).map((c) => {
            const label = c.name ? `${c.name}${c.email ? ` · ${c.email}` : ""}` : (c.email ?? c.id);
            return (
              <li key={c.id}>
                <button
                  type="button"
                  onClick={() => {
                    onPick({ id: c.id, label });
                    setQ("");
                  }}
                  className={cx(
                    "w-full truncate px-3 py-2 text-start text-body-2-regular hover:bg-background-secondary-hover",
                  )}
                >
                  {label}
                </button>
              </li>
            );
          })}
          {people.data && people.data.items.length === 0 ? (
            <li className="px-3 py-2 text-body-2-regular text-text-tertiary">Nobody matches.</li>
          ) : null}
        </ul>
      ) : null}
    </div>
  );
}
