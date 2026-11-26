/*
 * An embed as the writer draws it: the player readers will see, under a
 * cover that lets a click pick the block instead of starting the video.
 * A new embed, or one whose link is being changed, asks for its link in
 * a popover beside it, naming the hosts that work and refusing others.
 */
import { RiFilmLine } from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Popover, PopoverFooter, PopoverHeader } from "@/components/base/popover/popover";
import { cx } from "@/utils/cx";

import { BlockActions } from "./block-actions";
import { embedProviders, resolveEmbed } from "./embed-providers";

export function EmbedView({ node, updateAttributes, deleteNode, editor, selected }: NodeViewProps) {
  const src = String(node.attrs.src ?? "");
  const title = String(node.attrs.title ?? "");
  const embed = resolveEmbed(src);
  const anchor = useRef<HTMLDivElement | null>(null);
  const [editing, setEditing] = useState(src === "");
  const [link, setLink] = useState(src);
  const [name, setName] = useState(title);
  const [tried, setTried] = useState(false);
  const valid = resolveEmbed(link) !== null;

  const close = () => {
    setEditing(false);
    setTried(false);
    if (src === "") {
      deleteNode();
    }
  };
  const save = () => {
    setTried(true);
    if (!valid) {
      return;
    }
    updateAttributes({ src: link.trim(), title: name.trim() });
    setEditing(false);
    setTried(false);
    editor.commands.focus();
  };

  return (
    <NodeViewWrapper className="embed-block" data-drag-handle="">
      <div ref={anchor} contentEditable={false} className="relative">
        {embed ? (
          <figure className={cx("embed", `embed-${embed.provider}`)}>
            <iframe
              src={embed.player}
              title={title || "Embedded content"}
              loading="lazy"
              allow="autoplay; fullscreen; picture-in-picture; encrypted-media; clipboard-write"
              allowFullScreen
              sandbox="allow-scripts allow-same-origin allow-popups allow-presentation allow-forms"
              referrerPolicy="strict-origin-when-cross-origin"
            />
            {selected ? null : <span aria-hidden className="embed-cover" />}
          </figure>
        ) : (
          <div className="embed-empty">
            <RiFilmLine aria-hidden className="size-5 text-foreground-icon-tertiary" />
            <span className="text-body-2-regular text-text-tertiary">
              {src ? "This link can't be embedded" : "Add a video or demo link"}
            </span>
          </div>
        )}
        {selected && editor.isEditable && !editing ? (
          <BlockActions
            label="Embed"
            onEdit={() => {
              setLink(src);
              setName(title);
              setEditing(true);
            }}
            editLabel="Change link"
            onRemove={deleteNode}
          />
        ) : null}
      </div>
      <Popover
        triggerRef={anchor}
        isOpen={editing && editor.isEditable}
        onOpenChange={(open) => (open ? setEditing(true) : close())}
        placement="bottom start"
        aria-label="Embed a video or demo"
      >
        <form
          className="flex flex-col gap-3"
          onSubmit={(event) => {
            event.preventDefault();
            save();
          }}
        >
          <PopoverHeader title="Embed a video or demo" />
          <Input
            size="small"
            label="Link"
            inputDir="ltr"
            autoFocus
            placeholder="https://youtu.be/…"
            value={link}
            onChange={setLink}
            isInvalid={tried && !valid}
            errorMessage="This link isn't from a host articles can embed."
            hint={`Works with ${embedProviders.join(", ")}.`}
          />
          <Input
            size="small"
            label="Title"
            placeholder="What it shows"
            hint="Read out by screen readers."
            value={name}
            onChange={setName}
          />
          <PopoverFooter>
            <Button variant="secondary" size="small" type="button" onClick={close}>
              Cancel
            </Button>
            <Button size="small" type="submit">
              Embed
            </Button>
          </PopoverFooter>
        </form>
      </Popover>
    </NodeViewWrapper>
  );
}
