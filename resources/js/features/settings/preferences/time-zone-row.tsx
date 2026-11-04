/*
 * The person's own time zone, or the workspace's when they chose none.
 * Picking saves at once. A browser somewhere else only earns a quiet offer
 * to switch: nothing here ever changes the setting on its own.
 */
import { RiArrowDownSLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo } from "react";
import { Button as AriaButton, ComboBox, ListBox, Popover, useFilter } from "react-aria-components";

import { InputBase } from "@/components/base/input/input";
import { SelectItem } from "@/components/base/select/select";
import { updateProfile } from "@/lib/api/account";
import { workspaceQuery } from "@/lib/api/workspace";
import { browserZone, shouldOfferZone, zoneChoices } from "@/lib/location";
import { cx } from "@/utils/cx";

import { Action } from "../controls";
import { Row } from "../layout";
import { useAccount } from "../use-account";

/*
 * followWorkspace is the choice that clears the person's own time zone.
 */
const followWorkspace = "workspace-default";

interface Choice {
  id: string;
  label: string;
}

export function TimeZoneRows() {
  const me = useAccount();
  const queryClient = useQueryClient();
  const { data: workspace } = useQuery(workspaceQuery(me.workspace.id));
  const { contains } = useFilter({ sensitivity: "base" });
  const fallback = workspace?.timezone ?? "UTC";
  const browser = browserZone();

  const save = useMutation({
    mutationFn: (timezone: string | null) => updateProfile({ timezone }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  });

  const choices = useMemo<Choice[]>(
    () => [
      { id: followWorkspace, label: `Workspace default (${fallback})` },
      ...zoneChoices(me.timezone, Intl.supportedValuesOf("timeZone")).map((zone) => ({ id: zone, label: zone })),
    ],
    [fallback, me.timezone],
  );

  return (
    <>
      <Row label="Time zone" description={save.error?.message ?? "Used for times in the inbox and in notifications."}>
        <ComboBox
          aria-label="Time zone"
          className="w-64"
          key={`${me.timezone}-${fallback}`}
          defaultItems={choices}
          defaultSelectedKey={me.timezone ?? followWorkspace}
          defaultFilter={contains}
          menuTrigger="focus"
          onSelectionChange={(key) => {
            if (key !== null && key !== (me.timezone ?? followWorkspace)) {
              save.mutate(key === followWorkspace ? null : String(key));
            }
          }}
        >
          <InputBase
            size="small"
            trailingAddon={
              <AriaButton aria-label="Show time zones" className="flex items-center text-foreground-icon-tertiary">
                <RiArrowDownSLine className="size-4" />
              </AriaButton>
            }
          />
          <Popover
            offset={4}
            className={cx(
              "w-64 overflow-hidden rounded-xl border border-border-button-default",
              "bg-background-primary-default p-2 shadow-lg",
            )}
          >
            <ListBox<Choice> className="flex max-h-60 flex-col gap-1 overflow-auto outline-none">
              {(choice) => (
                <SelectItem id={choice.id} textValue={choice.label}>
                  {choice.label}
                </SelectItem>
              )}
            </ListBox>
          </Popover>
        </ComboBox>
      </Row>
      {shouldOfferZone(me.timezone, fallback, browser) ? (
        <Row label={`Your browser is on ${browser}`} description="Times show in the workspace's zone until you switch.">
          <Action onPress={() => save.mutate(browser)}>{`Use ${browser}`}</Action>
        </Row>
      ) : null}
    </>
  );
}
