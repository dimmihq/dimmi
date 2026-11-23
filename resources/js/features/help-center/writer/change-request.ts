/*
 * A change request drawn inside the article writer: blocks the request
 * removes are struck through in red where they stand, and blocks it adds
 * show in green after the block they follow. The writer stays editable;
 * the marks follow every edit, so a block reworded by hand to match the
 * request stops being marked. A requested new article is drawn whole, in
 * green.
 */

import { DOMSerializer, Fragment, type Node, type NodeType } from "@tiptap/pm/model";
import { type EditorState, Plugin, PluginKey } from "@tiptap/pm/state";
import { Decoration, DecorationSet } from "@tiptap/pm/view";
import { Extension } from "@tiptap/react";

import { diffBlocks } from "./change-diff";

export interface Requested {
  // proposed is the requested article, parsed with the writer's schema.
  proposed: Node | null;
  // whole marks every block as added, for a requested new article.
  whole: boolean;
}

export const changeRequestKey = new PluginKey<{ requested: Requested; marks: DecorationSet }>("changeRequest");

export const removedClass =
  "change-removed rounded-e-md ps-2 border-s-2 border-status-rose-text bg-status-rose-background/40 text-text-tertiary line-through decoration-status-rose-text/60";
export const addedClass =
  "change-added rounded-e-md ps-2 border-s-2 border-status-lime-text bg-status-lime-background/40";

/*
 * Unit is one line of the article as a request is read against it: a
 * block, one item of a list or one row of a table, so a list or table
 * with one reworded line marks only that line.
 */
interface Unit {
  node: Node;
  pos: number;
  key: string;
  // list is the kind of list an item belongs to.
  list: NodeType | null;
  // after is where the unit's block ends: its list's end for an item.
  after: number;
}

const lists = new Set(["bulletList", "orderedList", "table"]);

/*
 * unitsOf is a document's lines with where each starts, leaving out
 * empty paragraphs, which carry nothing to compare.
 */
function unitsOf(doc: Node): Unit[] {
  const out: Unit[] = [];
  doc.forEach((node, pos) => {
    if (node.type.name === "paragraph" && node.content.size === 0) {
      return;
    }
    if (!lists.has(node.type.name)) {
      out.push({ node, pos, key: JSON.stringify(node.toJSON()), list: null, after: pos + node.nodeSize });
      return;
    }
    node.forEach((item, offset) => {
      out.push({
        node: item,
        pos: pos + 1 + offset,
        key: `${node.type.name}:${JSON.stringify(item.toJSON())}`,
        list: node.type,
        after: pos + node.nodeSize,
      });
    });
  });
  return out;
}

/*
 * addedWidget draws one requested line, read-only, in green: an item
 * inside the list it joins, or a list or block of its own.
 */
function addedWidget(unit: Unit, inList: boolean, together: Unit[] = [unit]): () => HTMLElement {
  return () => {
    const serializer = DOMSerializer.fromSchema(unit.node.type.schema);
    let dom: HTMLElement;
    if (unit.list && inList) {
      dom = serializer.serializeNode(unit.node) as HTMLElement;
    } else if (unit.list) {
      // A new list or table is drawn whole, its items or rows together, not one list per item.
      const items = Fragment.from(together.map((u) => u.node));
      dom = serializer.serializeNode(unit.list.create(null, items)) as HTMLElement;
    } else {
      dom = document.createElement("div");
      dom.appendChild(serializer.serializeNode(unit.node));
    }
    dom.className = `${dom.className} ${addedClass}`.trim();
    dom.contentEditable = "false";
    dom.dataset.change = "added";
    return dom;
  };
}

/*
 * runOf is the added line at a change and the added lines right after
 * it that belong to the same new list or table.
 */
function runOf(changes: ReturnType<typeof diffBlocks>, at: number, after: Unit[]): Unit[] {
  const first = changes[at];
  if (first.kind !== "add") {
    return [];
  }
  const unit = after[first.to];
  const run = [unit];
  for (let i = at + 1; unit.list && i < changes.length; i++) {
    const next = changes[i];
    const more = next.kind === "add" ? after[next.to] : undefined;
    if (!more || more.list !== unit.list || more.after !== unit.after) {
      break;
    }
    run.push(more);
  }
  return run;
}

/*
 * marksOf draws a request against the document.
 */
function marksOf(doc: Node, requested: Requested): DecorationSet {
  if (!requested.proposed) {
    return DecorationSet.empty;
  }
  const before = unitsOf(doc);
  if (requested.whole) {
    return DecorationSet.create(
      doc,
      before.map((u) => Decoration.node(u.pos, u.pos + u.node.nodeSize, { class: addedClass })),
    );
  }
  return DecorationSet.create(doc, changeMarks(before, unitsOf(requested.proposed)));
}

/*
 * changeMarks marks the lines a request removes and draws the lines it
 * adds, each where it belongs among the document's lines.
 */
function changeMarks(before: Unit[], after: Unit[]): Decoration[] {
  const marks: Decoration[] = [];
  const first = before[0];
  let at = first ? first.pos : 0;
  let inList = Boolean(first?.list);
  let blockEnd = first ? first.after : 0;
  const changes = diffBlocks(
    before.map((u) => u.key),
    after.map((u) => u.key),
  );
  for (let i = 0; i < changes.length; i++) {
    const change = changes[i];
    if (change.kind === "add") {
      const unit = after[change.to];
      // The items or rows of one new list or table that follow each other are drawn as one.
      const together = inList ? [unit] : runOf(changes, i, after);
      i += together.length - 1;
      // The whole line is the key, so a line whose wording changed anywhere is drawn again.
      const key = `add:${change.to}:${together.map((u) => u.key).join("|")}`;
      const where = inList && !unit.list ? blockEnd : at;
      marks.push(Decoration.widget(where, addedWidget(unit, inList, together), { side: change.to, key }));
      continue;
    }
    const unit = before[change.from];
    if (change.kind === "del") {
      marks.push(Decoration.node(unit.pos, unit.pos + unit.node.nodeSize, { class: removedClass }));
    }
    at = unit.pos + unit.node.nodeSize;
    inList = Boolean(unit.list);
    blockEnd = unit.after;
  }
  return marks;
}

/*
 * ChangeRequest is the writer's change request marks. Send a request
 * with requestChanges.
 */
export const ChangeRequest = Extension.create({
  name: "changeRequest",
  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: changeRequestKey,
        state: {
          init: (): { requested: Requested; marks: DecorationSet } => ({
            requested: { proposed: null, whole: false },
            marks: DecorationSet.empty,
          }),
          apply: (tr, value, _old, state: EditorState) => {
            const sent = tr.getMeta(changeRequestKey) as Requested | undefined;
            if (!sent && !tr.docChanged) {
              return value;
            }
            const requested = sent ?? value.requested;
            return { requested, marks: marksOf(state.doc, requested) };
          },
        },
        props: {
          decorations: (state) => changeRequestKey.getState(state)?.marks,
        },
      }),
    ];
  },
});
