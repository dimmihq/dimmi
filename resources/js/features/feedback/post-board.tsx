/*
 * Where a request sits beside its votes: the board it is on, which the
 * team moves it between when the help center has more than one, and the
 * conversations it came up in.
 */
import { useQuery } from "@tanstack/react-query";
import { useId } from "react";

import { Select, SelectItem } from "@/components/base/select/select";
import { boardsQuery, type FeedbackPost } from "@/lib/api/feedback";

import { LinkedConversations } from "./linked-conversations";

/*
 * BoardPicker moves the request to another board of its help center. It
 * shows nothing while the help center has one board.
 */
export function BoardPicker({
  workspace,
  post,
  onMove,
}: {
  workspace: string;
  post: FeedbackPost;
  onMove: (board: string) => void;
}) {
  const { data: boards = [] } = useQuery(boardsQuery(workspace, post.help_center_id));
  if (boards.length < 2) {
    return null;
  }

  return (
    <Select
      label="Board"
      className="w-full"
      selectedKey={post.board_id ?? boards[0]?.id}
      onSelectionChange={(key) => (key && key !== post.board_id ? onMove(String(key)) : undefined)}
    >
      {boards.map((b) => (
        <SelectItem key={b.id} id={b.id} textValue={b.name}>
          {b.name}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * PostConversations lists the conversations linked to the request under
 * its own heading.
 */
export function PostConversations({ workspace, post }: { workspace: string; post: FeedbackPost }) {
  const titleId = useId();

  return (
    <section aria-labelledby={titleId} className="flex flex-col gap-2 border-t border-separator-border pt-4">
      <h3 id={titleId} className="text-body-2-semibold text-text-primary">
        Conversations
      </h3>
      <LinkedConversations workspace={workspace} postId={post.id} />
    </section>
  );
}
