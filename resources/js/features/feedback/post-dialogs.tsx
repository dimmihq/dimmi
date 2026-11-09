/*
 * The request page's dialogs. Moving a request can email its voters and
 * author once, with a note, and a shipped request links the changelog
 * update that shipped it. A teammate votes for a customer saying how much
 * it matters to them. Merging folds a duplicate into the request kept.
 */
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { entriesQuery } from "@/lib/api/changelog";
import {
  type FeedbackPost,
  type PostStatus,
  type Priority,
  postsQuery,
  priorities,
  type StatusChange,
  statuses,
} from "@/lib/api/feedback";

import { CustomerPicker, type Picked } from "./customer-picker";

const tallModal = "h-[min(640px,calc(100dvh-3rem))]";

function useBusy(): [boolean, (work: () => Promise<unknown>, done: () => void) => void] {
  const [busy, setBusy] = useState(false);
  return [
    busy,
    (work, done) => {
      setBusy(true);
      work()
        .then(done)
        .finally(() => setBusy(false));
    },
  ];
}

export function StatusDialog({
  workspace,
  post,
  status,
  onMove,
  onClose,
}: {
  workspace: string;
  post: FeedbackPost;
  status: PostStatus;
  onMove: (change: StatusChange) => Promise<unknown>;
  onClose: () => void;
}) {
  const [notify, setNotify] = useState(true);
  const [message, setMessage] = useState("");
  const [entry, setEntry] = useState("");
  const [busy, run] = useBusy();
  const entries = useQuery(entriesQuery(workspace, post.help_center_id, { status: "published", page: 1 }));
  const name = statuses.find((s) => s.id === status)?.name ?? status;
  const people = post.vote_count === 1 ? "1 voter" : `${post.vote_count} voters`;

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title={`Move to ${name}`} description={`#${post.number} ${post.title}`} />
      <ModalBody className="flex flex-col gap-4 pb-4">
        {status === "shipped" ? (
          <Select
            label="Changelog update"
            className="w-full"
            placeholder="Link the update that shipped it"
            selectedKey={entry || null}
            onSelectionChange={(key) => setEntry(key === null ? "" : String(key))}
          >
            {(entries.data?.data ?? []).map((e) => (
              <SelectItem key={e.id} id={e.id} textValue={e.title}>
                {e.title}
              </SelectItem>
            ))}
          </Select>
        ) : null}
        <Checkbox isSelected={notify} onChange={setNotify}>
          {`Email ${people} and whoever posted it`}
        </Checkbox>
        {notify ? (
          <Textarea
            label="Note"
            placeholder="Optional: what changed, and what comes next."
            value={message}
            onChange={setMessage}
            rows={4}
          />
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={busy}
          onClick={() =>
            run(() => onMove({ status, notify, message, ...(entry ? { changelog_entry_id: entry } : {}) }), onClose)
          }
        >
          {`Move to ${name}`}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

export function VoteDialog({
  workspace,
  onVote,
  onClose,
}: {
  workspace: string;
  onVote: (vote: { contact_id: string; priority: Priority }) => Promise<unknown>;
  onClose: () => void;
}) {
  const [customer, setCustomer] = useState<Picked | null>(null);
  const [priority, setPriority] = useState<Priority>("nice");
  const [busy, run] = useBusy();

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title="Vote for a customer" description="Count a request you heard from a customer." />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <CustomerPicker workspace={workspace} picked={customer} onPick={setCustomer} />
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
        <Button
          loading={busy}
          disabled={!customer}
          onClick={() => (customer ? run(() => onVote({ contact_id: customer.id, priority }), onClose) : undefined)}
        >
          Add vote
        </Button>
      </ModalFooter>
    </Modal>
  );
}

export function MergeDialog({
  workspace,
  post,
  onMerge,
  onClose,
}: {
  workspace: string;
  post: FeedbackPost;
  onMerge: (target: string) => Promise<unknown>;
  onClose: () => void;
}) {
  const [q, setQ] = useState("");
  const [target, setTarget] = useState<FeedbackPost | null>(null);
  const [busy, run] = useBusy();
  const found = useQuery({
    ...postsQuery(workspace, post.help_center_id, { sort: "top", page: 1, query: q || undefined }),
    enabled: workspace !== "",
  });
  const choices = (found.data?.data ?? []).filter((p) => p.id !== post.id).slice(0, 8);

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title="Merge into another request"
        description="Votes and comments move over; this one points readers there."
      />
      <ModalBody className="flex flex-col gap-3 pb-4">
        <Input label="Request to keep" placeholder="Search requests" value={q} onChange={setQ} />
        <ul aria-label="Requests to keep" className="flex flex-col gap-1">
          {choices.map((p) => (
            <li key={p.id}>
              <button
                type="button"
                aria-pressed={target?.id === p.id}
                onClick={() => setTarget(p)}
                className="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-start text-body-2-regular hover:bg-background-secondary-hover aria-pressed:bg-background-secondary-default aria-pressed:ring-1 aria-pressed:ring-border-button-default"
              >
                <span className="text-text-tertiary">#{p.number}</span>
                <span className="min-w-0 flex-1 truncate">{p.title}</span>
                <span className="text-caption-1-regular text-text-tertiary tabular-nums">{p.vote_count} votes</span>
              </button>
            </li>
          ))}
        </ul>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={busy}
          disabled={!target}
          onClick={() => (target ? run(() => onMerge(target.id), onClose) : undefined)}
        >
          Merge
        </Button>
      </ModalFooter>
    </Modal>
  );
}
