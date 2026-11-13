/*
 * What the messenger draws for requests: Home's "Get help with" cards,
 * the form of a request type, the filter of open requests, and the strip
 * above a request's thread from submitted to resolved. Words come from the messenger's texts, English when an older
 * config lacks them.
 */
import type { Child } from "widget-jsx/jsx-runtime";

import type { TicketBlock, TicketField, TicketType } from "../content";
import { type Words, word } from "../lib/ticket_words";
import { ArrowUpIcon, BackIcon, ChevronRightIcon } from "./icons";

function TypeIcon({ icon }: { icon: string }) {
  const paths: Record<string, string> = {
    bug: "M8 9h8v6a4 4 0 0 1-8 0Zm1-3 1.5 1.5M15 6l-1.5 1.5M12 13v6M4 13h4m8 0h4M5 8l3 2m11-2-3 2M5 19l3-2m11 2-3-2",
    question: "M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm-2.5-11.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.5v.7m0 3v.5",
  };

  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d={paths[icon] ?? "M5 7h14v4a2 2 0 0 0 0 4v4H5v-4a2 2 0 0 0 0-4Zm7 0v12"} />
    </svg>
  );
}

/*
 * HomeCards are the request types the visitor may open, a section in
 * Home's list.
 */
export function HomeCards({ types, texts }: { types: TicketType[]; texts: Words }) {
  return (
    <>
      {types.length > 0 && (
        <section className="section">
          <header className="section-head">
            <h3>{word(texts, "tickets_get_help")}</h3>
          </header>
          <div className="list">
            {types.map((type) => (
              <button
                className="row ticket-type"
                type="button"
                data-action="click->visit"
                data-open="ticket"
                data-ticket-type={type.id}
                key={type.id}
              >
                <span className="row-icon">
                  <TypeIcon icon={type.icon} />
                </span>
                <span className="text">
                  <span>{type.name}</span>
                  {type.description ? <small>{type.description}</small> : null}
                </span>
                <ChevronRightIcon />
              </button>
            ))}
          </div>
        </section>
      )}
    </>
  );
}

/*
 * Filter switches the Messages list between every conversation and the
 * visitor's open requests.
 */
export function Filter({ open, texts }: { open: boolean; texts: Words }) {
  return (
    <div className="ticket-filter">
      <button type="button" data-ticket-filter="all" aria-pressed={String(!open)}>
        {word(texts, "tickets_all")}
      </button>
      <button type="button" data-ticket-filter="open" aria-pressed={String(open)}>
        {word(texts, "tickets_open_only")}
      </button>
    </div>
  );
}

/*
 * Strip shows where a request stands above its thread: submitted, we're
 * on it, resolved, with the label of its state under the current stop.
 */
export function Strip({ ticket, texts }: { ticket: TicketBlock; texts: Words }) {
  const group = ticket.state.group;
  const at = group === "resolved" ? 2 : group === "submitted" ? 0 : 1;
  const stops = ["ticket_submitted", "ticket_on_it", "ticket_resolved"];

  return (
    <div className="ticket-strip" data-group={group}>
      <p>
        <b>{ticket.reference}</b> {ticket.title}
      </p>
      <ol>
        {stops.map((key, index) => (
          <li data-done={String(index <= at)} aria-current={index === at ? "step" : undefined} key={key}>
            <i />
            <span>{word(texts, key)}</span>
          </li>
        ))}
      </ol>
      {ticket.state.label === word(texts, stops[at] ?? "") ? null : <small>{ticket.state.label}</small>}
    </div>
  );
}

function Input({ field, texts }: { field: TicketField; texts: Words }) {
  const name = `field.${field.key}`;
  const required = field.required || undefined;
  const label = field.label;

  switch (field.data_type) {
    case "long_text":
      return <textarea name={name} rows={3} required={required} aria-label={label} />;
    case "single_choice":
      return (
        <select name={name} required={required} aria-label={label}>
          <option value="">{word(texts, "ticket_choose")}</option>
          {field.options.map((option) => (
            <option value={option} key={option}>
              {option}
            </option>
          ))}
        </select>
      );
    case "multiple_choice":
      return (
        <span className="choices">
          {field.options.map((option) => (
            <label key={option}>
              <input type="checkbox" name={name} value={option} />
              {option}
            </label>
          ))}
        </span>
      );
    case "boolean":
      return (
        <span className="choices">
          <label>
            <input type="checkbox" name={name} value="true" />
            {word(texts, "ticket_yes")}
          </label>
        </span>
      );
    case "file":
      return <input type="file" name={name} multiple aria-label={word(texts, "ticket_attach")} />;
    case "number":
    case "decimal":
    case "currency":
      return <input type="number" step="any" name={name} required={required} aria-label={label} />;
    case "date":
      return <input type="date" name={name} required={required} aria-label={label} />;
    default:
      return <input type="text" name={name} required={required} aria-label={label} />;
  }
}

function Field({
  label,
  optional,
  children,
  field,
}: {
  label: string;
  optional?: string;
  children: Child;
  field?: TicketField;
}) {
  return (
    <div
      className="ticket-field"
      data-field={field?.key}
      data-show-field={field?.show_when?.field}
      data-show-equals={field?.show_when?.equals?.join("\n")}
      hidden={field?.show_when ? true : undefined}
    >
      <span>
        {label}
        {optional ? <em> ({optional})</em> : null}
      </span>
      {field?.description ? <small>{field.description}</small> : null}
      {children}
    </div>
  );
}

/*
 * Form is a request type's form: a subject, details, then the fields the
 * customer fills, each shown only when its condition holds.
 */
export function Form({ type, texts }: { type: TicketType; texts: Words }) {
  const optional = word(texts, "ticket_optional");
  const fields = type.fields.filter((field) => field.editable && field.data_type !== "member");

  return (
    <form className="ticket-form" data-ticket-form={type.id} noValidate>
      <p className="ticket-error" role="alert" hidden />
      <Field label={word(texts, "ticket_subject")}>
        <input type="text" name="title" maxLength={200} required aria-label={word(texts, "ticket_subject")} />
      </Field>
      <Field label={word(texts, "ticket_details")} optional={optional}>
        <textarea name="description" rows={4} aria-label={word(texts, "ticket_details")} />
      </Field>
      {fields.map((field) => (
        <Field label={field.label} optional={field.required ? undefined : optional} field={field} key={field.key}>
          <Input field={field} texts={texts} />
        </Field>
      ))}
      <button className="button primary wide" type="submit">
        {word(texts, "ticket_send")}
        <ArrowUpIcon />
      </button>
    </form>
  );
}

/*
 * Head is the request page's bar: back to Home and the type's name.
 */
export function Head({ title }: { title: string }) {
  return (
    <header className="bar back-bar">
      <button className="icon-button" type="button" data-action="click->back" aria-label="Back">
        <BackIcon />
      </button>
      <h2>{title}</h2>
      <span />
    </header>
  );
}
