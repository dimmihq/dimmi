/*
 * The person's exports of conversations and customers: what each covers,
 * where it stands, and its file to download until the workspace's export
 * period runs out. Exports are started from the lists themselves; this
 * page only keeps them. Deleting one removes its file at once. The list
 * checks again every few seconds while one is still being built. Whoever
 * may export the whole workspace starts that archive here; it downloads
 * from a signed link, since it can be far larger than a list's file.
 */
import { RiArchiveLine, RiDownload2Line, RiFileList3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Notification } from "@/components/base/notification/notification";
import {
  type DataExport,
  dataExportLink,
  dataExportsKey,
  dataExportsQuery,
  deleteDataExport,
  downloadDataExport,
  requestWorkspaceExport,
  WORKSPACE_EXPORT,
} from "@/lib/api/data-exports";
import { callerQuery } from "@/lib/api/workspace";

import { Action } from "../controls";
import { building, exportDetail, statusNames } from "../export-format";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";

const pollEvery = 3000;

function save(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

export function ExportsSettings() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const exportsAll = caller?.member?.permissions.includes(WORKSPACE_EXPORT) ?? false;
  const { data: exports, isPending } = useQuery({
    ...dataExportsQuery(workspace),
    enabled: workspace !== "",
    refetchInterval: (query) => (building(query.state.data) ? pollEvery : false),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteDataExport(workspace, id),
    onSettled: () => client.invalidateQueries({ queryKey: dataExportsKey(workspace) }),
  });
  const download = useMutation({
    mutationFn: async (e: DataExport) => {
      if (e.kind === "workspace") {
        window.location.assign(await dataExportLink(workspace, e.id));
      } else if (e.url) {
        save(await downloadDataExport(workspace, e.url), e.file_name);
      }
    },
  });
  const whole = useMutation({
    mutationFn: () => requestWorkspaceExport(workspace),
    onSettled: () => client.invalidateQueries({ queryKey: dataExportsKey(workspace) }),
  });
  const failure = remove.error ?? download.error ?? whole.error;

  return (
    <SettingsPage title="Exports" description="Files of conversations and customers you exported from their lists.">
      {exportsAll ? (
        <Section
          title="Whole workspace"
          description="Take everything with you, in a format other tools can read. Passwords, keys and tokens stay out."
        >
          <Item
            mark={<Glyph icon={RiArchiveLine} />}
            title="Complete workspace export"
            detail="One zip of conversations, customers, tickets, articles, settings and files, as JSON lines."
          >
            <Button variant="secondary" size="small" loading={whole.isPending} onClick={() => whole.mutate()}>
              Export workspace
            </Button>
          </Item>
        </Section>
      ) : null}
      <Section title="Your exports" description="Only you see your exports. Files are removed when they expire.">
        {failure ? (
          <div className="py-2.5 pe-2.5">
            <Notification status="error" title={failure.message} dismissible={false} role="alert" className="w-full" />
          </div>
        ) : null}
        {isPending ? <Empty>Loading…</Empty> : null}
        {!isPending && (exports ?? []).length === 0 ? (
          <Empty>No exports yet. Use Export on a conversation list or the people directory.</Empty>
        ) : null}
        {(exports ?? []).map((e) => (
          <Item key={e.id} mark={<Glyph icon={RiFileList3Line} />} title={e.file_name} detail={exportDetail(e)}>
            {e.status === "ready" ? (
              <Button
                variant="secondary"
                size="small"
                leadingIcon={RiDownload2Line}
                disabled={download.isPending}
                onClick={() => download.mutate(e)}
              >
                Download
              </Button>
            ) : (
              <Chip variant="caption" color="gray">
                {statusNames[e.status]}
              </Chip>
            )}
            <Action onPress={() => remove.mutate(e.id)}>Delete</Action>
          </Item>
        ))}
      </Section>
    </SettingsPage>
  );
}
