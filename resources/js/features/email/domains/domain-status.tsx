/*
 * How a domain or one of its records stands, as a coloured chip: green
 * once proven, amber while waiting, red when something is wrong, grey
 * for guidance that never blocks.
 */
import { Chip, type ChipColor } from "@/components/base/badges/chip";
import type { DomainStatus, RecordStatus } from "@/lib/api/email-domains";

const domainLabels: Record<DomainStatus, [string, ChipColor]> = {
  pending: ["Not checked yet", "neutral"],
  verifying: ["Checking", "yellow"],
  verified: ["Verified", "lime"],
  degraded: ["Records missing", "orange"],
  failed: ["Failed", "rose"],
};

const recordLabels: Record<RecordStatus, [string, ChipColor]> = {
  verified: ["Found", "lime"],
  mismatch: ["Different value", "rose"],
  missing: ["Missing", "orange"],
  propagating: ["Spreading", "yellow"],
  error: ["Not readable", "neutral"],
  info: ["Optional", "gray"],
};

export function DomainStatusChip({ status }: { status: DomainStatus }) {
  const [label, color] = domainLabels[status];

  return (
    <Chip variant="caption" color={color}>
      {label}
    </Chip>
  );
}

export function RecordStatusChip({ status }: { status: RecordStatus }) {
  const [label, color] = recordLabels[status];

  return (
    <Chip variant="caption" color={color}>
      {label}
    </Chip>
  );
}
