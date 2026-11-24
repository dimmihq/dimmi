/*
 * The mark beside a mention's name, in the list and in the text: a
 * teammate's face, a tag in its own colour, an agent's square, and for
 * everything else the kind's icon on its tint.
 */
import { Avatar } from "@/components/base/avatar/avatar";
import { CHIP_COLORS } from "@/components/base/badges/chip";
import { AgentMark } from "@/features/agents/agent-mark";
import { TagIcon } from "@/features/inbox/tags/tag-chip";
import { avatarColor } from "@/features/shell/avatar-color";
import { cx } from "@/utils/cx";

import { looks, type MentionKind, type Named } from "./mentions";

export function MentionMark({ kind, target }: { kind: MentionKind; target: Named | undefined }) {
  if (kind === "teammate" && target) {
    return (
      <Avatar
        size="sm"
        color={avatarColor(target.name)}
        src={target.avatar ?? undefined}
        initials={target.name.charAt(0).toUpperCase()}
      />
    );
  }
  if (kind === "agent" && target) {
    return <AgentMark agent={{ id: target.id, name: target.name, is_default: target.isDefault }} size="sm" />;
  }
  if (kind === "tag" && target) {
    return <TagIcon tag={{ color: target.color ?? null, icon: target.icon ?? null }} className="size-6" />;
  }
  const Icon = looks[kind].icon;

  return (
    <span
      aria-hidden
      className={cx("flex size-6 shrink-0 items-center justify-center rounded-md", CHIP_COLORS[looks[kind].tint])}
    >
      <Icon className="size-4" />
    </span>
  );
}
