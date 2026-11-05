/*
 * The company the customer works at, with what the product told dimmi
 * about it. The name opens the company's page.
 */
import { Link } from "@tanstack/react-router";

import type { ConversationContext } from "@/lib/api/customers";

import { show } from "./format";
import { Copyable, Nothing, Row } from "./rows";

export function CompanySection({ context }: { context: ConversationContext }) {
  const company = context.company;
  if (!company) {
    return <Nothing>Not linked to a company.</Nothing>;
  }

  return (
    <dl className="flex flex-col">
      <Row label="Name">
        <Link
          to="/d/customers/companies/$company"
          params={{ company: company.id }}
          className="truncate rounded-md outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          {company.name}
        </Link>
      </Row>
      {company.domain ? (
        <Row label="Domain">
          <Copyable value={company.domain} label="domain" />
        </Row>
      ) : null}
      {Object.entries(company.custom_attributes).map(([key, value]) => (
        <Row key={key} label={<span className="truncate">{key}</span>}>
          <Copyable value={show(value)} label={key} />
        </Row>
      ))}
    </dl>
  );
}
