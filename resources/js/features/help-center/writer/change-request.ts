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
  "change-removed rounded-md bg-status-rose-background text-text-secondary line-through decoration-status-rose-text";
export const addedClass = "change-added rounded-md bg-status-lime-background";

/*
 * Unit is one line of the article as a request is read against it: a
 * block, or one item of a list, so a list with one reworded item marks
 * only that item.
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

const lists = new Set(["bulletList", "orderedList"]);

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
function addedWidget(unit: Unit, inList: boolean): () => HTMLElement {
  return () => {
    const serializer = DOMSerializer.fromSchema(unit.node.type.schema);
    let dom: HTMLElement;
    if (unit.list && inList) {
      dom = serializer.serializeNode(unit.node) as HTMLElement;
    } else if (unit.list) {
      dom = serializer.serializeNode(unit.list.create(null, Fragment.from(unit.node))) as HTMLElement;
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
  const after = unitsOf(requested.proposed);
  const marks: Decoration[] = [];
  let at = before[0]?.pos ?? 0;
  let inList = Boolean(before[0]?.list);
  let blockEnd = before[0]?.after ?? 0;
  for (const change of diffBlocks(
    before.map((u) => u.key),
    after.map((u) => u.key),
  )) {
    if (change.kind === "add") {
      const unit = after[change.to];
      const key = `add:${change.to}:${unit.node.textContent.slice(0, 40)}`;
      const where = inList && !unit.list ? blockEnd : at;
      marks.push(Decoration.widget(where, addedWidget(unit, inList), { side: change.to, key }));
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
  return DecorationSet.create(doc, marks);
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
