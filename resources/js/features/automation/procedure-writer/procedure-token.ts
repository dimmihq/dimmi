/*
 * A chip in a procedure step: an action dimmi calls, a customer detail,
 * a help article, a team to hand over to, a tag, a ticket type or a
 * sub-procedure. It is one atom in the text, written back as
 * [[kind:ref]], which is what the server reads. Pressing an action chip
 * opens how strictly dimmi must call it. A chip ends with a space so
 * typing goes on after it; punctuation typed straight after takes that
 * space back, so "Run Lookup order, then" reads without a gap.
 */
import { Plugin } from "@tiptap/pm/state";
import { mergeAttributes, Node, ReactNodeViewRenderer } from "@tiptap/react";
import { createContext } from "react";

import type { ProcedureBinding, ProcedureStep } from "@/lib/api/procedures";

import { ProcedureChip } from "./procedure-chip";
import type { ChipSources } from "./procedure-suggestions";
import { type Chip, marker } from "./tokens";

/*
 * ChipContext pushes live sources and step state into TipTap node views so
 * chips re-render as soon as actions/articles load or bindings change.
 */
export interface ChipContextValue {
  sources: ChipSources;
  step: ProcedureStep;
  earlier: (readonly [string, string])[];
}

export const ChipContext = createContext<ChipContextValue | null>(null);

declare module "@tiptap/react" {
  interface Commands<ReturnType> {
    procedureToken: {
      insertChip: (chip: Chip, label: string) => ReturnType;
    };
  }
}

/*
 * ChipOptions connect a chip to its step: the name to show, its binding,
 * how to change the binding and the step's prerequisites, and the
 * earlier actions it may wait for, as [id, name] pairs.
 */
export interface ChipOptions {
  label: (chip: Chip) => string;
  binding: (chip: Chip) => ProcedureBinding | undefined;
  onBinding: (chip: Chip, change: Partial<ProcedureBinding>) => void;
  requires: () => string[];
  onRequires: (ids: string[]) => void;
  earlier: () => (readonly [string, string])[];
}

export const ProcedureToken = Node.create<ChipOptions>({
  name: "procedureToken",

  addOptions() {
    return {
      label: (chip) => chip.ref,
      binding: () => undefined,
      onBinding: () => undefined,
      requires: () => [],
      onRequires: () => undefined,
      earlier: () => [],
    };
  },

  group: "inline",
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      kind: { default: "action" },
      ref: { default: "" },
      label: { default: "" },
    };
  },

  parseHTML() {
    return [
      {
        tag: "span[data-chip]",
        getAttrs: (element) => ({
          kind: element.getAttribute("data-chip"),
          ref: element.getAttribute("data-ref") ?? "",
          label: element.textContent ?? "",
        }),
      },
    ];
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      "span",
      mergeAttributes(HTMLAttributes, { "data-chip": node.attrs.kind, "data-ref": node.attrs.ref }),
      String(node.attrs.label || node.attrs.ref),
    ];
  },

  renderText({ node }) {
    return marker({ kind: node.attrs.kind, ref: String(node.attrs.ref) });
  },

  addNodeView() {
    return ReactNodeViewRenderer(ProcedureChip, { as: "span" });
  },

  addProseMirrorPlugins() {
    return [
      new Plugin({
        props: {
          handleTextInput(view, from, to, text) {
            const { doc } = view.state;
            if (!/^[,.!?;:)]$/.test(text) || from !== to || from < 2) {
              return false;
            }
            const before = doc.resolve(from - 1).nodeBefore;
            if (doc.textBetween(from - 1, from) !== " " || before?.type.name !== "procedureToken") {
              return false;
            }
            view.dispatch(view.state.tr.insertText(text, from - 1, from));

            return true;
          },
        },
      }),
    ];
  },

  addCommands() {
    return {
      insertChip:
        (chip, label) =>
        ({ commands }) =>
          commands.insertContent([
            { type: this.name, attrs: { kind: chip.kind, ref: chip.ref, label } },
            { type: "text", text: " " },
          ]),
    };
  },
});
