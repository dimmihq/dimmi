/*
 * Which of the customer's companies a ticket is about. It shows only
 * when the ticket's customer belongs to several companies, as a picker
 * that saves at once like the ticket's other properties.
 */
import { useQuery } from "@tanstack/react-query";

import { Select, SelectItem } from "@/components/base/select/select";
import { Row } from "@/features/inbox/customer/rows";
import { contactQuery } from "@/lib/api/customers";
import type { Ticket } from "@/lib/api/tickets";

import type { TicketSave } from "./use-ticket-save";

export function TicketCompanyRow({ workspace, ticket, save }: { workspace: string; ticket: Ticket; save: TicketSave }) {
  const contact = useQuery({
    ...contactQuery(workspace, ticket.contact_id ?? ""),
    enabled: workspace !== "" && ticket.contact_id !== null,
  });
  const companies = contact.data?.companies ?? [];
  if (companies.length < 2) {
    return null;
  }

  return (
    <Row label="Company">
      <Select
        aria-label="Company"
        size="sm"
        variant="quiet"
        placeholder="No company"
        selectedKey={ticket.company_id}
        isDisabled={save.busy}
        onSelectionChange={(key) => {
          if (key !== null && key !== ticket.company_id) {
            void save.save({ company_id: String(key) });
          }
        }}
      >
        {companies.map((c) => (
          <SelectItem key={c.id} id={c.id} textValue={c.name}>
            {c.name}
          </SelectItem>
        ))}
      </Select>
    </Row>
  );
}
