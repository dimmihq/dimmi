/*
 * The tags a tag step adds or removes, picked with the same tag picker
 * the inbox and customers use: type to find a tag, and a name nobody has
 * used yet becomes a new tag. Each chosen tag shows as a chip that can be
 * taken out again.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@/components/base/toast/toast";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { createTag, type Tag, tagsQuery } from "@/lib/api/tags";

export function TagField({
  workspace,
  ids,
  editable,
  onChange,
}: {
  workspace: string;
  ids: string[];
  editable: boolean;
  onChange: (ids: string[]) => void;
}) {
  const client = useQueryClient();
  const tags = useQuery(tagsQuery(workspace)).data ?? [];
  const chosen = ids.map((id) => tags.find((tag) => tag.id === id)).filter((tag): tag is Tag => tag !== undefined);
  const keep = (next: Tag[]) => onChange(next.map((tag) => tag.id));

  const add = async (name: string) => {
    const known = tags.find((tag) => tag.name.toLowerCase() === name.toLowerCase());
    if (known) {
      keep([...chosen, known]);
      return;
    }
    try {
      const created = await createTag(workspace, name);
      await client.invalidateQueries({ queryKey: tagsQuery(workspace).queryKey });
      keep([...chosen, created]);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "The tag was not created.");
    }
  };
  const remove = (tag: Tag) => keep(chosen.filter((other) => other.id !== tag.id));

  return (
    <div className="flex flex-wrap items-center gap-1.5 py-3">
      {chosen.map((tag) => (
        <TagChip key={tag.id} tag={tag} onRemove={editable ? () => remove(tag) : undefined} />
      ))}
      {editable ? (
        <TagPicker workspace={workspace} selected={chosen} onAdd={add} onRemove={remove} label="Add a tag" />
      ) : null}
      {!editable && chosen.length === 0 ? (
        <span className="text-body-2-regular text-text-tertiary">No tags chosen</span>
      ) : null}
    </div>
  );
}
