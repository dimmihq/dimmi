/*
 * The company the customer works at, with what the product told dimmi
 * about it. The name opens the company's page. A customer with no
 * company shows no section.
 */
import { Link } from "@tanstack/react-router";

import type { ConversationContext } from "@/lib/api/customers";

import { show } from "./format";
import { Copyable, Row, Rows, Section } from "./rows";

export function CompanySection({ context }: { context: ConversationContext }) {
  const company = context.company;
  if (!company) {
    return null;
  }

  return (
    <Section title="Company">
      <Rows
        key={company.id}
        rows={[
          <Row key="name" label="Name">
            <Link
              to="/d/customers/companies/$company"
              params={{ company: company.id }}
              className="truncate rounded-md outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              {company.name}
            </Link>
          </Row>,
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
