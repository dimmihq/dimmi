/*
 * What the header of a list offers about its view: the filter icon opens
 * a dropdown of conditions, starting from the list on screen and keeping
 * its status and sort, that either filters the list right away or is
 * saved as a new view. Opened from the view picker's new view action,
 * the same dropdown leads with saving, since a view is what was asked
 * for. While conditions are applied the icon carries a dot, and a
 * second icon clears them; a saved view can also be
 * edited in the same dropdown, copied, and pinned so the view picker
 * counts its open conversations. The new view dropdown also opens from
 * the view picker's new view action.
 */
import {
  RiFileCopyLine,
  RiFilter3Line,
  RiFilterOffLine,
  RiPencilLine,
  RiPushpin2Fill,
  RiPushpin2Line,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { IconButton } from "@/components/base/buttons/icon-button";
import { Popover, PopoverTrigger } from "@/components/base/popover/popover";
import { preferencesQuery, updatePreferences } from "@/lib/api/inbox";
import { createView, viewsQuery } from "@/lib/api/views";
import { useMutationId } from "@/lib/realtime/context";
import { realtimeKeys } from "@/lib/realtime/keys";

import { type ViewDraftStart, ViewEditor } from "./view-editor";
import { decodeWhere, encodeWhere } from "./where";

/*
 * EditorPopover is the dropdown the view editor opens in, wide enough
 * for a row of conditions and no taller than the window.
 */
function EditorPopover({ children }: { children: ReactNode }) {
  return (
    <Popover
      placement="bottom end"
      className="w-[42rem]"
      dialogClassName="max-h-[min(44rem,calc(100dvh-7rem))] gap-3 p-1.5"
    >
      {children}
    </Popover>
  );
}

/*
 * usePin pins and unpins a saved view in the person's sidebar.
 */
function usePin(workspace: string, id: string) {
  const client = useQueryClient();
  const mutation = useMutationId();
  const { data: prefs } = useQuery({ ...preferencesQuery(workspace), enabled: workspace !== "" });
  const pinned = ((prefs?.sidebar.pinned_views as string[] | undefined) ?? []).includes(id);
  const toggle = useMutation({
    mutationFn: () => {
      const current = (prefs?.sidebar.pinned_views as string[] | undefined) ?? [];
      const next = pinned ? current.filter((v) => v !== id) : [...current, id];
      return updatePreferences(
        { workspace, mutation: mutation() },
        { sidebar: { ...prefs?.sidebar, pinned_views: next } },
      );
    },
    onSuccess: (saved) => {
      client.setQueryData(realtimeKeys.preferences(workspace), saved);
      void client.invalidateQueries({ queryKey: realtimeKeys.counts(workspace) });
    },
  });

  return { pinned, toggle: () => toggle.mutate() };
}

function SavedViewActions({ workspace, id }: { workspace: string; id: string }) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const { data: views } = useQuery({ ...viewsQuery(workspace), enabled: workspace !== "" });
  const view = views?.find((v) => v.id === id) ?? null;
  const [editing, setEditing] = useState(false);
  const pin = usePin(workspace, id);
  const copy = useMutation({
    mutationFn: () =>
      createView(workspace, {
        name: `Copy of ${view?.name ?? "view"}`.slice(0, 60),
        scope: "personal",
        definition: view?.definition,
        sort: view?.sort ?? undefined,
        status_filter: view?.status_filter ?? undefined,
      }),
    onSuccess: (saved) => {
      void client.invalidateQueries({ queryKey: realtimeKeys.views(workspace) });
      void navigate({ to: "/d/inbox/$view", params: { view: saved.id } });
    },
  });

  return (
    <>
      <IconButton
        icon={pin.pinned ? RiPushpin2Fill : RiPushpin2Line}
        size="small"
        aria-label={pin.pinned ? "Unpin view" : "Pin view"}
        aria-pressed={pin.pinned}
        onClick={pin.toggle}
      />
      <IconButton icon={RiFileCopyLine} size="small" aria-label="Duplicate view" onClick={() => copy.mutate()} />
      <PopoverTrigger isOpen={editing} onOpenChange={setEditing}>
        <IconButton icon={RiPencilLine} size="small" aria-label="Edit view" disabled={view === null} />
        <EditorPopover>
          {view ? (
            <ViewEditor key={view.id} workspace={workspace} view={view} onClose={() => setEditing(false)} />
          ) : null}
        </EditorPopover>
      </PopoverTrigger>
    </>
  );
}

export function ViewActions({ workspace, view, start }: { workspace: string; view: string; start: ViewDraftStart }) {
  const navigate = useNavigate();
  const { editor, where } = useSearch({ strict: false }) as { editor?: "new"; where?: string };
  const [creating, setCreating] = useState(false);
  const applied = decodeWhere(where);
  const open = creating || editor === "new";
  const setWhere = (next: string | undefined) =>
    void navigate({ to: ".", search: (prev: Record<string, unknown>) => ({ ...prev, where: next }) });
  const close = (next: boolean) => {
    setCreating(next);
    if (!next && editor) {
      void navigate({ to: ".", search: (prev: Record<string, unknown>) => ({ ...prev, editor: undefined }) });
    }
  };

  return (
    <>
      {applied ? (
        <IconButton
          icon={RiFilterOffLine}
          size="small"
          aria-label="Clear filters"
          onClick={() => setWhere(undefined)}
        />
      ) : null}
      <PopoverTrigger isOpen={open} onOpenChange={close}>
        <span className="relative flex">
          <IconButton
            icon={RiFilter3Line}
            size="small"
            aria-label={applied ? "Edit filters" : "Filter"}
            aria-pressed={applied !== null}
          />
          {applied ? (
            <span
              aria-hidden
              className="pointer-events-none absolute end-1 top-1 size-1.5 rounded-full bg-accent-500"
            />
          ) : null}
        </span>
        <EditorPopover>
          <ViewEditor
            workspace={workspace}
            view={null}
            start={start}
            applied={applied}
            onApply={editor === "new" ? undefined : (definition) => setWhere(encodeWhere(definition))}
            onClose={() => close(false)}
          />
        </EditorPopover>
      </PopoverTrigger>
      {view.startsWith("cvw_") ? <SavedViewActions workspace={workspace} id={view} /> : null}
    </>
  );
}
