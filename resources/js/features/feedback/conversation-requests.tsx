/*
 * The feature requests a conversation asked for, beside its thread. A
 * teammate links the conversation to a request, found by its title or
 * posted right there from the conversation, which counts the customer's
 * vote; when the request ships, they hear about it. Taking a link back
 * keeps the vote.
 */
import { RiAddLine, RiCloseLine, RiThumbUpLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import {
  conversationPostsQuery,
  type FeedbackPost,
  type FeedbackStatus,
  feedbackKeys,
  linkConversation,
  type Priority,
  postsQuery,
  priorities,
  unlinkConversation,
} from "@/lib/api/feedback";
import { helpCentersQuery } from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { StatusDot } from "./status-chip";
import { useStatuses } from "./use-statuses";

function statusOf(list: FeedbackStatus[], post: FeedbackPost): FeedbackStatus | undefined {
  return list.find((s) => s.id === post.status);
}

function Row({
  post,
  status,
  onUnlink,
}: {
  post: FeedbackPost;
  status: FeedbackStatus | undefined;
  onUnlink: () => void;
}) {
  return (
    <li className="flex min-h-9 items-center gap-2 rounded-xl px-2 hover:bg-background-secondary-hover">
      <StatusDot color={status?.color ?? "#a1a1aa"} />
      <Link
        to="/d/feedback/$post"
        params={{ post: post.id }}
        className="min-w-0 flex-1 truncate text-body-2-regular text-text-primary outline-none hover:underline focus-visible:underline"
      >
        {post.title}
      </Link>
      <span className="sr-only">{status?.name}</span>
      <span className="flex shrink-0 items-center gap-1 text-caption-1-medium tabular-nums text-text-tertiary">
        <RiThumbUpLine aria-hidden className="size-3.5" />
        {post.vote_count}
      </span>
      <IconButton size="small" icon={RiCloseLine} aria-label={`Unlink ${post.title}`} onClick={onUnlink} />
    </li>
  );
}

/*
 * ConversationRequests lists the requests linked to the conversation and
 * offers to link another.
 */
export function ConversationRequests({
  workspace,
  conversation,
  subject,
}: {
  workspace: string;
  conversation: string;
  subject: string;
}) {
  const client = useQueryClient();
  const statuses = useStatuses(workspace);
  const [linking, setLinking] = useState(false);
  const { data } = useQuery(conversationPostsQuery(workspace, conversation));
  const posts = data?.items ?? [];
  const unlink = useMutation({
    mutationFn: (post: string) => unlinkConversation(workspace, conversation, post),
    onSuccess: () => client.invalidateQueries({ queryKey: feedbackKeys.conversation(workspace, conversation) }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <div className="flex flex-col gap-1">
      {posts.length === 0 ? (
        <p className="text-body-2-regular text-text-tertiary">No requests linked yet.</p>
      ) : (
        <ul aria-label="Linked feature requests" className="-mx-2 flex flex-col">
          {posts.map((post) => (
            <Row key={post.id} post={post} status={statusOf(statuses, post)} onUnlink={() => unlink.mutate(post.id)} />
          ))}
        </ul>
      )}
      <LinkButton variant="secondary" size="small" leadingIcon={RiAddLine} onClick={() => setLinking(true)}>
        Link to feature request
      </LinkButton>
      {linking ? (
        <LinkRequestDialog
          workspace={workspace}
          conversation={conversation}
          subject={subject}
          linked={posts.map((p) => p.id)}
          onClose={() => setLinking(false)}
        />
      ) : null}
    </div>
  );
}

/*
 * LinkRequestDialog finds a request by its title, or posts a new one
 * from the conversation, and links it with how much it matters.
 */
function LinkRequestDialog({
  workspace,
  conversation,
  subject,
  linked,
  onClose,
}: {
  workspace: string;
  conversation: string;
  subject: string;
  linked: string[];
  onClose: () => void;
}) {
  const client = useQueryClient();
  const statuses = useStatuses(workspace);
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.is_default) ?? centers[0];
  const [q, setQ] = useState(subject);
  const [picked, setPicked] = useState<FeedbackPost | null>(null);
  const [creating, setCreating] = useState(false);
  const [priority, setPriority] = useState<Priority>("nice");
  const found = useQuery({
    ...postsQuery(workspace, center?.id ?? "", { sort: "top", page: 1, query: q.trim() || undefined }),
    enabled: center !== undefined && !creating,
  });
  const link = useMutation({
    mutationFn: () =>
      linkConversation(
        workspace,
        conversation,
        picked ? { post_id: picked.id, priority } : { title: q.trim(), priority },
      ),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: feedbackKeys.conversation(workspace, conversation) });
      if (center) {
        void client.invalidateQueries({ queryKey: feedbackKeys.posts(workspace, center.id) });
      }
      onClose();
    },
    onError: (error) => toast.error(error.message),
  });
  const results = (found.data?.data ?? []).filter((p) => !linked.includes(p.id)).slice(0, 8);
  const ready = picked !== null || (creating && q.trim() !== "");

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(o) => (o ? undefined : onClose())}
    >
      <ModalHeader
        title="Link to feature request"
        description="The customer's vote counts, and they hear when it ships."
      />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <Input
          label={creating ? "New request title" : "Find a request"}
          placeholder={creating ? "Short, like “Export to CSV”" : "Search by title"}
          value={q}
          onChange={(value) => {
            setQ(value);
            setPicked(null);
          }}
          maxLength={150}
          autoFocus
        />
        {creating ? null : (
          <Matches
            results={results}
            statuses={statuses}
            picked={picked}
            onPick={setPicked}
            empty={found.data !== undefined && results.length === 0}
          />
        )}
        <LinkButton
          variant="secondary"
          size="small"
          leadingIcon={creating ? undefined : RiAddLine}
          onClick={() => {
            setCreating(!creating);
            setPicked(null);
          }}
        >
          {creating ? "Search existing requests instead" : "Post a new request from this conversation"}
        </LinkButton>
        <div className="flex flex-col gap-1.5">
          <Label>How much it matters to them</Label>
          <RadioGroup aria-label="How much it matters" value={priority} onChange={(v) => setPriority(v as Priority)}>
            {priorities.map((p) => (
              <RadioCard key={p.id} value={p.id} title={p.name} />
            ))}
          </RadioGroup>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button loading={link.isPending} disabled={!ready} onClick={() => link.mutate()}>
          {creating ? "Post and link" : "Link request"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * Matches lists the requests whose titles match, to pick one.
 */
function Matches({
  results,
  statuses,
  picked,
  onPick,
  empty,
}: {
  results: FeedbackPost[];
  statuses: FeedbackStatus[];
  picked: FeedbackPost | null;
  onPick: (post: FeedbackPost) => void;
  empty: boolean;
}) {
  return (
    <ul aria-label="Matching requests" className="flex flex-col">
      {results.map((post) => {
        const status = statusOf(statuses, post);
        return (
          <li key={post.id}>
            <button
              type="button"
              aria-pressed={picked?.id === post.id}
              onClick={() => onPick(post)}
              className={cx(
                "flex w-full items-center gap-2 rounded-xl px-3 py-2 text-start text-body-2-regular outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                picked?.id === post.id && "bg-background-secondary-default",
              )}
            >
              <StatusDot color={status?.color ?? "#a1a1aa"} />
              <span className="min-w-0 flex-1 truncate">{post.title}</span>
              <span className="shrink-0 text-caption-1-medium tabular-nums text-text-tertiary">
                {post.vote_count} votes
              </span>
            </button>
          </li>
        );
      })}
      {empty ? <li className="px-3 py-2 text-body-2-regular text-text-tertiary">No request matches.</li> : null}
    </ul>
  );
}
