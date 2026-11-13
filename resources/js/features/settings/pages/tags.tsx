/*
 * The workspace's tags, on the server, with how many conversations carry
 * each. Any teammate can add one. Renaming, recolouring and deleting
 * change what every record means, so they show only to people the server
 * lets manage tags, and deleting says how many it takes the tag from.
 * Colours picked on the rows wait in one draft for the page's save bar;
 * the dialogs to create, edit and delete a tag save on their own.
 */

import { RiDeleteBinLine, RiPencilLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useState } from "react";
import { parseColor } from "react-aria-components";

import { Button } from "@/components/base/buttons/button";
import { ColorWell } from "@/components/base/color-picker/color-picker";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagColorPicker } from "@/features/inbox/tags/tag-color-picker";
import { tagPalette } from "@/features/inbox/tags/tag-colors";
import { TagIconPicker } from "@/features/inbox/tags/tag-icon-picker";
import { createTag, deleteTag, type ListedTag, tagsQuery, updateTag } from "@/lib/api/tags";
import { callerQuery } from "@/lib/api/workspace";
import { Action } from "../controls";
import { SettingsPage } from "../layout";
import { placeholders } from "../placeholders";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const swatches = tagPalette;

function useTagWrites(workspace: string) {
  const queryClient = useQueryClient();

  return () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
}

function countOf(tag: ListedTag): string {
  const count = tag.counts?.conversations ?? 0;

  return count === 1 ? "1 conversation" : `${count} conversations`;
}

/*
 * TagFields are what a tag is made of: its name, colour and icon, above
 * the chip they make.
 */
function TagFields({
  name,
  color,
  icon,
  error,
  onName,
  onColor,
  onIcon,
}: {
  name: string;
  color: string;
  icon: string;
  error: Error | null;
  onName: (name: string) => void;
  onColor: (color: string) => void;
  onIcon: (icon: string) => void;
}) {
  return (
    <div className="flex flex-col gap-5">
      <Input
        label="Tag name"
        value={name}
        onChange={onName}
        isInvalid={error !== null}
        errorMessage={failureText(error)}
      />
      <TagColorPicker value={color} onChange={onColor} />
      <TagIconPicker value={icon} onChange={onIcon} />
      <div className="flex">
        <TagChip tag={{ id: "preview", object: "tag", name: name.trim() || "Tag", color, icon }} />
      </div>
    </div>
  );
}

function NewTagDialog({ isOpen, onClose, suggested }: { isOpen: boolean; onClose: () => void; suggested: string }) {
  const workspace = useAccount().workspace.id;
  const refresh = useTagWrites(workspace);
  const [name, setName] = useState("");
  const [color, setColor] = useState<string | null>(null);
  const [icon, setIcon] = useState("price-tag-3");
  const create = useMutation({
    mutationFn: () => createTag(workspace, name, { color: color ?? suggested, icon }),
    onSuccess: async () => {
      setName("");
      setColor(null);
      setIcon("price-tag-3");
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
      <TagFields
        name={name}
        color={color ?? suggested}
        icon={icon}
        error={create.error}
        onName={setName}
        onColor={setColor}
        onIcon={setIcon}
      />
    </AlertDialog>
  );
}

function EditTagDialog({ tag, onClose }: { tag: ListedTag; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const refresh = useTagWrites(workspace);
  const [name, setName] = useState(tag.name);
  const [color, setColor] = useState(tag.color ?? tagPalette[0]);
  const [icon, setIcon] = useState(tag.icon ?? "price-tag-3");
  const save = useMutation({
    mutationFn: () => updateTag(workspace, tag.id, { name, icon, color }),
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
      <TagFields
        name={name}
        color={color}
        icon={icon}
        error={save.error}
        onName={setName}
        onColor={setColor}
        onIcon={setIcon}
      />
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
 * useColorDraft holds the colours picked on the rows but not saved yet,
 * by tag. Save sends each one that differs from the tag's saved colour
 * and reloads the tags; Discard puts them back. A colour saved from the
 * edit dialog meanwhile stops counting as a change.
 */
function useColorDraft(workspace: string, tags: ListedTag[] | undefined) {
  const refresh = useTagWrites(workspace);
  const [pending, setPending] = useState<Record<string, string>>({});
  const save = useMutation({
    mutationFn: (sent: Record<string, string>) =>
      Promise.all(Object.entries(sent).map(([id, color]) => updateTag(workspace, id, { color }))),
    onSuccess: () => setPending({}),
    onSettled: refresh,
  });

  const changes = Object.fromEntries(
    Object.entries(pending).filter(([id, color]) => {
      const tag = tags?.find((t) => t.id === id);

      return tag !== undefined && tag.color?.toLowerCase() !== color;
    }),
  );

  return {
    pending,
    changes,
    dirty: Object.keys(changes).length > 0,
    save,
    set: (tag: ListedTag, color: string) =>
      setPending(({ [tag.id]: _, ...rest }) =>
        color === tag.color?.toLowerCase() ? rest : { ...rest, [tag.id]: color },
      ),
    discard: () => {
      setPending({});
      save.reset();
    },
  };
}

type ColorDraft = ReturnType<typeof useColorDraft>;

/*
 * TagColor is the swatch on a tag's row: it opens the colour picker and
 * shows the colour picked, which waits in the draft for Save.
 */
function TagColor({ tag, draft }: { tag: ListedTag; draft: ColorDraft }) {
  const hex = draft.pending[tag.id] ?? tag.color ?? "#737373";

  return (
    <ColorWell
      aria-label={`${tag.name} colour`}
      value={parseColor(hex)}
      onChange={(next) => draft.set(tag, next.toString("hex").toLowerCase())}
      swatches={swatches}
      placement="bottom end"
      wellClassName="size-6"
    />
  );
}

function TagRow({
  tag,
  canManage,
  draft,
  onEdit,
  onDelete,
}: {
  tag: ListedTag;
  canManage: boolean;
  draft: ColorDraft;
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
          <TagColor tag={tag} draft={draft} />
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

/*
 * TagRowsSkeleton stands in for the tags while they load, each as tall as
 * a tag's row.
 */
function TagRowsSkeleton() {
  return placeholders(4).map((id) => (
    <li
      key={id}
      aria-hidden
      className="flex h-[52px] items-center gap-3 rounded-2xl bg-background-secondary-default px-3"
    >
      <Skeleton className="h-6 w-24 rounded-full" />
      <Skeleton shape="text" className="ms-auto h-3 w-16" />
    </li>
  ));
}

export function TagsSettings() {
  const workspace = useAccount().workspace.id;
  const { data: tags } = useQuery(tagsQuery(workspace, true));
  const { data: caller } = useQuery(callerQuery(workspace));
  const canManage = caller?.member?.permissions.includes("tags:manage") ?? false;
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<ListedTag | null>(null);
  const [deleting, setDeleting] = useState<ListedTag | null>(null);
  const colors = useColorDraft(workspace, tags);
  useBlocker({
    shouldBlockFn: () => colors.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => colors.dirty,
  });

  return (
    <SettingsPage
      title="Tags"
      description="Any teammate can create tags; admins rename, recolour and delete them."
      footer={
        <SaveBar
          dirty={colors.dirty}
          saving={colors.save.isPending}
          error={colors.save.error ? failureText(colors.save.error) : null}
          onSave={() => colors.save.mutate(colors.changes)}
          onDiscard={colors.discard}
        />
      }
    >
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
        <ul aria-busy={!tags || undefined} className="flex flex-col gap-2">
          {tags ? null : <TagRowsSkeleton />}
          {tags?.map((tag) => (
            <TagRow
              key={tag.id}
              tag={tag}
              canManage={canManage}
              draft={colors}
              onEdit={() => setEditing(tag)}
              onDelete={() => setDeleting(tag)}
            />
          ))}
        </ul>
      </section>
      <NewTagDialog
        isOpen={creating}
        onClose={() => setCreating(false)}
        suggested={tagPalette[(tags?.length ?? 0) % tagPalette.length] ?? "#2563eb"}
      />
      {editing ? (
        <EditTagDialog
          key={editing.id}
          tag={{ ...editing, color: colors.pending[editing.id] ?? editing.color }}
          onClose={() => setEditing(null)}
        />
      ) : null}
      {deleting ? <DeleteTagDialog tag={deleting} onClose={() => setDeleting(null)} /> : null}
    </SettingsPage>
  );
}
