/*
 * Topics are the lists people subscribe to, such as product updates or
 * webinars, so leaving one never means leaving everything. The page
 * lists them in a table: what each covers, how people get on it, how
 * many it reaches now, how many left it and how many broadcasts went
 * out under it. A row opens the topic to rename, describe or archive
 * it. Whether people opt in or are in until they opt out is chosen when
 * the topic is made and never changes after, since people chose under
 * it. An archived topic keeps every choice people made.
 */
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ColumnDef } from "@tanstack/react-table";
import { useId, useState } from "react";

import { DataTable } from "@/components/application/data-table/data-table";
import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { toast } from "@/components/base/toast/toast";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  type BroadcastTopic,
  broadcastTopicsQuery,
  createBroadcastTopic,
  updateBroadcastTopic,
} from "@/lib/api/broadcasts";
import { countOf } from "@/lib/i18n/format";

type Consent = BroadcastTopic["consent"];

const consentLook: Record<Consent, { chip: string; title: string; detail: string }> = {
  opt_out: {
    chip: "Opt out",
    title: "Everyone, until they opt out",
    detail: "People get it unless they unsubscribe from this topic.",
  },
  opt_in: {
    chip: "Opt in",
    title: "Only people who opt in",
    detail: "Nobody gets it until they subscribe to this topic.",
  },
};

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : "That didn't save. Try again.";
}

/*
 * TopicModal makes a topic, or changes the name and description of one.
 */
function TopicModal({
  workspace,
  topic,
  onClose,
  onArchive,
}: {
  workspace: string;
  topic: BroadcastTopic | null;
  onClose: () => void;
  onArchive: (topic: BroadcastTopic) => void;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(topic?.name ?? "");
  const [description, setDescription] = useState(topic?.description ?? "");
  const [consent, setConsent] = useState<Consent>(topic?.consent ?? "opt_out");
  const save = useMutation({
    mutationFn: () =>
      topic
        ? updateBroadcastTopic(workspace, topic.id, { name, description })
        : createBroadcastTopic(workspace, { name, description, consent }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: broadcastTopicsQuery(workspace).queryKey });
      toast.success(topic ? "Topic saved." : "Topic created.");
      onClose();
    },
  });
  const form = useId();
  const unchanged = topic !== null && name === topic.name && description === topic.description;

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={topic ? "Edit topic" : "New topic"}
        description="People see the name and description on the page where they choose what to hear about."
      />
      <ModalBody>
        <form
          id={form}
          className="flex flex-col gap-4 pb-2"
          onSubmit={(event) => {
            event.preventDefault();
            if (name.trim() !== "" && !unchanged) {
              save.mutate();
            }
          }}
        >
          <Input label="Name" autoFocus maxLength={80} value={name} onChange={setName} placeholder="Product updates" />
          <Input
            label="Description"
            maxLength={300}
            value={description}
            onChange={setDescription}
            placeholder="New features and improvements, about once a month."
          />
          {topic ? (
            <div className="flex flex-col gap-1">
              <Label>Who gets it</Label>
              <p className="text-body-2-regular text-text-secondary">
                {consentLook[topic.consent].title}. This can't change, since people chose under it.
              </p>
            </div>
          ) : (
            <div className="flex flex-col gap-1">
              <Label>Who gets it</Label>
              <RadioGroup
                aria-label="Who gets it"
                value={consent}
                onChange={(next) => setConsent(next as Consent)}
                className="gap-2"
              >
                {(Object.keys(consentLook) as Consent[]).map((id) => (
                  <RadioCard key={id} value={id} title={consentLook[id].title} description={consentLook[id].detail} />
                ))}
              </RadioGroup>
              <p className="text-caption-1-regular text-text-tertiary">You can't change this later.</p>
            </div>
          )}
          {save.error ? <p className="text-body-2-regular text-text-error-primary">{errorText(save.error)}</p> : null}
        </form>
      </ModalBody>
      <ModalFooter>
        {topic ? (
          <Button variant="secondary" className="me-auto" onClick={() => onArchive(topic)}>
            Archive
          </Button>
        ) : null}
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form={form} loading={save.isPending} disabled={name.trim() === "" || unchanged}>
          {topic ? "Save" : "Create topic"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

function Count({ value, one, many }: { value: number; one: string; many: string }) {
  return (
    <span className="whitespace-nowrap text-body-2-regular text-text-primary tabular-nums">
      {countOf(value, one, many)}
    </span>
  );
}

const columns: ColumnDef<BroadcastTopic>[] = [
  {
    id: "topic",
    header: "Topic",
    cell: ({ row }) => (
      <span className="flex min-w-0 max-w-96 flex-col">
        <span className="truncate text-body-medium text-text-primary">{row.original.name}</span>
        <span className="truncate text-body-2-regular text-text-tertiary">
          {row.original.description || "No description"}
        </span>
      </span>
    ),
  },
  {
    id: "consent",
    header: "Who gets it",
    cell: ({ row }) => (
      <Chip variant="caption" color={row.original.consent === "opt_in" ? "blue" : "neutral"}>
        {consentLook[row.original.consent].chip}
      </Chip>
    ),
  },
  {
    id: "subscribed",
    header: "Subscribed",
    cell: ({ row }) => <Count value={row.original.subscribed_count} one="person" many="people" />,
  },
  {
    id: "unsubscribed",
    header: "Unsubscribed",
    cell: ({ row }) => <Count value={row.original.unsubscribed_count} one="person" many="people" />,
  },
  {
    id: "broadcasts",
    header: "Broadcasts",
    cell: ({ row }) => <Count value={row.original.broadcast_count} one="broadcast" many="broadcasts" />,
  },
];

const widths = { topic: "w-[380px]", consent: "w-[140px]", subscribed: "w-[160px]", unsubscribed: "w-[160px]" };

type Editing = { topic: BroadcastTopic | null } | null;

export function TopicsPanel({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const { data: topics = [], isPending } = useQuery(broadcastTopicsQuery(workspace));
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<Editing>(null);
  const [archiving, setArchiving] = useState<BroadcastTopic | null>(null);
  const needle = query.trim().toLowerCase();
  const rows = topics.filter((t) => needle === "" || `${t.name} ${t.description}`.toLowerCase().includes(needle));
  const archive = useMutation({
    mutationFn: (topic: BroadcastTopic) =>
      updateBroadcastTopic(workspace, topic.id, { name: topic.name, description: topic.description, archived: true }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: broadcastTopicsQuery(workspace).queryKey });
      toast.success("Topic archived.");
    },
    onError: (error) => toast.error(errorText(error)),
  });

  return (
    <Panel label="Topics" ground className="h-full w-full flex-1">
      <PageHeader title="Topics">
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setEditing({ topic: null })}>
          New topic
        </Button>
      </PageHeader>
      <ScrollArea ground>
        <div className="flex h-full flex-col gap-4 px-4 pb-4 animate-page-in">
          <p className="max-w-2xl text-body-2-regular text-text-secondary">
            Topics are the lists people can leave one at a time. Every broadcast email links to a page where they
            choose. A broadcast with no topic reaches everyone who hasn't unsubscribed from all.
          </p>
          <div className="min-h-[420px] flex-1">
            <DataTable
              aria-label="Topics"
              data={rows}
              columns={columns}
              getRowId={(topic) => topic.id}
              total={countOf(rows.length, "topic", "topics")}
              toolbar={
                <Input
                  aria-label="Search topics"
                  placeholder="Search topics"
                  value={query}
                  onChange={setQuery}
                  className="w-64"
                />
              }
              page={1}
              pageCount={1}
              onPageChange={() => undefined}
              onRowAction={(id) => {
                const topic = topics.find((t) => t.id === id);
                if (topic) {
                  setEditing({ topic });
                }
              }}
              loading={isPending}
              widths={widths}
              empty={
                needle ? "No topic matches." : "No topics yet. Create one to let people pick what they hear about."
              }
            />
          </div>
        </div>
      </ScrollArea>
      {editing ? (
        <TopicModal
          key={editing.topic?.id ?? "new"}
          workspace={workspace}
          topic={editing.topic}
          onClose={() => setEditing(null)}
          onArchive={(topic) => {
            setEditing(null);
            setArchiving(topic);
          }}
        />
      ) : null}
      <AlertDialog
        isOpen={archiving !== null}
        onOpenChange={(open) => (open ? undefined : setArchiving(null))}
        title={`Archive ${archiving?.name ?? "the topic"}?`}
        description="New broadcasts can no longer use it. Every choice people made under it is kept."
        confirmLabel="Archive"
        onConfirm={() => (archiving ? archive.mutateAsync(archiving) : undefined)}
      />
    </Panel>
  );
}
