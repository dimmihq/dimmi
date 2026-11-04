/*
 * What keeps two teammates from answering the same customer at once:
 * who else has the conversation open, and whether the customer is
 * typing. Both follow the conversation's channel while on screen and
 * show nothing when there is nothing to tell.
 */
import { Avatar } from "@/components/base/avatar/avatar";
import { channels } from "@/lib/realtime/keys";
import { usePresence } from "@/lib/realtime/use-channel";
import { cx } from "@/utils/cx";

import { typingLabel, viewingLabel } from "./labels";
import { type Typist, useTyping } from "./typing";

function initialsOf(name: string): string {
  return name.trim().charAt(0).toUpperCase() || "?";
}

/*
 * ConversationViewers shows the teammates who have the conversation
 * open, the person reading left out however many tabs they use.
 */
export function ConversationViewers({
  conversation,
  self,
  className,
}: {
  conversation: string;
  self: string;
  className?: string;
}) {
  const others = usePresence(channels.conversation(conversation)).filter((listener) => listener.user !== self);
  if (others.length === 0) {
    return null;
  }

  const label = viewingLabel(others);

  return (
    <div role="status" aria-label={label} className={cx("flex items-center gap-2", className)}>
      <div className="-space-x-1.5 flex">
        {others.slice(0, 3).map((listener) => (
          <Avatar
            key={listener.user}
            size="xs"
            initials={initialsOf(listener.name)}
            src={listener.avatarUrl ?? undefined}
            alt={listener.name}
            className="ring-2 ring-background-primary-default"
          />
        ))}
      </div>
      <span className="text-caption-1-medium text-text-secondary">{label}</span>
    </div>
  );
}

/*
 * TypingIndicator shows who types in the conversation. nameOf names a
 * typist; by default every typist is the customer.
 */
export function TypingIndicator({
  conversation,
  nameOf = () => "The customer",
  className,
}: {
  conversation: string;
  nameOf?: (typist: Typist) => string;
  className?: string;
}) {
  const typists = useTyping(conversation);
  if (typists.length === 0) {
    return null;
  }

  return (
    <p role="status" className={cx("text-caption-1-medium text-text-tertiary", className)}>
      {typingLabel(typists, nameOf)}
    </p>
  );
}
