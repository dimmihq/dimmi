/*
 * The controls above a list: which status it shows, and in which order.
 * A view with a status of its own, such as Snoozed, hides the status
 * control. Snooze ends is offered only while the list shows snoozed
 * conversations, the one list it can order.
 */
import type { Key } from "react-aria-components";

import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem } from "@/components/base/select/select";
import type { ConversationSort, StatusFilter } from "@/lib/api/inbox";
import { sortsFor } from "./view-route";

const statusLabels: [StatusFilter, string][] = [
  ["open", "Open"],
  ["snoozed", "Snoozed"],
  ["closed", "Closed"],
  ["any", "All"],
];

export function ListControls({
  status,
  fixedStatus,
  sort,
  onStatusChange,
  onSortChange,
}: {
  status: StatusFilter;
  fixedStatus: boolean;
  sort: ConversationSort;
  onStatusChange: (status: StatusFilter) => void;
  onSortChange: (sort: ConversationSort) => void;
}) {
  const sorts = sortsFor(status);

  return (
    <div className="flex min-w-0 shrink-0 flex-wrap items-center justify-between gap-2 px-[7px] pb-2">
      {fixedStatus ? (
        <span />
      ) : (
        <SegmentedControl
          aria-label="Status"
          selectedKeys={[status]}
          onSelectionChange={(selected) => {
            const next = statusLabels.find(([value]) => selected.has(value));
            if (next) {
              onStatusChange(next[0]);
            }
          }}
          className="[&_button]:px-2 [&_button]:py-0.5 [&_button]:text-body-2-medium"
        >
          {statusLabels.map(([value, text]) => (
            <SegmentedControlItem key={value} id={value}>
              {text}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      )}
      <Select
        aria-label="Sort"
        size="sm"
        selectedKey={sorts.some((s) => s.value === sort) ? sort : sorts[0].value}
        onSelectionChange={(key: Key | null) => {
          const next = sorts.find((s) => s.value === key);
          if (next) {
            onSortChange(next.value);
          }
        }}
        triggerClassName="text-body-2-medium"
      >
        {sorts.map((option) => (
          <SelectItem key={option.value} id={option.value} textValue={option.label}>
            {option.label}
          </SelectItem>
        ))}
      </Select>
    </div>
  );
}
