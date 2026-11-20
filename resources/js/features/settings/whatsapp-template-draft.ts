/*
 * A WhatsApp template as the editor holds it. Its body is written like a
 * saved reply, with variables picked from the toolbar and kept in
 * Liquid's form; a detail that is not the customer's, such as an order
 * number, is a custom value with a name and an example. Meta wants
 * numbered variables with a sample each, so sending the template for
 * review numbers them in order, takes each one's sample from the detail
 * it stands for, and records which detail that is; opening a template
 * Meta has turns its numbers back.
 */
import { variableMarkdown, variableOf, variablePattern } from "@/features/broadcasts/writer/variables";
import { cleanVariables } from "@/features/saved-replies/render";
import type {
  WhatsAppBinding,
  WhatsAppButton,
  WhatsAppButtonType,
  WhatsAppTemplate,
  WhatsAppTemplateDraft,
} from "@/lib/api/whatsapp";

const liquid = new RegExp(variablePattern.source.slice(1), "g");
const numbered = /\{\{\s*(\d+)\s*\}\}/g;

export const templateLanguages: { id: string; name: string }[] = [
  { id: "en_US", name: "English (US)" },
  { id: "en_GB", name: "English (UK)" },
  { id: "en", name: "English" },
  { id: "fr", name: "French" },
  { id: "es", name: "Spanish" },
  { id: "es_MX", name: "Spanish (Mexico)" },
  { id: "pt_BR", name: "Portuguese (Brazil)" },
  { id: "pt_PT", name: "Portuguese (Portugal)" },
  { id: "de", name: "German" },
  { id: "it", name: "Italian" },
  { id: "nl", name: "Dutch" },
  { id: "ar", name: "Arabic" },
  { id: "hi", name: "Hindi" },
  { id: "id", name: "Indonesian" },
  { id: "tr", name: "Turkish" },
  { id: "ha", name: "Hausa" },
  { id: "yo", name: "Yoruba" },
  { id: "ig", name: "Igbo" },
  { id: "sw", name: "Swahili" },
  { id: "zh_CN", name: "Chinese (simplified)" },
  { id: "ja", name: "Japanese" },
];

export const buttonKinds: Record<WhatsAppButtonType, { label: string; detail: string; limit: number }> = {
  quick_reply: { label: "Quick reply", detail: "Comes back as the customer's message.", limit: 10 },
  url: { label: "Link", detail: "Opens a page in the customer's browser.", limit: 2 },
  phone_number: { label: "Phone number", detail: "Starts a call to this number.", limit: 1 },
};

/*
 * CustomValue is a detail filled in by hand when the template is sent:
 * its key in the body, the name teammates read, and the example Meta
 * reviews the template with.
 */
export interface CustomValue {
  key: string;
  label: string;
  sample: string;
}

/*
 * TemplateForm is the template while it is being written.
 */
export interface TemplateForm {
  name: string;
  language: string;
  category: "utility" | "marketing";
  header: string;
  body: string;
  footer: string;
  buttons: WhatsAppButton[];
  customs: CustomValue[];
}

export function emptyForm(): TemplateForm {
  return {
    name: "",
    language: "en_US",
    category: "utility",
    header: "",
    body: "",
    footer: "",
    buttons: [],
    customs: [],
  };
}

function words(key: string): string {
  const text = key.replace(/^custom\./, "").replaceAll("_", " ");
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/*
 * customKey names a custom value's variable from what the teammate
 * called it, apart from those already there.
 */
export function customKey(label: string, taken: string[]): string {
  const base = `custom.${
    label
      .toLowerCase()
      .replace(/[^a-z]+/g, "_")
      .replace(/^_+|_+$/g, "") || "value"
  }`;
  let key = base;
  while (taken.includes(key)) {
    key = `${key}_x`;
  }
  return key;
}

/*
 * formFrom opens a template Meta has: each numbered variable becomes the
 * detail dimmi recorded for it, or a custom value named by its number.
 */
export function formFrom(t: WhatsAppTemplate): TemplateForm {
  const customs: CustomValue[] = [];
  const body = t.content.body.replace(numbered, (_, n: string) => {
    const at = `body.${n}`;
    const bound = t.bindings[at];
    const key = bound?.key ?? `custom.value_${"abcdefghijklmnopqrstuvwxyz".charAt(Number(n) - 1) || "x"}`;
    if (!variableOf(key) && !customs.some((c) => c.key === key)) {
      customs.push({
        key,
        label: bound ? words(key) : `Value ${n}`,
        sample: t.content.samples[at] ?? "",
      });
    }
    return variableMarkdown(key, bound?.fallback ?? "");
  });

  return {
    name: t.name,
    language: t.language,
    category: t.category === "marketing" ? "marketing" : "utility",
    header: t.content.header,
    body,
    footer: t.content.footer,
    buttons: t.content.buttons.map((b) => ({ ...b })),
    customs,
  };
}

/*
 * usedCustoms are the custom values the body still names.
 */
export function usedCustoms(form: TemplateForm): CustomValue[] {
  const body = cleanVariables(form.body);
  return form.customs.filter((c) => body.includes(c.key));
}

function sampleOf(key: string, fallback: string | undefined, customs: CustomValue[]): string {
  return variableOf(key)?.sample || customs.find((c) => c.key === key)?.sample || fallback || words(key);
}

/*
 * draftOf writes the form the way Meta takes it: variables numbered in
 * the order they first appear, a sample for each, and the detail each
 * stands for.
 */
export function draftOf(form: TemplateForm): WhatsAppTemplateDraft {
  const order: string[] = [];
  const samples: Record<string, string> = {};
  const bindings: Record<string, WhatsAppBinding> = {};

  const body = cleanVariables(form.body).replace(liquid, (_, key: string, fallback?: string) => {
    if (!order.includes(key)) {
      order.push(key);
      const at = `body.${order.length}`;
      samples[at] = sampleOf(key, fallback, form.customs);
      bindings[at] = fallback ? { key, fallback } : { key };
    }
    return `{{${order.indexOf(key) + 1}}}`;
  });

  return {
    name: form.name,
    language: form.language,
    category: form.category,
    header: form.header.trim(),
    body,
    footer: form.footer.trim(),
    buttons: form.buttons,
    samples,
    bindings,
  };
}

/*
 * previewOf is the body as the customer reads it, each variable as its
 * sample.
 */
export function previewOf(form: TemplateForm): string {
  return cleanVariables(form.body).replace(liquid, (_, key: string, fallback?: string) =>
    sampleOf(key, fallback, form.customs),
  );
}

/*
 * sameForm reports whether two forms hold the same template.
 */
export function sameForm(a: TemplateForm, b: TemplateForm): boolean {
  return JSON.stringify(draftOf(a)) === JSON.stringify(draftOf(b));
}

/*
 * templateName turns what a teammate types into a name Meta takes.
 */
export function templateName(typed: string): string {
  return typed
    .toLowerCase()
    .replace(/[\s-]+/g, "_")
    .replace(/[^a-z0-9_]/g, "");
}
