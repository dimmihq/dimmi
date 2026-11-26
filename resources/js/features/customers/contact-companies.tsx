/*
 * The companies a customer belongs to, in the edit modal. Each is a row
 * with its initial and name, the default marked; a row's menu makes it
 * the default or takes it off. Add company searches the workspace's
 * companies. Nothing is saved until Save, like the rest of the form.
 * Taking off the default makes the next company the default, as the
 * server would.
 */
import { RiCheckboxCircleLine, RiDeleteBin6Line } from "@remixicon/react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useState } from "react";

import { SearchComboBox } from "@/components/application/search-combobox/search-combobox";
import { Avatar } from "@/components/base/avatar/avatar";
import { Badge } from "@/components/base/badges/badge";
import { avatarColor } from "@/features/shell/avatar-color";
import { RowMenu } from "@/features/shell/row-menu";
import { type CompanySummary, companiesQuery } from "@/lib/api/customers";

import type { EditState } from "./contact-edit-state";
import { useWorkspace } from "./names";

type Patch = (change: Partial<EditState>) => void;

function CompanyPicker({ taken, onPick }: { taken: string[]; onPick: (company: CompanySummary) => void }) {
  const workspace = useWorkspace();
  const [words, setWords] = useState("");
  const [round, setRound] = useState(0);
  const list = useInfiniteQuery({ ...companiesQuery(workspace, words), enabled: workspace !== "" && words !== "" });
  const items = (list.data?.pages.flatMap((page) => page.items) ?? []).filter((c) => !taken.includes(c.id));

  return (
    <SearchComboBox
      key={round}
      label="Add company"
      placeholder="Search companies"
      items={items}
      searching={list.isFetching}
      keyOf={(company) => company.id}
      textOf={(company) => company.name}
      onSearch={setWords}
      onPick={(company) => {
        if (company) {
          onPick({ id: company.id, object: "company", name: company.name, domain: company.domain });
          setWords("");
          setRound((n) => n + 1);
        }
      }}
      empty="No company matches."
    >
      {(company) => (
        <>
          <Avatar
            size="xs"
            color={avatarColor(company.name)}
            initials={company.name.charAt(0).toUpperCase()}
            className="rounded-md"
          />
          <span className="shrink-0 truncate text-text-primary">{company.name}</span>
          {company.domain ? (
            <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-tertiary">{company.domain}</span>
          ) : null}
        </>
      )}
    </SearchComboBox>
  );
}

function CompanyRow({
  company,
  isDefault,
  onDefault,
  onRemove,
}: {
  company: CompanySummary;
  isDefault: boolean;
  onDefault: () => void;
  onRemove: () => void;
}) {
  const actions = [
    ...(isDefault ? [] : [{ kind: "default" as const, label: "Make default", icon: RiCheckboxCircleLine }]),
    { kind: "remove" as const, label: "Remove", icon: RiDeleteBin6Line, danger: true },
  ];

  return (
    <li className="flex min-h-11 items-center gap-2.5 py-1.5">
      <Avatar
        size="sm"
        color={avatarColor(company.name)}
        initials={company.name.charAt(0).toUpperCase()}
        className="rounded-md"
      />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-medium text-text-primary">{company.name}</span>
        {company.domain ? (
          <span className="truncate text-body-2-regular text-text-tertiary">{company.domain}</span>
        ) : null}
      </span>
      {isDefault ? <Badge className="shrink-0">Default</Badge> : null}
      <RowMenu
        label={company.name}
        actions={actions}
        quiet
        onPick={(kind) => (kind === "default" ? onDefault() : onRemove())}
      />
    </li>
  );
}

export function ContactCompanies({ state, set }: { state: EditState; set: Patch }) {
  const remove = (id: string) => {
    const companies = state.companies.filter((c) => c.id !== id);
    const defaultCompany = state.defaultCompany === id ? (companies[0]?.id ?? null) : state.defaultCompany;
    set({ companies, defaultCompany });
  };
  const add = (company: CompanySummary) =>
    set({ companies: [...state.companies, company], defaultCompany: state.defaultCompany ?? company.id });

  return (
    <section aria-label="Companies" className="flex flex-col gap-3">
      <h3 className="text-body-medium text-text-primary">Companies</h3>
      {state.companies.length > 0 ? (
        <ul className="flex flex-col divide-y divide-separator-border">
          {state.companies.map((company) => (
            <CompanyRow
              key={company.id}
              company={company}
              isDefault={company.id === state.defaultCompany}
              onDefault={() => set({ defaultCompany: company.id })}
              onRemove={() => remove(company.id)}
            />
          ))}
        </ul>
      ) : (
        <p className="text-body-2-regular text-text-tertiary">Not in any company yet.</p>
      )}
      <CompanyPicker taken={state.companies.map((c) => c.id)} onPick={add} />
    </section>
  );
}
