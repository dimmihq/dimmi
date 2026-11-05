/*
 * The tools those who manage members use to review them: a view of the
 * list narrowed to members without a second factor, a download of the
 * list as a spreadsheet, and the marks on each row saying who lacks a
 * second factor and when each person last came by.
 */
import { RiDownloadLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { exportMembers, type Member, type MemberFilter } from "@/lib/api/workspace";

import { activity, type FilterKey, filters, lacksTwoFactor } from "./member-review";

/*
 * save hands the browser a downloaded file under its own name.
 */
function save(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

export function MemberFilterSelect({ value, onChange }: { value: FilterKey; onChange: (key: FilterKey) => void }) {
  return (
    <Select
      aria-label="Show"
      size="sm"
      selectedKey={value}
      onSelectionChange={(key) =>
        onChange(key === filters.withoutTwoFactor ? filters.withoutTwoFactor : filters.everyone)
      }
    >
      <SelectItem id={filters.everyone} textValue="Everyone">
        Everyone
      </SelectItem>
      <SelectItem id={filters.withoutTwoFactor} textValue="Without two-factor">
        Without two-factor
      </SelectItem>
    </Select>
  );
}

export function ExportMembersButton({ workspace, filter }: { workspace: string; filter: MemberFilter }) {
  const download = useMutation({
    mutationFn: () => exportMembers(workspace, filter),
    onSuccess: (file) => save(file.blob, file.filename),
  });

  return (
    <Button
      variant="secondary"
      size="small"
      leadingIcon={RiDownloadLine}
      loading={download.isPending}
      onClick={() => download.mutate()}
    >
      Export CSV
    </Button>
  );
}

export function TwoFactorMark({ member }: { member: Member }) {
  if (!lacksTwoFactor(member)) {
    return null;
  }

  return (
    <Chip variant="caption" color="yellow">
      No two-factor
    </Chip>
  );
}

export function LastActive({ member, timeZone }: { member: Member; timeZone?: string }) {
  const seen = activity(member, Date.now(), timeZone);
  if (seen === null) {
    return null;
  }

  return (
    <span title={seen.full || undefined} className="hidden text-body-2-regular text-text-tertiary sm:inline">
      {seen.relative}
    </span>
  );
}
