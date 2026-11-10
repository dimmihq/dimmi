/*
 * Topics are the lists people subscribe to, such as product updates or
 * webinars, so leaving one never means leaving everything. Each is opt
 * in, reaching only people who said yes, or opt out, reaching everyone
 * who has not said no; that choice is fixed once made, since people
 * chose under it. A topic can be renamed, described or archived, and an
 * archived topic keeps every choice people made.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import {
  type BroadcastTopic,
  broadcastTopicsQuery,
  createBroadcastTopic,
  updateBroadcastTopic,
} from "@/lib/api/broadcasts";

const consentNames: Record<BroadcastTopic["consent"], string> = {
  opt_out: "Everyone until they opt out",
  opt_in: "Only people who opt in",
};

function errorText(error: unknown): string {
  return error instanceof Error ? error.message : "That didn't save. Try again.";
}

function TopicRow({ workspace, topic }: { workspace: string; topic: BroadcastTopic }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(topic.name);
  const [description, setDescription] = useState(topic.description);
  const save = useMutation({
    mutationFn: (archived: boolean) => updateBroadcastTopic(workspace, topic.id, { name, description, archived }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: broadcastTopicsQuery(workspace).queryKey }),
  });
  const changed = name !== topic.name || description !== topic.description;

  return (
    <li className="flex flex-col gap-2 border-b border-separator-border py-3 last:border-b-0">
      <div className="flex items-center gap-2">
        <Input aria-label="Topic name" className="min-w-0 flex-1" value={name} onChange={setName} />
        <Chip variant="caption" color={topic.consent === "opt_in" ? "blue" : "gray"}>
          {topic.consent === "opt_in" ? "Opt in" : "Opt out"}
        </Chip>
      </div>
      <Input
        aria-label="Topic description"
        value={description}
        onChange={setDescription}
        placeholder="What it covers"
      />
      <div className="flex items-center justify-end gap-2">
        {save.error ? (
          <span className="text-body-2-regular text-text-error-primary">{errorText(save.error)}</span>
        ) : null}
        <Button variant="secondary" size="small" loading={save.isPending} onClick={() => save.mutate(true)}>
          Archive
        </Button>
        <Button size="small" disabled={!changed || name.trim() === ""} onClick={() => save.mutate(false)}>
          Save
        </Button>
      </div>
    </li>
  );
}

function NewTopic({ workspace }: { workspace: string }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [consent, setConsent] = useState<BroadcastTopic["consent"]>("opt_out");
  const add = useMutation({
    mutationFn: () => createBroadcastTopic(workspace, { name, description, consent }),
    onSuccess: () => {
      setName("");
      setDescription("");
      return queryClient.invalidateQueries({ queryKey: broadcastTopicsQuery(workspace).queryKey });
    },
  });

  return (
    <div className="flex flex-col gap-2 rounded-2xl bg-background-secondary-default p-3">
      <span className="text-body-medium text-text-primary">New topic</span>
      <Input aria-label="New topic name" value={name} onChange={setName} placeholder="Product updates" />
      <Input
        aria-label="New topic description"
        value={description}
        onChange={setDescription}
        placeholder="New features and improvements, about once a month."
      />
      <Select
        aria-label="Who gets it"
        selectedKey={consent}
        onSelectionChange={(key) => key !== null && setConsent(key as BroadcastTopic["consent"])}
      >
        {Object.entries(consentNames).map(([id, label]) => (
          <SelectItem key={id} id={id}>
            {label}
          </SelectItem>
        ))}
      </Select>
      <div className="flex items-center justify-end gap-2">
        {add.error ? <span className="text-body-2-regular text-text-error-primary">{errorText(add.error)}</span> : null}
        <Button size="small" loading={add.isPending} disabled={name.trim() === ""} onClick={() => add.mutate()}>
          Add topic
        </Button>
      </div>
    </div>
  );
}

export function TopicsModal({ workspace, onClose }: { workspace: string; onClose: () => void }) {
  const topics = useQuery(broadcastTopicsQuery(workspace)).data ?? [];

  return (
    <Modal size="wide" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title="Topics"
        description="The lists people can leave one at a time. Every broadcast email links to a page where they choose."
      />
      <ModalBody className="flex flex-col gap-4 pb-2">
        {topics.length === 0 ? (
          <p className="text-body-2-regular text-text-secondary">
            No topics yet. Broadcasts without one reach everyone who hasn't unsubscribed from all.
          </p>
        ) : (
          <ul className="flex flex-col">
            {topics.map((topic) => (
              <TopicRow key={topic.id} workspace={workspace} topic={topic} />
            ))}
          </ul>
        )}
        <NewTopic workspace={workspace} />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Done
        </Button>
      </ModalFooter>
    </Modal>
  );
}
