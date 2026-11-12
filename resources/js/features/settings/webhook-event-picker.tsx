/*
 * Picks the events an endpoint hears from the catalog: every event, or
 * the ones ticked under each area. An area's own box ticks or clears
 * all of its types at once. Types that are too frequent for a wildcard
 * say so, and are only sent when ticked by name.
 */
import { RiApps2Line, RiListCheck2 } from "@remixicon/react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import type { WebhookEventType } from "@/lib/api/webhooks";

import { groupEventTypes } from "./webhooks";

export type PickMode = "all" | "some";

export function WebhookEventPicker({
  catalog,
  mode,
  picked,
  onModeChange,
  onPickedChange,
}: {
  catalog: WebhookEventType[];
  mode: PickMode;
  picked: string[];
  onModeChange: (mode: PickMode) => void;
  onPickedChange: (picked: string[]) => void;
}) {
  const set = (types: string[], on: boolean) =>
    onPickedChange(on ? [...new Set([...picked, ...types])] : picked.filter((t) => !types.includes(t)));

  return (
    <div className="flex flex-col gap-4">
      <RadioGroup aria-label="Events" value={mode} onChange={(next) => onModeChange(next as PickMode)}>
        <RadioCard value="all" icon={RiApps2Line} title="Every event" description="Including ones added later." />
        <RadioCard value="some" icon={RiListCheck2} title="Choose events" description="Only the ones you tick." />
      </RadioGroup>
      {mode === "some" ? (
        <div className="flex flex-col gap-5">
          {groupEventTypes(catalog).map((group) => {
            const types = group.types.map((t) => t.type);
            const count = types.filter((t) => picked.includes(t)).length;

            return (
              <fieldset key={group.name} className="flex flex-col gap-2.5">
                <legend className="mb-1">
                  <Checkbox
                    isSelected={count === types.length}
                    isIndeterminate={count > 0 && count < types.length}
                    onChange={(on) => set(types, on)}
                  >
                    {group.name}
                  </Checkbox>
                </legend>
                <div className="flex flex-col gap-2 ps-7">
                  {group.types.map((t) => (
                    <Checkbox
                      key={t.type}
                      size="sm"
                      isSelected={picked.includes(t.type)}
                      onChange={(on) => set([t.type], on)}
                    >
                      <span className="flex flex-col">
                        <span className="font-mono text-body-2-regular text-text-primary">{t.type}</span>
                        <span className="text-body-2-regular text-text-tertiary">{t.description}</span>
                      </span>
                    </Checkbox>
                  ))}
                </div>
              </fieldset>
            );
          })}
        </div>
      ) : null}
    </div>
  );
}
