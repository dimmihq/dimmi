/*
 * How the people table draws a person, their company, tags and when they
 * were last seen.
 */
import type { ReactNode } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { avatarColor } from "@/features/shell/avatar-color";
import type { CompanySummary, Contact } from "@/lib/api/customers";
import type { Tag } from "@/lib/api/tags";
import { formatDate, formatRelative } from "@/lib/i18n/format";

import { personName } from "./names";

export function Muted({ children, ltr = false }: { children: ReactNode; ltr?: boolean }) {
  return (
    <span dir={ltr ? "ltr" : "auto"} className="truncate whitespace-nowrap text-body-medium text-text-secondary">
      {children}
    </span>
  );
}

export function Person({ person }: { person: Contact }) {
  const name = personName(person);

  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <Avatar
        size="sm"
        src={person.avatar_url ?? undefined}
        color={avatarColor(name)}
        initials={name.charAt(0).toUpperCase()}
      />
      <span className="truncate text-body-medium text-text-primary">{name}</span>
    </span>
  );
}

export function Company({ person }: { person: Contact }) {
  if (!person.company) {
    return <Muted>—</Muted>;
  }

  return <CompanyName company={person.company} />;
}

/*
 * CompanyName draws a company as its coloured initial, square-cornered so
 * it reads apart from a person, and its name.
 */
export function CompanyName({ company }: { company: CompanySummary }) {
  return (
    <span className="flex min-w-0 items-center gap-2.5">
      <Avatar
        size="sm"
        color={avatarColor(company.name)}
        initials={company.name.charAt(0).toUpperCase()}
        className="rounded-md"
      />
      <span className="truncate text-body-medium text-text-primary">{company.name}</span>
    </span>
  );
}

export function Tags({ tags }: { tags: Tag[] }) {
  if (tags.length === 0) {
    return <Muted>—</Muted>;
  }
  const shown = tags.slice(0, 2);

  return (
    <span className="flex min-w-0 items-center gap-1">
      {shown.map((tag) => (
        <TagChip key={tag.id} tag={tag} size="small" />
      ))}
      {tags.length > shown.length ? (
        <Chip variant="caption" color="soft">
          +{tags.length - shown.length}
        </Chip>
      ) : null}
    </span>
  );
}

export function Seen({ at }: { at: string | null }) {
  if (!at) {
    return <Muted>Never</Muted>;
  }

  return (
    <span title={formatDate(at)} className="whitespace-nowrap text-body-medium text-text-primary">
      {formatRelative(at)}
    </span>
  );
}
