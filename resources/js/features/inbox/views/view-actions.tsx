/*
 * What the header of a list offers about its view: any list can be saved
 * as a view, keeping its status and sort; a saved view can be edited,
 * copied, and pinned so the view picker counts its open conversations. The
 * editor also opens from the view picker's new view action.
 */
import { RiFileCopyLine, RiFilter3Line, RiPencilLine, RiPushpin2Fill, RiPushpin2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { preferencesQuery, updatePreferences } from "@/lib/api/inbox";
import { createView, viewsQuery } from "@/lib/api/views";
import { useMutationId } from "@/lib/realtime/context";
import { realtimeKeys } from "@/lib/realtime/keys";

import { type ViewDraftStart, ViewEditor } from "./view-editor";

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
      <Button
        iconOnly
        size="xs"
        variant="ghost"
        leadingIcon={pin.pinned ? RiPushpin2Fill : RiPushpin2Line}
        aria-label={pin.pinned ? "Unpin view" : "Pin view"}
        aria-pressed={pin.pinned}
        onClick={pin.toggle}
      />
      <Button
        iconOnly
        size="xs"
        variant="ghost"
        leadingIcon={RiFileCopyLine}
        aria-label="Duplicate view"
        onClick={() => copy.mutate()}
      />
      <Button
        iconOnly
        size="xs"
        variant="ghost"
        leadingIcon={RiPencilLine}
        aria-label="Edit view"
        onClick={() => setEditing(true)}
        disabled={view === null}
      />
      {view && editing ? (
        <ViewEditor key={view.id} workspace={workspace} view={view} isOpen={editing} onOpenChange={setEditing} />
      ) : null}
    </>
  );
}

export function ViewActions({ workspace, view, start }: { workspace: string; view: string; start: ViewDraftStart }) {
  const navigate = useNavigate();
  const { editor } = useSearch({ strict: false }) as { editor?: "new" };
  const [creating, setCreating] = useState(false);
  const open = creating || editor === "new";
  const close = (next: boolean) => {
    setCreating(next);
    if (!next && editor) {
      void navigate({ to: ".", search: (prev: Record<string, unknown>) => ({ ...prev, editor: undefined }) });
    }
  };

  return (
    <>
      {view.startsWith("cvw_") ? (
        <SavedViewActions workspace={workspace} id={view} />
      ) : (
        <Button
          iconOnly
          size="xs"
          variant="ghost"
          leadingIcon={RiFilter3Line}
          aria-label="Save as view"
          onClick={() => setCreating(true)}
        />
      )}
      {open ? <ViewEditor workspace={workspace} view={null} start={start} isOpen={open} onOpenChange={close} /> : null}
    </>
  );
}
