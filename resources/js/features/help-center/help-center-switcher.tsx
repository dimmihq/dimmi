/*
 * The help center switcher: which of the workspace's help centers the
 * page works on, and the way to add another. It shows wherever a page
 * reads one help center, the Help center page and its settings alike.
 */
import { useState } from "react";

import { Select, SelectItem } from "@/components/base/select/select";
import type { HelpCenter } from "@/lib/api/help-centers";

import { CreateHelpCenterDialog } from "./create-help-center";

const adding = "new-help-center";

export function HelpCenterSwitcher({
  workspace,
  centers,
  center,
  onPick,
  className,
}: {
  workspace: string;
  centers: HelpCenter[];
  center: HelpCenter | undefined;
  onPick: (id: string) => void;
  className?: string;
}) {
  const [creating, setCreating] = useState(false);

  return (
    <>
      <Select
        aria-label="Help center"
        size="sm"
        className={className}
        selectedKey={center?.id ?? null}
        onSelectionChange={(key) => {
          if (key === adding) {
            setCreating(true);
          } else if (key !== null) {
            onPick(String(key));
          }
        }}
        popoverClassName="w-64"
      >
        {[
          ...centers.map((c) => (
            <SelectItem key={c.id} id={c.id} textValue={c.name}>
              {c.name}
            </SelectItem>
          )),
          <SelectItem key={adding} id={adding} textValue="New help center">
            New help center…
          </SelectItem>,
        ]}
      </Select>
      {creating ? <CreateHelpCenterDialog workspace={workspace} onClose={() => setCreating(false)} /> : null}
    </>
  );
}
