/*
 * One feature request as the team works it: what was asked, the
 * conversation under it with the team's internal notes set apart, and
 * beside it where it stands, who voted and how much it matters to them,
 * the approval it waits for, and merging or deleting it. Moving it asks
 * whether to tell the people behind it.
 */
import { RiArrowLeftLine, RiDeleteBinLine, RiExternalLinkLine, RiGitMergeLine, RiLockLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useId, useState } from "react";
import { TextArea, TextField } from "react-aria-components";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  addComment,
  commentsQuery,
  deletePost,
  type FeedbackPost,
  feedbackKeys,
  type PostStatus,
  postQuery,
  priorities,
  removeVote,
  statuses,
  updatePost,
  voteFor,
  votersQuery,
} from "@/lib/api/feedback";
import { helpCentersQuery } from "@/lib/api/help-centers";
import { formatDate, formatDateTime } from "@/lib/i18n/format";
import { cx } from "@/utils/cx";

import { MergeDialog, StatusDialog, VoteDialog } from "./post-dialogs";
import { StatusChip, StatusDot } from "./status-chip";

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

function Thread({ workspace, id }: { workspace: string; id: string }) {
  const client = useQueryClient();
  const { data: comments = [] } = useQuery(commentsQuery(workspace, id));
  const [body, setBody] = useState("");
  const [internal, setInternal] = useState(false);
  const titleId = useId();
  const send = useMutation({
    mutationFn: () => addComment(workspace, id, { body, internal }),
    onSuccess: () => {
      setBody("");
      void client.invalidateQueries({ queryKey: feedbackKeys.post(workspace, id) });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <section aria-labelledby={titleId} className="mt-10 flex flex-col gap-4">
      <h2 id={titleId} className="text-headline-semibold text-text-primary">
        Comments
      </h2>
      {comments.length === 0 ? <p className="text-body-2-regular text-text-tertiary">No comments yet.</p> : null}
      <ol className="flex flex-col gap-3">
        {comments.map((c) => (
          <li
            key={c.id}
            className={cx(
              "flex gap-3 rounded-2xl p-3",
              c.internal ? "bg-background-tertiary-default ring-1 ring-inset ring-border-button-default" : "",
              c.parent_id ? "ms-8" : "",
            )}
          >
            <Avatar size="sm" initials={initials(c.author_name)} />
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <p className="flex flex-wrap items-center gap-2 text-body-2-medium text-text-primary">
                {c.author_name}
                {c.from_team ? (
                  <Chip variant="caption" color="blue">
                    Team
                  </Chip>
                ) : null}
                {c.internal ? (
                  <span className="inline-flex items-center gap-1 text-caption-1-medium text-text-tertiary">
                    <RiLockLine aria-hidden className="size-3.5" />
                    Internal note
                  </span>
                ) : null}
                <span className="text-caption-1-regular text-text-tertiary">{formatDateTime(c.created_at)}</span>
              </p>
              <p className="whitespace-pre-line text-body-regular text-text-secondary">{c.body}</p>
            </div>
          </li>
        ))}
      </ol>
      <form
        className="flex flex-col gap-3 rounded-2xl bg-background-primary-default p-3 shadow-xs ring-1 ring-inset ring-border-button-default"
        onSubmit={(event) => {
          event.preventDefault();
          send.mutate();
        }}
      >
        <Textarea
          aria-label={internal ? "Internal note" : "Reply"}
          placeholder={internal ? "Only the team sees this note" : "Reply to everyone following this request"}
          value={body}
          onChange={setBody}
          rows={3}
        />
        <div className="flex items-center justify-between gap-3">
          <Switch size="sm" isSelected={internal} onChange={setInternal}>
            Internal note
          </Switch>
          <Button type="submit" size="small" loading={send.isPending} disabled={body.trim() === ""}>
            {internal ? "Add note" : "Reply"}
          </Button>
        </div>
      </form>
    </section>
  );
}

function Voters({ workspace, post, onVote }: { workspace: string; post: FeedbackPost; onVote: () => void }) {
  const client = useQueryClient();
  const { data } = useQuery(votersQuery(workspace, post.id));
  const voters = data?.items ?? [];
  const titleId = useId();
  const remove = useMutation({
    mutationFn: (contact: string) => removeVote(workspace, post.id, contact),
    onSuccess: () => client.invalidateQueries({ queryKey: feedbackKeys.post(workspace, post.id) }),
    onError: (error) => toast.error(error.message),
  });

  return (
    <section aria-labelledby={titleId} className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <h3 id={titleId} className="text-body-2-semibold text-text-primary">
          {post.vote_count === 1 ? "1 vote" : `${post.vote_count} votes`}
        </h3>
        <Button size="small" variant="secondary" onClick={onVote}>
          Vote for a customer
        </Button>
      </div>
      <ul className="flex flex-col gap-1">
        {voters.map((v) => (
          <li key={v.contact_id} className="group flex items-center gap-2 rounded-xl px-1 py-1.5">
            <Avatar size="xs" src={v.avatar_url ?? undefined} initials={initials(v.name ?? v.email ?? "?")} />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className="truncate text-body-2-medium text-text-primary">{v.name ?? v.email ?? "Customer"}</span>
              <span className="truncate text-caption-1-regular text-text-tertiary">
                {[v.company, priorities.find((p) => p.id === v.priority)?.name, v.by_team ? "added by the team" : null]
                  .filter(Boolean)
                  .join(" · ")}
              </span>
            </span>
            <IconButton
              size="small"
              icon={RiDeleteBinLine}
              aria-label={`Remove the vote of ${v.name ?? v.email ?? "this customer"}`}
              className="opacity-0 group-hover:opacity-100 focus-visible:opacity-100"
              onClick={() => remove.mutate(v.contact_id)}
            />
          </li>
        ))}
      </ul>
    </section>
  );
}

function Side({
  workspace,
  post,
  onOpen,
  onModerate,
}: {
  workspace: string;
  post: FeedbackPost;
  onOpen: (open: Open) => void;
  onModerate: (moderation: "approved" | "spam") => void;
}) {
  return (
    <aside className="flex w-full shrink-0 flex-col gap-5 rounded-3xl border border-border-button-default bg-background-secondary-default p-4 lg:w-80">
      {post.moderation === "pending" ? (
        <div className="flex flex-col gap-2 rounded-2xl bg-background-primary-default p-3 shadow-xs">
          <p className="text-body-2-medium text-text-primary">Waiting for approval</p>
          <p className="text-body-2-regular text-text-tertiary">Only its author sees it until you approve it.</p>
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
      <Select
        label="Status"
        className="w-full"
        selectedKey={post.status}
        onSelectionChange={(key) =>
          key && key !== post.status ? onOpen({ kind: "status", status: key as PostStatus }) : undefined
        }
      >
        {statuses.map((s) => (
          <SelectItem key={s.id} id={s.id} textValue={s.name}>
            <span className="flex items-center gap-2">
              <StatusDot color={s.color} />
              {s.name}
            </span>
          </SelectItem>
        ))}
      </Select>
      <Voters workspace={workspace} post={post} onVote={() => onOpen({ kind: "vote" })} />
      <dl className="flex flex-col gap-1 border-t border-separator-border pt-4 text-body-2-regular">
        <div className="flex justify-between gap-2">
          <dt className="text-text-tertiary">Posted by</dt>
          <dd className="truncate text-text-secondary">{post.author}</dd>
        </div>
        <div className="flex justify-between gap-2">
          <dt className="text-text-tertiary">Posted</dt>
          <dd className="text-text-secondary">{formatDate(post.created_at)}</dd>
        </div>
      </dl>
      <div className="flex gap-2 border-t border-separator-border pt-4">
        <Button size="small" variant="secondary" leadingIcon={RiGitMergeLine} onClick={() => onOpen({ kind: "merge" })}>
          Merge
        </Button>
        <Button
          size="small"
          variant="danger-secondary"
          leadingIcon={RiDeleteBinLine}
          onClick={() => onOpen({ kind: "delete" })}
        >
          Delete
        </Button>
      </div>
    </aside>
  );
}

export function PostPage({ id }: { id: string }) {
  const workspace = useAccount().workspace.id;
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
    <Panel label="Request" className="h-full w-full flex-1">
      <PageHeader
        title={
          <span className="flex min-w-0 items-center gap-2">
            <Link
              to="/d/feedback"
              aria-label="Back to feature requests"
              className="rounded-md p-1 text-foreground-icon-tertiary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiArrowLeftLine aria-hidden className="size-4" />
            </Link>
            <span className="text-text-tertiary">#{post.number}</span>
            <span className="truncate">{post.title}</span>
            <StatusChip post={post} />
          </span>
        }
      >
        {live ? (
          <a
            href={live}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 rounded-lg px-2 py-1.5 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiExternalLinkLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
            View
          </a>
        ) : null}
        <Button size="small" onClick={() => save.mutate()} disabled={!changed} loading={save.isPending}>
          Save
        </Button>
      </PageHeader>
      <ScrollArea>
        <div className="mx-auto flex w-full max-w-[64rem] flex-col gap-6 px-6 pt-8 pb-16 lg:flex-row lg:items-start">
          <div className="flex min-w-0 flex-1 flex-col">
            <TextField aria-label="Title" value={title} onChange={setTitle} maxLength={150}>
              <TextArea rows={1} className={cx(bareField, "text-title-1-medium tracking-tight text-text-primary")} />
            </TextField>
            <TextField aria-label="Details" value={body} onChange={setBody}>
              <TextArea
                rows={3}
                placeholder="Add details"
                className={cx(bareField, "mt-4 text-body-regular whitespace-pre-line text-text-secondary")}
              />
            </TextField>
            <Thread workspace={workspace} id={id} />
          </div>
          <Side
            workspace={workspace}
            post={post}
            onOpen={setOpen}
            onModerate={(moderation) => change.mutate(() => updatePost(workspace, id, { moderation }))}
          />
        </div>
      </ScrollArea>
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
    </Panel>
  );
}
