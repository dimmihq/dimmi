/*
 * The report's actions: a CSV of one row per conversation the view
 * covers, built in the background like every export, and the member's
 * digest of the reports by mail, daily, weekly or off.
 */
import { RiDownload2Line, RiMailLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Popover, PopoverHeader, PopoverTrigger } from "@/components/base/popover/popover";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import { dataExportsKey } from "@/lib/api/data-exports";
import {
  type DigestFrequency,
  deleteReportDigest,
  exportReportFacts,
  type ReportDigest,
  type ReportFilters,
  reportDigestKey,
  reportDigestQuery,
  updateReportDigest,
} from "@/lib/api/reports";
import { formatDateTime } from "@/lib/i18n/format";

export function FactsExportButton({ workspace, filters }: { workspace: string; filters: ReportFilters }) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const start = useMutation({
    mutationFn: () => exportReportFacts(workspace, filters),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: dataExportsKey(workspace) });
      toast.success("Export started", {
        description: "It is built in the background. Download it from Exports once it is ready.",
        actions: [
          {
            label: "Go to Exports",
            onClick: () => void navigate({ to: "/d/settings/$section", params: { section: "exports" } }),
          },
        ],
      });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <>
      <Button
        size="small"
        variant="secondary"
        iconOnly
        leadingIcon={RiDownload2Line}
        aria-label="Export facts CSV"
        className="sm:hidden"
        loading={start.isPending}
        onClick={() => start.mutate()}
      />
      <Button
        size="small"
        variant="secondary"
        leadingIcon={RiDownload2Line}
        className="max-sm:hidden"
        loading={start.isPending}
        onClick={() => start.mutate()}
      >
        Export facts CSV
      </Button>
    </>
  );
}

type Choice = DigestFrequency | "off";

const choices: [Choice, string][] = [
  ["off", "Off"],
  ["daily", "Daily"],
  ["weekly", "Weekly"],
];

function DigestForm({ workspace, digest }: { workspace: string; digest: ReportDigest | undefined }) {
  const client = useQueryClient();
  const save = useMutation({
    mutationFn: ({ frequency, businessHours }: { frequency: Choice; businessHours: boolean }) =>
      frequency === "off"
        ? deleteReportDigest(workspace).then(() => null)
        : updateReportDigest(workspace, {
            frequency,
            inbox_ids: digest?.inbox_ids,
            team_ids: digest?.team_ids,
            channels: digest?.channels,
            business_hours: businessHours,
          }),
    onSuccess: (next, { frequency }) => {
      client.setQueryData(
        reportDigestKey(workspace),
        next ?? (digest ? { ...digest, frequency: null, next_send_at: null } : undefined),
      );
      toast.success(
        frequency === "off" ? "Digest turned off" : frequency === "daily" ? "Daily digest on" : "Weekly digest on",
      );
    },
    onError: (error) => toast.error(error.message),
  });
  const frequency: Choice = digest?.frequency ?? "off";
  const businessHours = digest?.business_hours ?? false;

  return (
    <div className="flex flex-col gap-3">
      <PopoverHeader title="Email digest" description="The main numbers of your reports, by mail." />
      <SegmentedControl
        aria-label="How often"
        selectedKeys={new Set([frequency])}
        isDisabled={digest === undefined || save.isPending}
        onSelectionChange={(keys) => {
          const [key] = [...keys];
          if (key === "off" || key === "daily" || key === "weekly") {
            if (key !== frequency) {
              save.mutate({ frequency: key, businessHours });
            }
          }
        }}
      >
        {choices.map(([id, name]) => (
          <SegmentedControlItem key={id} id={id}>
            {name}
          </SegmentedControlItem>
        ))}
      </SegmentedControl>
      {frequency === "off" ? null : (
        <Switch
          size="sm"
          isSelected={businessHours}
          isDisabled={save.isPending}
          onChange={(on) => save.mutate({ frequency, businessHours: on })}
        >
          Count business hours only
        </Switch>
      )}
      {frequency !== "off" && digest?.next_send_at ? (
        <p className="text-caption-1-regular text-text-tertiary">Next one {formatDateTime(digest.next_send_at)}</p>
      ) : null}
    </div>
  );
}

export function DigestButton({ workspace }: { workspace: string }) {
  const [open, setOpen] = useState(false);
  const digest = useQuery({ ...reportDigestQuery(workspace), enabled: workspace !== "" });
  const on = Boolean(digest.data?.frequency);

  return (
    <PopoverTrigger isOpen={open} onOpenChange={setOpen}>
      <Button
        size="small"
        variant="secondary"
        leadingIcon={RiMailLine}
        aria-label={on ? "Email digest: on" : "Email digest"}
        className="max-sm:size-8 max-sm:px-0 max-sm:[&>span]:hidden"
      >
        Email digest
      </Button>
      <Popover aria-label="Email digest" placement="bottom end" className="w-72">
        {digest.isError ? (
          <p className="text-body-2-regular text-text-error-primary">{digest.error.message}</p>
        ) : (
          <DigestForm workspace={workspace} digest={digest.data} />
        )}
      </Popover>
    </PopoverTrigger>
  );
}
