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
import { Form, Head, Sent } from "../components/tickets";
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

let pointed = false;

/*
 * picked shows a select's new choice on its trigger as it changes. A
 * choice made with the pointer closes the menu; arrow keys move the
 * choice and leave it open until Enter.
 */
function picked(target: EventTarget | null) {
  if (!(target instanceof HTMLInputElement) || target.type !== "radio") {
    return;
  }
  const select = target.closest<HTMLDetailsElement>("[data-select]");
  const shown = select?.querySelector<HTMLElement>("[data-select-label]");
  if (!select || !shown) {
    return;
  }
  shown.textContent = target.value;
  shown.dataset.empty = "false";
  if (pointed) {
    close(select);
  }
}

/*
 * close shuts a select's menu and hands focus back to its trigger.
 */
function close(select: HTMLDetailsElement) {
  select.open = false;
  select.querySelector("summary")?.focus();
}

const watched = new WeakSet<ShadowRoot>();

/*
 * place shows an opening select's menu in the page's top layer, so it
 * can reach past the messenger's edges, lined up with the trigger: under
 * it when the screen has room there, above it when there is more room
 * above, and never taller than the room it has. Without the top layer
 * the menu stays inside the view and fits the room there.
 */
function place(target: EventTarget | null) {
  if (!(target instanceof HTMLDetailsElement) || !target.matches("[data-select]")) {
    return;
  }
  const menu = target.querySelector<HTMLElement>(".ticket-menu");
  const trigger = target.querySelector<HTMLElement>("summary");
  if (!menu || !trigger) {
    return;
  }
  const floating = float(menu, target.open);
  if (!target.open) {
    return;
  }
  const room = floating ? screenRoom(trigger) : viewRoom(target, trigger);
  const wanted = Math.min(menu.scrollHeight, 360);
  const up = room.below < wanted && room.above > room.below;
  target.dataset.up = String(up);
  menu.style.maxBlockSize = `${Math.max(120, Math.min(360, up ? room.above : room.below))}px`;
  if (floating) {
    pin(menu, trigger, up);
  }
}

/*
 * float shows or hides the menu in the top layer to match its select,
 * answering whether the browser has a top layer at all.
 */
function float(menu: HTMLElement, open: boolean): boolean {
  if (typeof menu.showPopover !== "function") {
    return false;
  }
  const shown = menu.matches(":popover-open");
  if (open && !shown) {
    menu.showPopover();
  } else if (!open && shown) {
    menu.hidePopover();
  }
  return true;
}

/*
 * pin lines a floating menu up with its trigger, under it or above it.
 */
function pin(menu: HTMLElement, trigger: HTMLElement, up: boolean) {
  const at = trigger.getBoundingClientRect();
  const height = trigger.ownerDocument.defaultView?.innerHeight ?? 0;
  menu.style.left = `${at.left}px`;
  menu.style.width = `${at.width}px`;
  menu.style.top = up ? "auto" : `${at.bottom + 6}px`;
  menu.style.bottom = up ? `${height - at.top + 6}px` : "auto";
}

/*
 * screenRoom is how much of the screen is free under and over the
 * trigger.
 */
function screenRoom(trigger: HTMLElement) {
  const at = trigger.getBoundingClientRect();
  const height = trigger.ownerDocument.defaultView?.innerHeight ?? 0;

  return { below: height - at.bottom - 12, above: at.top - 12 };
}

/*
 * viewRoom is how much of the view is free under the trigger, above the
 * pinned action, and over it, under the header.
 */
function viewRoom(select: HTMLElement, trigger: HTMLElement) {
  const view = select.closest<HTMLElement>(".view");
  const at = trigger.getBoundingClientRect();
  const bounds = view?.getBoundingClientRect() ?? at;
  const bar = view?.querySelector<HTMLElement>(".ticket-actions")?.getBoundingClientRect();
  const head = view?.querySelector<HTMLElement>(".bar")?.getBoundingClientRect();

  return {
    below: (bar ? bar.top : bounds.bottom) - at.bottom - 14,
    above: at.top - (head ? head.bottom : bounds.top) - 14,
  };
}

/*
 * closeSelects notes whether the customer is using the pointer or the
 * keyboard, closes an open select when they click outside it, and closes
 * it on Escape, or on Enter once a choice is made.
 */
function closeSelects(root: ShadowRoot) {
  if (watched.has(root)) {
    return;
  }
  watched.add(root);
  root.addEventListener("pointerdown", (event) => {
    pointed = true;
    const path = event.composedPath();
    for (const select of root.querySelectorAll<HTMLDetailsElement>("[data-select][open]")) {
      if (!path.includes(select)) {
        select.open = false;
      }
    }
  });
  root.addEventListener("toggle", (event) => place(event.target), true);
  const follow = () => {
    for (const select of root.querySelectorAll("[data-select][open]")) {
      place(select);
    }
  };
  root.addEventListener("scroll", follow, true);
  root.host.ownerDocument.defaultView?.addEventListener("resize", follow);
  root.addEventListener("keydown", (event) => {
    pointed = false;
    const select = (event.target as Element | null)?.closest<HTMLDetailsElement>("[data-select][open]");
    if (select && (event as KeyboardEvent).key === "Enter" && event.target instanceof HTMLInputElement) {
      event.preventDefault();
      close(select);
    }
  });
  // Escape closes an open menu first, wherever focus is, before the
  // messenger hears it and closes the panel.
  root.host.ownerDocument.defaultView?.addEventListener(
    "keydown",
    (event) => {
      const open = root.querySelector<HTMLDetailsElement>("[data-select][open]");
      if (open && event.key === "Escape") {
        event.preventDefault();
        event.stopImmediatePropagation();
        close(open);
      }
    },
    true,
  );
}

/*
 * nudge shakes the button once, when the form cannot be sent yet.
 */
function nudge(button: HTMLButtonElement | null) {
  if (!button) {
    return;
  }
  button.dataset.state = "nudge";
  button.addEventListener("animationend", () => delete button.dataset.state, { once: true });
}

/*
 * done swaps the form for the page that says the request was sent; its
 * button opens the request's conversation.
 */
function done(view: HTMLElement, type: TicketType, title: string, texts: object, opener: HTMLElement) {
  const stack = view.querySelector<HTMLElement>(".stack");
  if (!stack) {
    opener.click();
    return;
  }
  stack.innerHTML = String(Sent({ type, title, texts }));
  const open = stack.querySelector<HTMLButtonElement>("[data-ticket-view]");
  open?.addEventListener("click", () => opener.click());
  open?.focus();
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
  form.addEventListener("input", (event) => {
    picked(event.target);
    showWhen(form);
  });
  closeSelects(root);
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const sent = read(form, type);
    for (const box of form.querySelectorAll<HTMLElement>("[data-invalid]")) {
      delete box.dataset.invalid;
    }
    for (const box of sent.missing) {
      box.dataset.invalid = word(texts, "ticket_required");
    }
    const button = form.querySelector<HTMLButtonElement>('button[type="submit"]');
    if (sent.missing.length > 0) {
      sent.missing[0]?.querySelector<HTMLElement>("summary, input, select, textarea")?.focus();
      nudge(button);
      return;
    }
    button?.toggleAttribute("disabled", true);
    button?.setAttribute("aria-busy", "true");
    if (button) {
      button.dataset.state = "sending";
    }
    error.hidden = true;
    try {
      opener.dataset.conversation = live ? await send(live, { client: clientId, type: type.id }, sent) : "";
      done(view, type, sent.title, texts, opener);
    } catch {
      error.textContent = word(texts, "ticket_failed");
      error.hidden = false;
      button?.toggleAttribute("disabled", false);
      button?.removeAttribute("aria-busy");
      nudge(button);
    }
  });
}
