/*
 * The company the conversation is about, with what the product told
 * dimmi about it; the name opens the company's page. A customer who
 * belongs to several companies gets a picker in its place to switch the
 * conversation to another of them, which saves at once like the case's
 * other fields, with their other companies listed under it, the default
 * marked, and a link to the chosen company's page. A
 * customer with no company shows no section.
 */
import { Link } from "@tanstack/react-router";

import { Badge } from "@/components/base/badges/badge";
import { Select, SelectItem } from "@/components/base/select/select";
import type { ConversationContext } from "@/lib/api/customers";

import { show } from "./format";
import { Copyable, Row, Rows, Section } from "./rows";

const link =
  "truncate rounded-md outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring";

function CompanyLink({ id, name }: { id: string; name: string }) {
  return (
    <Link to="/d/customers/companies/$company" params={{ company: id }} className={link}>
      {name}
    </Link>
  );
}

export function CompanySection({
  context,
  onSwitch,
}: {
  context: ConversationContext;
  onSwitch?: (company: string) => void;
}) {
  const company = context.company;
  if (!company) {
    return null;
  }

  const all = context.contact.companies ?? [];
  const others = all.filter((c) => c.id !== company.id);
  const several = all.length > 1;

  return (
    <Section title={several ? "Companies" : "Company"}>
      <Rows
        key={company.id}
        rows={[
          several && onSwitch ? (
            <Row key="about" label="About">
              <Select
                aria-label="Conversation is about"
                size="sm"
                variant="quiet"
                selectedKey={company.id}
                onSelectionChange={(key) => (key && key !== company.id ? onSwitch(String(key)) : undefined)}
              >
                {all.map((c) => (
                  <SelectItem key={c.id} id={c.id} textValue={c.name}>
                    {c.name}
                  </SelectItem>
                ))}
              </Select>
            </Row>
          ) : (
            <Row key="name" label="Name">
              <CompanyLink id={company.id} name={company.name} />
            </Row>
          ),
          others.length > 0 ? (
            <Row key="others" label="Also at">
              <span className="flex min-w-0 flex-col items-start gap-1">
                {others.map((c) => (
                  <span key={c.id} className="flex min-w-0 max-w-full items-center gap-1.5">
                    <CompanyLink id={c.id} name={c.name} />
                    {c.is_default ? <Badge className="shrink-0">Default</Badge> : null}
                  </span>
                ))}
              </span>
            </Row>
          ) : null,
          several ? (
            <Row key="page" label="Page">
              <CompanyLink id={company.id} name={`Open ${company.name}`} />
            </Row>
          ) : null,
          company.domain ? (
            <Row key="domain" label="Domain">
              <Copyable value={company.domain} label="domain" />
            </Row>
          ) : null,
          ...Object.entries(company.custom_attributes).map(([key, value]) => (
            <Row key={`attribute:${key}`} label={<span className="truncate">{key}</span>}>
              <Copyable value={show(value)} label={key} />
            </Row>
          )),
        ].filter(Boolean)}
      />
    </Section>
  );
}
