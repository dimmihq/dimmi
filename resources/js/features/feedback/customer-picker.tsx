/*
 * Finding the customer a teammate speaks for, by name or email, among
 * the workspace's people.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { SearchComboBox } from "@/components/application/search-combobox/search-combobox";
import { Avatar } from "@/components/base/avatar/avatar";
import { avatarColor } from "@/features/shell/avatar-color";
import { type Contact, peoplePageQuery } from "@/lib/api/customers";

export interface Picked {
  id: string;
  label: string;
}

function nameOf(person: Contact): string {
  return person.name || person.email || "Unnamed";
}

export function CustomerPicker({
  workspace,
  onPick,
}: {
  workspace: string;
  picked: Picked | null;
  onPick: (picked: Picked | null) => void;
}) {
  const [words, setWords] = useState("");
  const people = useQuery({
    ...peoplePageQuery(workspace, { q: words || undefined }, 1),
    enabled: workspace !== "" && words !== "",
  });

  return (
    <SearchComboBox
      label="Customer"
      placeholder="Search by name or email"
      items={people.data?.items ?? []}
      searching={people.isFetching}
      keyOf={(person) => person.id}
      textOf={nameOf}
      onSearch={setWords}
      onPick={(person) => onPick(person ? { id: person.id, label: nameOf(person) } : null)}
      empty="Nobody matches."
    >
      {(person) => (
        <>
          <Avatar size="xs" color={avatarColor(nameOf(person))} initials={nameOf(person).charAt(0)} />
          <span className="shrink-0 truncate text-text-primary">{nameOf(person)}</span>
          {person.name && person.email ? (
            <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-tertiary">{person.email}</span>
          ) : null}
        </>
      )}
    </SearchComboBox>
  );
}
