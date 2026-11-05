/*
 * The workspace's tags, on the server, with how many conversations carry
 * each. Any teammate can add one. Renaming, recolouring and deleting
 * change what every record means, so they show only to people the server
 * lets manage tags, and deleting says how many it takes the tag from.
 */

import { RiDeleteBinLine, RiPencilLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { parseColor } from "react-aria-components";

import { Button } from "@/components/base/buttons/button";
import { ColorWell } from "@/components/base/color-picker/color-picker";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagIconPicker } from "@/features/inbox/tags/tag-icon-picker";
import { createTag, deleteTag, type ListedTag, tagsQuery, updateTag } from "@/lib/api/tags";
import { callerQuery } from "@/lib/api/workspace";

import { Action } from "../controls";
import { SettingsPage } from "../layout";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const swatches = [
  "#2563eb",
  "#16a34a",
  "#dc2626",
  "#9333ea",
  "#ea580c",
  "#0891b2",
  "#db2777",
  "#ca8a04",
  "#4f46e5",
  "#0d9488",
  "#65a30d",
  "#c026d3",
];

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
  const [icon, setIcon] = useState(tag.icon ?? "price-tag-3");
  const save = useMutation({
    mutationFn: () => updateTag(workspace, tag.id, { name, icon, color: color.toString("hex").toLowerCase() }),
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
      <TagIconPicker value={icon} onChange={setIcon} />
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

/*
 * TagColor is the swatch on a tag's row: it opens the colour picker and
 * saves the colour picked once the pointer settles, so dragging across
 * the picker does not send a request for every step.
 */
function TagColor({ tag }: { tag: ListedTag }) {
  const workspace = useAccount().workspace.id;
  const refresh = useTagWrites(workspace);
  const [color, setColor] = useState(parseColor(tag.color ?? "#737373"));
  const pending = useRef<number | undefined>(undefined);
  const save = useMutation({
    mutationFn: (hex: string) => updateTag(workspace, tag.id, { color: hex }),
    onSuccess: refresh,
  });

  useEffect(() => () => window.clearTimeout(pending.current), []);

  return (
    <ColorWell
      aria-label={`${tag.name} colour`}
      value={color}
      onChange={(next) => {
        setColor(next);
        window.clearTimeout(pending.current);
        pending.current = window.setTimeout(() => save.mutate(next.toString("hex").toLowerCase()), 400);
      }}
      swatches={swatches}
      placement="bottom end"
      wellClassName="size-6"
    />
  );
}

function TagRow({
  tag,
  canManage,
  onEdit,
  onDelete,
}: {
  tag: ListedTag;
  canManage: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  return (
    <li className="flex min-h-[52px] items-center gap-3 rounded-2xl bg-background-secondary-default px-3 py-2.5">
      <span className="min-w-0 flex-1">
        <TagChip tag={tag} />
      </span>
      <span className="text-body-2-regular text-text-tertiary">{countOf(tag)}</span>
      {canManage ? (
        <span className="flex items-center gap-2">
          <TagColor tag={tag} />
          <Button
            iconOnly
            variant="secondary"
            size="small"
            leadingIcon={RiPencilLine}
            aria-label={`Edit ${tag.name}`}
            onClick={onEdit}
          />
          <Button
            iconOnly
            variant="danger-secondary"
            size="small"
            leadingIcon={RiDeleteBinLine}
            aria-label={`Delete ${tag.name}`}
            onClick={onDelete}
          />
        </span>
      ) : null}
    </li>
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
      <section aria-label="Tags" className="flex flex-col gap-2">
        <div className="flex items-end justify-between gap-4 px-3">
          <p className="text-body-2-regular text-text-tertiary">
            {canManage
              ? "Click a tag's colour to change it."
              : "Tags you can use on conversations, people and companies."}
          </p>
          <Action onPress={() => setCreating(true)}>New tag</Action>
        </div>
        {tags?.length === 0 ? (
          <p className="rounded-2xl bg-background-secondary-default px-3 py-2.5 text-body-2-regular text-text-tertiary">
            No tags yet.
          </p>
        ) : null}
        <ul className="flex flex-col gap-2">
          {tags?.map((tag) => (
            <TagRow
              key={tag.id}
              tag={tag}
              canManage={canManage}
              onEdit={() => setEditing(tag)}
              onDelete={() => setDeleting(tag)}
            />
          ))}
        </ul>
      </section>
      <NewTagDialog isOpen={creating} onClose={() => setCreating(false)} />
      {editing ? <EditTagDialog key={editing.id} tag={editing} onClose={() => setEditing(null)} /> : null}
      {deleting ? <DeleteTagDialog tag={deleting} onClose={() => setDeleting(null)} /> : null}
    </SettingsPage>
  );
}
