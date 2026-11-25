/*
 * A mention as it reads in the instructions: a small badge with the
 * kind's icon and the name, in the kind's colour; a tag in its own
 * colour and a teammate with their face. The name is read from the
 * workspace each time, so the badge follows a rename.
 */
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { useContext } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { CHIP_COLORS } from "@/components/base/badges/chip";
import { tagTint } from "@/features/inbox/tags/tag-colors";
import { tagIcon } from "@/features/inbox/tags/tag-icons";
import { avatarColor } from "@/features/shell/avatar-color";
import { cx } from "@/utils/cx";

import { MentionContext } from "./mention-context";
import { isMentionKind, looks, targetOf } from "./mentions";

const pill = "mx-px inline-flex items-center gap-1 rounded-md px-1.5 align-baseline text-body-medium";

export function MentionPill({ node, selected }: NodeViewProps) {
  const known = useContext(MentionContext);
  const kind = String(node.attrs.kind);
  const ref = String(node.attrs.ref);
  const ring = selected && "ring-2 ring-border-focus-ring";

  if (!isMentionKind(kind)) {
    return (
      <NodeViewWrapper as="span" className="inline">
        <span className={cx(pill, CHIP_COLORS.neutral, ring)}>something</span>
      </NodeViewWrapper>
    );
  }

  const target = targetOf(known, kind, ref);
  const name = target?.name ?? looks[kind].word;

  if (kind === "tag") {
    const tint = tagTint(target?.color ?? null);
    const Icon = tagIcon(target?.icon);

    return (
      <NodeViewWrapper as="span" data-mention={kind} className="inline">
        <span style={tint.style} className={cx(pill, tint.className, ring)}>
          <Icon aria-hidden className="size-3.5 shrink-0" />
          {name}
        </span>
      </NodeViewWrapper>
    );
  }

  if (kind === "teammate" && target) {
    return (
      <NodeViewWrapper as="span" data-mention={kind} className="inline">
        <span className={cx(pill, "ps-0.5", CHIP_COLORS.blue, ring)}>
          <Avatar
            size="xs"
            className="size-4 text-[9px]"
            color={avatarColor(target.name)}
            src={target.avatar ?? undefined}
            initials={target.name.charAt(0).toUpperCase()}
          />
          {name}
        </span>
      </NodeViewWrapper>
    );
  }

  const Icon = looks[kind].icon;

  return (
    <NodeViewWrapper as="span" data-mention={kind} className="inline">
      <span className={cx(pill, CHIP_COLORS[looks[kind].tint], ring)}>
        <Icon aria-hidden className="size-3.5 shrink-0" />
        {name}
      </span>
    </NodeViewWrapper>
  );
}
