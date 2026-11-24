/*
 * One feature request as the team works it, laid out the way a ticket
 * is: what was asked and the conversation under it in the middle, with
 * the team's internal notes set apart and the reply box at the foot; and
 * beside it, in a panel that resizes, where it stands, who voted and how
 * much it matters to them, the approval it waits for, the conversations
 * it came up in, and merging or deleting it. Moving it asks whether to
 * tell the people behind it.
 */
import {
  RiAddLine,
  RiArrowDownSLine,
  RiArrowLeftLine,
  RiArrowUpSLine,
  RiDeleteBinLine,
  RiExternalLinkLine,
  RiGitMergeLine,
  RiLockLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { TextArea, TextField } from "react-aria-components";

import { Accordion } from "@/components/base/accordion/accordion";
import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { NoteComposer } from "@/features/inbox/composer";
import { onRowHover } from "@/features/inbox/customer/quiet";
import { Row, Section } from "@/features/inbox/customer/rows";
import { MessageBody } from "@/features/inbox/message-body";
import { useViewing } from "@/features/operator/viewing";
import { useAccount } from "@/features/settings/use-account";
import { avatarColor } from "@/features/shell/avatar-color";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { DetailsColumn, TicketSplit } from "@/features/tickets/ticket-thread";
import {
  addComment,
  boardsQuery,
  commentsQuery,
  deletePost,
  type FeedbackPost,
  feedbackKeys,
  type PostStatus,
  postQuery,
  priorities,
  removeVote,
  updatePost,
  voteFor,
  votersQuery,
} from "@/lib/api/feedback";
import { helpCentersQuery } from "@/lib/api/help-centers";
import { formatDate, formatDateTime } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";
import { LinkedConversations } from "./linked-conversations";
import { MergeDialog, StatusDialog, VoteDialog } from "./post-dialogs";
import { StatusChip, StatusDot } from "./status-chip";
import { useStatuses } from "./use-statuses";

type Open = { kind: "status"; status: PostStatus } | { kind: "vote" } | { kind: "merge" } | { kind: "delete" } | null;

const bareField =
  "w-full resize-none border-0 bg-transparent p-0 outline-none [field-sizing:content] placeholder:text-text-placeholder";

function initials(name: string): string {
  return name
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
}

/*
 * usePost loads the request and keeps every list in step with it.
 */
function usePost(workspace: string, id: string) {
  const client = useQueryClient();
  const { data: post } = useQuery(postQuery(workspace, id));
  const settle = (next: FeedbackPost) => {
    client.setQueryData(postQuery(workspace, id).queryKey, next);
    void client.invalidateQueries({ queryKey: feedbackKeys.post(workspace, id) });
    void client.invalidateQueries({ queryKey: feedbackKeys.posts(workspace, next.help_center_id) });
  };
  return { post, settle };
}

/*
 * shownVoters is how many voters show before "Show all".
 */
const shownVoters = 6;

/*
 * Comments are the conversation under the request, oldest first, with
 * the team's internal notes set apart by a note's edge.
 */
function Comments({ workspace, id }: { workspace: string; id: string }) {
  const { data: comments = [] } = useQuery(commentsQuery(workspace, id));
  if (comments.length === 0) {
    return <p className="text-body-2-regular text-text-tertiary">No comments yet.</p>;
  }

  return (
    <ol aria-label="Comments" className="flex flex-col gap-4">
      {comments.map((c) => (
        <li key={c.id} className={cx("flex gap-3", c.parent_id && "ms-9")}>
          <Avatar size="sm" color={avatarColor(c.author_name)} initials={initials(c.author_name)} />
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <p className="flex flex-wrap items-center gap-2 text-body-2-medium text-text-primary">
              {c.author_name}
              {c.from_team ? (
                <Chip variant="caption" color="blue" className="py-0">
                  Team
                </Chip>
              ) : null}
              <span className="text-caption-1-regular text-text-tertiary">{formatDateTime(c.created_at)}</span>
            </p>
            {c.internal ? (
              <div className="flex w-fit max-w-full flex-col gap-1 rounded-2xl border border-status-yellow-text/40 bg-background-primary-default px-3 py-2 shadow-card">
                <MessageBody text={c.body} className="text-body-regular text-text-primary" />
                <span className="flex items-center gap-1 text-caption-1-regular text-text-tertiary">
                  <RiLockLine aria-hidden className="size-3.5" />
                  Only your team sees this
                </span>
              </div>
            ) : c.from_team ? (
              <MessageBody text={c.body} className="text-body-regular text-text-secondary" />
            ) : (
              <p className="whitespace-pre-line text-body-regular text-text-secondary">{c.body}</p>
            )}
          </div>
        </li>
      ))}
    </ol>
  );
}

/*
 * ReplyBox sits at the foot of the request, the composer conversations
 * and tickets use: a reply everyone following the request reads, or a
 * note only the team sees.
 */
function ReplyBox({ workspace, id }: { workspace: string; id: string }) {
  const client = useQueryClient();
  const send = (internal: boolean) => (body: string) =>
    addComment(workspace, id, { body, internal }).then(() =>
      client.invalidateQueries({ queryKey: feedbackKeys.post(workspace, id) }),
    );

  return (
    <div className="px-3 pb-3">
      <NoteComposer
        onAdd={send(true)}
        reply={{
          onReply: send(false),
          to: "To everyone following this request",
          placeholder: "Write a reply to everyone following this request…",
        }}
      />
    </div>
  );
}

/*
 * Voters lists who voted and how much it matters to them, the first few
 * until the teammate asks for the rest; removing a vote shows on hover.
 */
function Voters({ workspace, post, onVote }: { workspace: string; post: FeedbackPost; onVote: () => void }) {
  const client = useQueryClient();
  const { data } = useQuery(votersQuery(workspace, post.id));
  const [all, setAll] = useState(false);
  const voters = data?.items ?? [];
  const remove = useMutation({
    mutationFn: (contact: string) => removeVote(workspace, post.id, contact),
    onSuccess: () => client.invalidateQueries({ queryKey: feedbackKeys.post(workspace, post.id) }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <Section title="Votes" count={post.vote_count}>
      <div className="flex flex-col items-start">
        <div className="flex min-h-8 items-center">
          <LinkButton size="xs" variant="secondary" leadingIcon={RiAddLine} onClick={onVote}>
            Vote for a customer
          </LinkButton>
        </div>
        <ul className="-mx-1.5 flex w-[calc(100%+0.75rem)] flex-col">
          {(all ? voters : voters.slice(0, shownVoters)).map((v) => {
            const name = v.name ?? v.email ?? "Customer";

            return (
              <li key={v.contact_id} className="group/row flex min-h-11 items-center gap-2 rounded-lg px-1.5">
                <Avatar size="xs" src={v.avatar_url ?? undefined} color={avatarColor(name)} initials={initials(name)} />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-body-2-medium text-text-primary">{name}</span>
                  <span className="truncate text-caption-1-regular text-text-tertiary">
                    {[
                      v.company,
                      priorities.find((p) => p.id === v.priority)?.name,
                      v.by_team ? "Added by the team" : null,
                    ]
                      .filter(Boolean)
                      .join(" · ")}
                  </span>
                </span>
                <Button
                  variant="secondary"
                  size="xs"
                  iconOnly
                  leadingIcon={RiDeleteBinLine}
                  aria-label={`Remove the vote of ${name}`}
                  className={onRowHover}
                  onClick={() => remove.mutate(v.contact_id)}
                />
              </li>
            );
          })}
        </ul>
        {voters.length > shownVoters ? (
          <LinkButton
            variant="secondary"
            size="xs"
            leadingIcon={all ? RiArrowUpSLine : RiArrowDownSLine}
            aria-expanded={all}
            className="mt-1"
            onClick={() => setAll((was) => !was)}
          >
            {all ? "Show less" : `Show all ${voters.length}`}
          </LinkButton>
        ) : null}
      </div>
    </Section>
  );
}

/*
 * Details is the column beside the request, built like a conversation's
 * details: where it stands and who posted it as rows, then who voted
 * and the conversations it came up in, and merging or deleting it at
 * the end.
 */
function Details({
  workspace,
  post,
  onOpen,
  onModerate,
  onMoveBoard,
}: {
  workspace: string;
  post: FeedbackPost;
  onOpen: (open: Open) => void;
  onModerate: (moderation: "approved" | "spam") => void;
  onMoveBoard: (board: string) => void;
}) {
  const ordered = useStatuses(workspace);
  const { data: boards = [] } = useQuery(boardsQuery(workspace, post.help_center_id));

  return (
    <Accordion
      variant="plain"
      size="sm"
      allowsMultipleExpanded
      defaultExpandedKeys={["Votes", "Conversations"]}
      className="pt-1"
    >
      <div className="flex flex-col gap-3">
        {post.moderation === "pending" ? (
          <div className="flex flex-col gap-2 rounded-xl border border-border-button-default p-3">
            <div className="flex flex-col">
              <p className="text-body-2-medium text-text-primary">Waiting for approval</p>
              <p className="text-body-2-regular text-text-tertiary">Only its author sees it until you approve it.</p>
            </div>
            <div className="flex gap-2">
              <Button size="small" onClick={() => onModerate("approved")}>
                Approve
              </Button>
              <Button size="small" variant="secondary" onClick={() => onModerate("spam")}>
                Spam
              </Button>
            </div>
          </div>
        ) : null}
        <dl className="flex flex-col">
          <Row label="Status">
            <Select
              aria-label="Status"
              size="sm"
              variant="quiet"
              selectedKey={post.status}
              onSelectionChange={(key) =>
                key && key !== post.status ? onOpen({ kind: "status", status: key as PostStatus }) : undefined
              }
            >
              {ordered.map((s) => (
                <SelectItem key={s.id} id={s.id} textValue={s.name}>
                  <span className="flex items-center gap-2">
                    <StatusDot color={s.color} />
                    {s.name}
                  </span>
                </SelectItem>
              ))}
            </Select>
          </Row>
          {boards.length > 1 ? (
            <Row label="Board">
              <Select
                aria-label="Board"
                size="sm"
                variant="quiet"
                selectedKey={post.board_id ?? boards[0]?.id}
                onSelectionChange={(key) => (key && key !== post.board_id ? onMoveBoard(String(key)) : undefined)}
              >
                {boards.map((b) => (
                  <SelectItem key={b.id} id={b.id} textValue={b.name}>
                    {b.name}
                  </SelectItem>
                ))}
              </Select>
            </Row>
          ) : null}
          <Row label="Posted by">
            <span className="truncate">{post.author}</span>
          </Row>
          <Row label="Posted">{formatDate(post.created_at)}</Row>
        </dl>
      </div>
      <Voters workspace={workspace} post={post} onVote={() => onOpen({ kind: "vote" })} />
      <Section title="Conversations">
        <LinkedConversations workspace={workspace} postId={post.id} />
      </Section>
      <div className="flex items-center gap-4 pt-4">
        <LinkButton
          size="xs"
          variant="secondary"
          leadingIcon={RiGitMergeLine}
          onClick={() => onOpen({ kind: "merge" })}
        >
          Merge into another
        </LinkButton>
        <LinkButton
          size="xs"
          variant="secondary"
          leadingIcon={RiDeleteBinLine}
          className="text-text-error-primary"
          onClick={() => onOpen({ kind: "delete" })}
        >
          Delete
        </LinkButton>
      </div>
    </Accordion>
  );
}

export function PostPage({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
  useViewing("feature_request", id);
  const navigate = useNavigate();
  const { post, settle } = usePost(workspace, id);
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.id === post?.help_center_id);
  const [open, setOpen] = useState<Open>(null);
  const [title, setTitle] = useState<string | null>(null);
  const [body, setBody] = useState<string | null>(null);

  useEffect(() => {
    if (post && title === null) {
      setTitle(post.title);
      setBody(post.body_markdown);
    }
  }, [post, title]);

  const save = useMutation({
    mutationFn: () => updatePost(workspace, id, { title: title ?? "", body_markdown: body ?? "" }),
    onSuccess: (next) => {
      settle(next);
      toast.success("Saved");
    },
    onError: (error) => toast.error(error.message),
  });
  const change = useMutation({
    mutationFn: (fn: () => Promise<FeedbackPost>) => fn(),
    onSuccess: settle,
    onError: (error) => toast.error(error.message),
  });

  if (!post || title === null || body === null) {
    return (
      <Panel label="Request" className="h-full flex-1">
        <PageHeader title="Request" />
      </Panel>
    );
  }
  const changed = title !== post.title || body !== post.body_markdown;
  const live = center?.settings.feedback_board ? `${center.url}${post.path}` : null;

  return (
    <>
      <TicketSplit
        thread={
          <Panel label={`Request ${post.number}`} className="flex h-full min-w-0 flex-1 flex-col">
            <header className="flex h-12 shrink-0 items-center gap-2 ps-2 pe-2.5 pt-[7px]">
              <Link
                to="/d/feedback"
                aria-label="Back to feature requests"
                className="grid size-7 shrink-0 place-items-center rounded-lg text-foreground-icon-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <RiArrowLeftLine aria-hidden className="size-4 rtl:-scale-x-100" />
              </Link>
              <span className="shrink-0 text-body-2-medium text-text-tertiary">#{post.number}</span>
              <span className="min-w-0 truncate text-body-medium text-text-primary">{post.title}</span>
              <span className="ms-auto flex shrink-0 items-center gap-2">
                <StatusChip post={post} />
                {live ? (
                  <LinkButton
                    href={live}
                    target="_blank"
                    rel="noopener noreferrer"
                    size="xs"
                    variant="secondary"
                    leadingIcon={RiExternalLinkLine}
                  >
                    View
                  </LinkButton>
                ) : null}
                {changed ? (
                  <Button size="small" onClick={() => save.mutate()} loading={save.isPending}>
                    Save
                  </Button>
                ) : null}
              </span>
            </header>
            <ScrollArea className="flex-1">
              <div className="mx-auto flex w-full max-w-[760px] flex-col gap-6 px-4 py-5">
                <div className="flex flex-col gap-3">
                  <TextField aria-label="Title" value={title} onChange={setTitle} maxLength={150}>
                    <TextArea
                      rows={1}
                      className={cx(bareField, "text-title-2-medium tracking-tight text-text-primary")}
                    />
                  </TextField>
                  <TextField aria-label="Details" value={body} onChange={setBody}>
                    <TextArea
                      rows={2}
                      placeholder="Add details"
                      className={cx(bareField, "text-body-regular whitespace-pre-line text-text-secondary")}
                    />
                  </TextField>
                </div>
                <div aria-hidden className="h-px bg-separator-border" />
                <Comments workspace={workspace} id={id} />
              </div>
            </ScrollArea>
            <ReplyBox workspace={workspace} id={id} />
          </Panel>
        }
        details={
          <DetailsColumn>
            <Details
              workspace={workspace}
              post={post}
              onOpen={setOpen}
              onModerate={(moderation) => change.mutate(() => updatePost(workspace, id, { moderation }))}
              onMoveBoard={(board) => change.mutate(() => updatePost(workspace, id, { board_id: board }))}
            />
          </DetailsColumn>
        }
      />
      {open?.kind === "status" ? (
        <StatusDialog
          workspace={workspace}
          post={post}
          status={open.status}
          onMove={(next) => change.mutateAsync(() => updatePost(workspace, id, next))}
          onClose={() => setOpen(null)}
        />
      ) : null}
      {open?.kind === "vote" ? (
        <VoteDialog
          workspace={workspace}
          onVote={(vote) => change.mutateAsync(() => voteFor(workspace, id, vote))}
          onClose={() => setOpen(null)}
        />
      ) : null}
      {open?.kind === "merge" ? (
        <MergeDialog
          workspace={workspace}
          post={post}
          onMerge={async (target) => {
            const kept = await updatePost(workspace, id, { merge_into_id: target });
            settle(post);
            toast.success(`Merged into #${kept.number}`);
            await navigate({ to: "/d/feedback/$post", params: { post: kept.id } });
          }}
          onClose={() => setOpen(null)}
        />
      ) : null}
      <AlertDialog
        isOpen={open?.kind === "delete"}
        onOpenChange={(next) => (next ? undefined : setOpen(null))}
        title={`Delete #${post.number}?`}
        description="It leaves the board with its votes and comments. Nobody is emailed."
        confirmLabel="Delete request"
        onConfirm={async () => {
          await deletePost(workspace, id);
          settle(post);
          await navigate({ to: "/d/feedback" });
        }}
      />
    </>
  );
}
