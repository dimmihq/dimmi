/*
 * The workspace's tags, on the server, with how many conversations carry
 * each. Any teammate can add one. Renaming, recolouring and deleting
 * change what every record means, so they show only to people the server
 * lets manage tags, and deleting says how many it takes the tag from.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { parseColor } from "react-aria-components";

import { ColorWell } from "@/components/base/color-picker/color-picker";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { TagDot } from "@/features/inbox/tags/tag-chip";
import { createTag, deleteTag, type ListedTag, tagsQuery, updateTag } from "@/lib/api/tags";
import { callerQuery } from "@/lib/api/workspace";

import { Action, Danger } from "../controls";
import { Empty, Row, Section, SettingsPage } from "../layout";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const swatches = ["#10b981", "#3b82f6", "#8b5cf6", "#ec4899", "#ef4444", "#f59e0b", "#737373"];

function useTagWrites(workspace: string) {
  const queryClient = useQueryClient();

  return () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
}

function countOf(tag: ListedTag): string {
  const count = tag.counts?.conversations ?? 0;

  return count === 1 ? "1 conversation" : `${count} conversations`;
}

function NewTagDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const refresh = useTagWrites(workspace);
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => createTag(workspace, name),
    onSuccess: async () => {
      setName("");
      await refresh();
    },
  });

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          create.reset();
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title="New tag"
      description="Teammates can use it on conversations, people and companies."
      confirmLabel="Create tag"
      isConfirmDisabled={name.trim() === ""}
      onConfirm={() => create.mutateAsync()}
    >
      <Input
        label="Tag name"
        value={name}
        onChange={setName}
        isInvalid={create.error !== null}
        errorMessage={failureText(create.error)}
      />
    </AlertDialog>
  );
}

function EditTagDialog({ tag, onClose }: { tag: ListedTag; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const refresh = useTagWrites(workspace);
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(parseColor(tag.color ?? "#737373"));
  const save = useMutation({
    mutationFn: () => updateTag(workspace, tag.id, { name, color: color.toString("hex").toLowerCase() }),
    onSuccess: refresh,
  });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title={`Edit ${tag.name}`}
      confirmLabel="Save"
      isConfirmDisabled={name.trim() === ""}
      onConfirm={() => save.mutateAsync()}
    >
      <div className="flex items-start gap-3">
        <Input
          label="Tag name"
          className="min-w-0 flex-1"
          value={name}
          onChange={setName}
          isInvalid={save.error !== null}
          errorMessage={failureText(save.error)}
        />
        <div className="pt-7">
          <ColorWell
            aria-label="Tag colour"
            value={color}
            onChange={setColor}
            swatches={swatches}
            placement="bottom end"
            wellClassName="size-5"
          />
        </div>
      </div>
    </AlertDialog>
  );
}

function DeleteTagDialog({ tag, onClose }: { tag: ListedTag; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const refresh = useTagWrites(workspace);
  const remove = useMutation({ mutationFn: () => deleteTag(workspace, tag.id), onSuccess: refresh });
  const counts = tag.counts;

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      title={`Delete ${tag.name}?`}
      description={`It comes off ${counts?.conversations ?? 0} conversations, ${counts?.contacts ?? 0} people and ${counts?.companies ?? 0} companies. This cannot be undone.`}
      confirmLabel="Delete tag"
      onConfirm={() => remove.mutateAsync()}
    />
  );
}

export function TagsSettings() {
  const workspace = useAccount().workspace.id;
  const { data: tags } = useQuery(tagsQuery(workspace, true));
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = caller?.member?.permissions.includes("tags:manage") ?? false;
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<ListedTag | null>(null);
  const [deleting, setDeleting] = useState<ListedTag | null>(null);

  return (
    <SettingsPage title="Tags" description="Any teammate can create tags; admins rename, recolour and delete them.">
      <Section title="Tags" action={<Action onPress={() => setCreating(true)}>New tag</Action>}>
        {tags?.length === 0 ? <Empty>No tags yet.</Empty> : null}
        {tags?.map((tag) => (
          <Row
            key={tag.id}
            label={
              <span className="flex items-center gap-2">
                <TagDot color={tag.color} />
                {tag.name}
              </span>
            }
          >
            <span className="text-body-2-regular text-text-tertiary">{countOf(tag)}</span>
            {canManage ? (
              <>
                <Action size="xs" onPress={() => setEditing(tag)}>
                  Edit
                </Action>
                <Danger onPress={() => setDeleting(tag)}>Delete</Danger>
              </>
            ) : null}
          </Row>
        ))}
      </Section>
      <NewTagDialog isOpen={creating} onClose={() => setCreating(false)} />
      {editing ? <EditTagDialog key={editing.id} tag={editing} onClose={() => setEditing(null)} /> : null}
      {deleting ? <DeleteTagDialog tag={deleting} onClose={() => setDeleting(null)} /> : null}
    </SettingsPage>
  );
}
