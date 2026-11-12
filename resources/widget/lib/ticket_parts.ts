/*
 * What the requests chunk carries: the parts Home, Messages and a
 * request's thread draw, the request form with its checks and sending,
 * and their stylesheet, adopted once into each messenger root.
 *
 * The form shows a field only while its condition holds and leaves a
 * hidden field out of what it sends. It checks the fields the customer
 * must fill before anything leaves the browser, uploads the files of its
 * file fields the way the composer does, and sends the request with a
 * client id kept across retries, so a second press makes one request.
 * Once sent, the messenger opens the request's conversation.
 */
import { Form, Head } from "../components/tickets";
import type { TicketField, TicketType } from "../content";
import styles from "../styles/tickets.css?inline";
import type { Live } from "./live";
import { word } from "./ticket_words";

export { Filter, HomeCards, Strip } from "../components/tickets";

const styled = new WeakSet<ShadowRoot>();

export function adopt(root: ShadowRoot) {
  if (styled.has(root)) {
    return;
  }
  const sheet = new CSSStyleSheet();
  sheet.replaceSync(styles);
  root.adoptedStyleSheets = [...root.adoptedStyleSheets, sheet];
  styled.add(root);
}

type Inputs = HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;

/*
 * showWhen shows each conditional field while the field it names has a
 * value, one of those listed when it lists any.
 */
export function showWhen(form: HTMLFormElement) {
  const data = new FormData(form);
  for (const box of form.querySelectorAll<HTMLElement>("[data-show-field]")) {
    const values = data.getAll(`field.${box.dataset.showField}`).filter((v) => typeof v === "string" && v !== "");
    const equals = (box.dataset.showEquals ?? "").split("\n").filter(Boolean);
    box.hidden = values.length === 0 || (equals.length > 0 && !values.some((v) => equals.includes(String(v))));
  }
}

interface Read {
  title: string;
  description: string;
  fields: Record<string, unknown>;
  files: Record<string, File[]>;
  missing: HTMLElement[];
}

/*
 * read takes what the form holds, leaving hidden fields out, and lists
 * the required ones left empty.
 */
function read(form: HTMLFormElement, type: TicketType): Read {
  const data = new FormData(form);
  const out: Read = {
    title: String(data.get("title") ?? "").trim(),
    description: String(data.get("description") ?? "").trim(),
    fields: {},
    files: {},
    missing: [],
  };
  const title = form.querySelector<HTMLElement>(".ticket-field");
  if (!out.title && title) {
    out.missing.push(title);
  }
  for (const field of type.fields) {
    const box = form.querySelector<HTMLElement>(`[data-field="${field.key}"]`);
    if (box && !box.hidden) {
      take(out, field, data.getAll(`field.${field.key}`));
      if (field.required && !(field.key in out.fields) && !(field.key in out.files)) {
        out.missing.push(box);
      }
    }
  }

  return out;
}

const numeric = ["number", "decimal", "currency"];

/*
 * take keeps one field's value as the field holds it: files to upload,
 * a list of choices, a number, true, or text.
 */
function take(out: Read, field: TicketField, all: FormDataEntryValue[]) {
  const files = all.filter((v): v is File => v instanceof File && v.size > 0);
  const texts = all.filter((v): v is string => typeof v === "string" && v !== "");
  const first = texts[0];
  if (field.data_type === "file") {
    if (files.length > 0) {
      out.files[field.key] = files;
    }
    return;
  }
  if (field.data_type === "multiple_choice") {
    if (texts.length > 0) {
      out.fields[field.key] = texts;
    }
    return;
  }
  if (first !== undefined) {
    out.fields[field.key] =
      field.data_type === "boolean" ? true : numeric.includes(field.data_type) ? Number(first) : first;
  }
}

/*
 * send uploads the form's files and sends the request, answering the id
 * of the conversation it opened.
 */
async function send(live: Live, key: { client: string; type: string }, form: Read): Promise<string> {
  await live.ensure();
  const fields = { ...form.fields };
  for (const [key, files] of Object.entries(form.files)) {
    fields[key] = await live.upload(files);
  }
  const res = await live.http.post<{ data: { conversation_id: string } }>("/tickets", {
    client_id: key.client,
    type_id: key.type,
    title: form.title,
    description: form.description || undefined,
    fields,
  });
  await live.reloadConversations();

  return res.data.conversation_id;
}

/*
 * openForm draws the type's form on the request page and sends it from
 * there; without a connection, as in a preview, it opens a new
 * conversation instead.
 */
export function openForm(root: ShadowRoot, type: TicketType, texts: object, live: Live | undefined) {
  const view = root.querySelector<HTMLElement>('.view[data-view="ticket"]');
  if (!view) {
    return;
  }
  view.innerHTML =
    String(Head({ title: type.name })) +
    `<div class="stack">${String(Form({ type, texts }))}</div>` +
    '<button type="button" hidden data-action="click->converse" data-ticket-open></button>';
  root.dispatchEvent(new CustomEvent("dimmi:connect"));

  const form = view.querySelector<HTMLFormElement>("form");
  const error = view.querySelector<HTMLElement>(".ticket-error");
  const opener = view.querySelector<HTMLElement>("[data-ticket-open]");
  if (!form || !error || !opener) {
    return;
  }
  const clientId = crypto.randomUUID();
  showWhen(form);
  form.addEventListener("input", () => showWhen(form));
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const sent = read(form, type);
    for (const box of form.querySelectorAll<HTMLElement>("[data-invalid]")) {
      delete box.dataset.invalid;
    }
    for (const box of sent.missing) {
      box.dataset.invalid = word(texts, "ticket_required");
    }
    if (sent.missing.length > 0) {
      sent.missing[0]?.querySelector<Inputs>("input, select, textarea")?.focus();
      return;
    }
    const button = form.querySelector<HTMLButtonElement>('button[type="submit"]');
    button?.toggleAttribute("disabled", true);
    error.hidden = true;
    try {
      opener.dataset.conversation = live ? await send(live, { client: clientId, type: type.id }, sent) : "";
      opener.click();
    } catch {
      error.textContent = word(texts, "ticket_failed");
      error.hidden = false;
    } finally {
      button?.toggleAttribute("disabled", false);
    }
  });
}
