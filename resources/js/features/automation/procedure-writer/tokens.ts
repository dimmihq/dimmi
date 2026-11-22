/*
 * A step's words and its chips. A chip is kept in the text as
 * [[kind:ref]], exactly as the server reads it, and the step's bindings
 * describe each one, so the runtime never guesses from a label. The
 * writer turns that text into a document of words and chips and back;
 * every edit brings the bindings in line with the chips still there,
 * keeping the settings of those that stayed.
 */
import type { BindingKind, ProcedureBinding, ProcedureStep } from "@/lib/api/procedures";

export const bindingKinds: BindingKind[] = [
  "action",
  "attribute",
  "article",
  "handoff",
  "tag",
  "ticket_type",
  "sub_procedure",
];

const markerPattern = /\[\[(action|attribute|article|handoff|tag|ticket_type|sub_procedure):([^\]\s]+)\]\]/g;

export interface Chip {
  kind: BindingKind;
  ref: string;
}

/*
 * DocNode is the part of a TipTap document the writer reads and writes.
 */
export interface DocNode {
  type: string;
  attrs?: Record<string, unknown>;
  content?: DocNode[];
  text?: string;
}

export function marker(chip: Chip): string {
  return `[[${chip.kind}:${chip.ref}]]`;
}

export function chipsOf(text: string): Chip[] {
  return [...text.matchAll(markerPattern)].map((match) => ({
    kind: match[1] as BindingKind,
    ref: match[2] ?? "",
  }));
}

/*
 * textToDoc reads a step's words into one paragraph of text, chips and
 * line breaks.
 */
export function textToDoc(text: string, labelOf: (chip: Chip) => string): DocNode {
  const content: DocNode[] = [];
  const words = (part: string) => {
    part.split("\n").forEach((line, index) => {
      if (index > 0) {
        content.push({ type: "hardBreak" });
      }
      if (line) {
        content.push({ type: "text", text: line });
      }
    });
  };
  let at = 0;
  for (const match of text.matchAll(markerPattern)) {
    words(text.slice(at, match.index));
    const chip = { kind: match[1] as BindingKind, ref: match[2] ?? "" };
    content.push({ type: "procedureToken", attrs: { ...chip, label: labelOf(chip) } });
    at = (match.index ?? 0) + match[0].length;
  }
  words(text.slice(at));

  return { type: "doc", content: [{ type: "paragraph", content }] };
}

/*
 * docToText writes a document back as words with chip markers.
 */
export function docToText(doc: DocNode): string {
  const lines: string[] = [];
  const inline = (nodes: DocNode[] = []): string =>
    nodes
      .map((node): string => {
        if (node.type === "text") {
          return node.text ?? "";
        }
        if (node.type === "hardBreak") {
          return "\n";
        }
        if (node.type === "procedureToken") {
          return marker({ kind: node.attrs?.kind as BindingKind, ref: String(node.attrs?.ref ?? "") });
        }
        return inline(node.content);
      })
      .join("");
  for (const block of doc.content ?? []) {
    lines.push(inline(block.content));
  }

  return lines.join("\n").trim();
}

/*
 * defaultExecution is how an action chip is enforced when nobody said:
 * conditional in a condition written in plain words, required anywhere
 * else.
 */
export function defaultExecution(step: ProcedureStep): "required" | "conditional" {
  return step.kind === "condition" && !step.condition ? "conditional" : "required";
}

/*
 * syncBindings brings a step's bindings in line with the chips in its
 * new words: chips that stayed keep their settings, new ones get the
 * defaults, and removed ones go. A legacy action step's action stays.
 */
export function syncBindings(
  step: ProcedureStep,
  text: string,
  labelOf: (chip: Chip) => string,
): ProcedureBinding[] | undefined {
  const chips = chipsOf(text);
  const kept = new Map((step.bindings ?? []).map((b) => [`${b.kind}:${b.ref}`, b]));
  const out: ProcedureBinding[] = [];
  const seen = new Set<string>();
  for (const chip of chips) {
    const key = `${chip.kind}:${chip.ref}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    const before = kept.get(key);
    if (before) {
      out.push(before);
      continue;
    }
    const binding: ProcedureBinding = { kind: chip.kind, ref: chip.ref, label: labelOf(chip) };
    if (chip.kind === "action") {
      binding.execution = defaultExecution(step);
      binding.on_missing = "handoff";
    }
    out.push(binding);
  }
  if (step.action_id && !seen.has(`action:${step.action_id}`)) {
    const legacy = kept.get(`action:${step.action_id}`);
    if (legacy) {
      out.push(legacy);
    }
  }

  return out.length > 0 ? out : undefined;
}
