/*
 * A button as the writer draws it: the help center's brand button, which
 * opens its settings instead of following the link. Its label, its link
 * and where it sits are set in a popover; a new button opens there.
 */
import { RiAlignCenter, RiAlignLeft } from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverFooter, PopoverHeader } from "@/components/base/popover/popover";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { cx } from "@/utils/cx";

import type { ButtonAlign } from "./article-button";
import { BlockActions } from "./block-actions";
import { validLink } from "./button-link";

export function ButtonView({ node, updateAttributes, deleteNode, editor, selected }: NodeViewProps) {
  const label = String(node.attrs.label ?? "");
  const href = String(node.attrs.href ?? "");
  const align = (node.attrs.align as ButtonAlign) ?? "start";
  const anchor = useRef<HTMLButtonElement | null>(null);
  const [editing, setEditing] = useState(href === "" && label === "");
  const [draft, setDraft] = useState({ label, href, align: align === "center" ? "center" : "start" });
  const [tried, setTried] = useState(false);
  const labelled = draft.label.trim() !== "";
  const linked = validLink(draft.href);

  const open = () => {
    setDraft({ label, href, align: align === "center" ? "center" : "start" });
    setEditing(true);
  };
  // A block just added asks for its settings, so bring it into view first.
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs once, for a new block
  useEffect(() => {
    if (editing) {
      anchor.current?.scrollIntoView({ block: "center" });
    }
  }, []);

  const close = () => {
    setEditing(false);
    setTried(false);
    if (href === "" && label === "") {
      deleteNode();
    }
  };
  const save = () => {
    setTried(true);
    if (!labelled || !linked) {
      return;
    }
    updateAttributes({ label: draft.label.trim(), href: draft.href.trim(), align: draft.align });
    setEditing(false);
    setTried(false);
    editor.commands.focus();
  };

  return (
    <NodeViewWrapper as="p" data-article-button="" className={cx("button-row", `button-align-${align}`, "relative")}>
      <button
        ref={anchor}
        type="button"
        contentEditable={false}
        className="button"
        onClick={() => editor.isEditable && open()}
      >
        {label || "Button"}
      </button>
      {selected && editor.isEditable && !editing ? (
        <BlockActions label="Button" editLabel="Edit button" onEdit={open} onRemove={deleteNode} />
      ) : null}
      <Popover
        triggerRef={anchor}
        isOpen={editing && editor.isEditable}
        onOpenChange={(next) => (next ? setEditing(true) : close())}
        placement="bottom start"
        aria-label="Button"
      >
        <form
          className="flex flex-col gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            save();
          }}
        >
          <PopoverHeader title="Button" />
          <Input
            size="small"
            label="Label"
            autoFocus
            placeholder="Start a free trial"
            value={draft.label}
            onChange={(value) => setDraft({ ...draft, label: value })}
            isInvalid={tried && !labelled}
            errorMessage="Give the button a label."
          />
          <Input
            size="small"
            label="Link"
            inputDir="ltr"
            placeholder="https://"
            value={draft.href}
            onChange={(value) => setDraft({ ...draft, href: value })}
            isInvalid={tried && !linked}
            errorMessage="Use a web address, an email address or a path such as /pricing."
          />
          <RadioGroup
            aria-label="Position"
            value={draft.align}
            onChange={(value) => setDraft({ ...draft, align: value })}
            className="grid grid-cols-2 gap-2"
          >
            <RadioCard value="start" title="Left" icon={RiAlignLeft} />
            <RadioCard value="center" title="Center" icon={RiAlignCenter} />
          </RadioGroup>
          <PopoverFooter>
            <Button variant="secondary" size="small" type="button" onClick={close}>
              Cancel
            </Button>
            <Button size="small" type="submit">
              Save
            </Button>
          </PopoverFooter>
        </form>
      </Popover>
    </NodeViewWrapper>
  );
}
