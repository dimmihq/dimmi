/*
 * What the messenger draws for requests: Home's "Get help with" cards,
 * the form of a request type, the filter of open requests, and the strip
 * above a request's thread from submitted to resolved. Words come from the messenger's texts, English when an older
 * config lacks them.
 */
import type { Child } from "widget-jsx/jsx-runtime";

import type { TicketBlock, TicketField, TicketType } from "../content";
import { type Words, word } from "../lib/ticket_words";
import { BackIcon, CheckIcon, ChevronDownIcon, ChevronRightIcon } from "./icons";

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

/*
 * Option is one checkbox card: the real checkbox stays in the form,
 * hidden, and a drawn box in the brand's colour shows its state.
 */
function Option({ name, value, children }: { name: string; value: string; children: Child }) {
  return (
    <label className="ticket-option">
      <input type="checkbox" name={name} value={value} />
      <span className="ticket-box" aria-hidden="true">
        <CheckIcon className="ticket-box-tick" />
      </span>
      <span>{children}</span>
    </label>
  );
}

/*
 * Select is a single choice drawn by the messenger rather than the
 * browser: a trigger that shows the pick, and a menu of real radio
 * inputs, so the form reads and checks it like any other field.
 */
function Select({
  name,
  label,
  options,
  placeholder,
}: {
  name: string;
  label: string;
  options: string[];
  placeholder: string;
}) {
  return (
    <details className="ticket-select" data-select>
      <summary className="ticket-trigger" aria-label={label}>
        <span className="ticket-value" data-select-label data-empty="true">
          {placeholder}
        </span>
        <ChevronDownIcon className="ticket-chevron" />
      </summary>
      <div className="ticket-menu" role="radiogroup" aria-label={label} popover="manual">
        {options.map((option) => (
          <label className="ticket-menu-item" key={option}>
            <input type="radio" name={name} value={option} />
            <span className="ticket-menu-text">{option}</span>
            <CheckIcon className="ticket-tick" />
          </label>
        ))}
      </div>
    </details>
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
      return <Select name={name} label={label} options={field.options} placeholder={word(texts, "ticket_choose")} />;
    case "multiple_choice":
      return (
        <fieldset className="ticket-options" aria-label={label}>
          {field.options.map((option) => (
            <Option name={name} value={option} key={option}>
              {option}
            </Option>
          ))}
        </fieldset>
      );
    case "boolean":
      return (
        <fieldset className="ticket-options" aria-label={label}>
          <Option name={name} value="true">
            {word(texts, "ticket_yes")}
          </Option>
        </fieldset>
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
      {field?.description ? <small className="ticket-hint">{field.description}</small> : null}
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
      <div className="ticket-actions">
        <button className="button primary wide ticket-submit" type="submit">
          <span className="ticket-submit-label">{word(texts, "ticket_send")}</span>
          <ChevronRightIcon className="ticket-caret" />
          <span className="ticket-spinner" aria-hidden="true" />
        </button>
      </div>
    </form>
  );
}

/*
 * Sent is the page a request lands on once sent: a tick that draws
 * itself, what was sent, where the reply will come, and the way to the
 * request's conversation, pinned at the bottom like the form's button.
 */
export function Sent({ type, title, texts }: { type: TicketType; title: string; texts: Words }) {
  return (
    <div className="ticket-sent" role="status">
      <div className="ticket-sent-body">
        <svg className="ticket-sent-mark" viewBox="0 0 64 64" aria-hidden="true">
          <circle className="ticket-sent-ring" cx="32" cy="32" r="28" />
          <path className="ticket-sent-tick" d="m21 33 7.5 7.5L43 25" />
        </svg>
        <h3>{word(texts, "ticket_sent")}</h3>
        <p>{word(texts, "ticket_sent_body")}</p>
        <div className="ticket-sent-card">
          <small className="ticket-sent-type">{type.name}</small>
          <strong className="ticket-sent-title">{title}</strong>
        </div>
      </div>
      <div className="ticket-actions">
        <button className="button primary wide ticket-submit" type="button" data-ticket-view>
          <span className="ticket-submit-label">{word(texts, "ticket_view")}</span>
          <ChevronRightIcon className="ticket-caret" />
        </button>
      </div>
    </div>
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
