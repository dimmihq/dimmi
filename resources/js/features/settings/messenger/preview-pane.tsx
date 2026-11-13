/*
 * The preview messenger in a column of its own beside the settings. The
 * column is wider than the messenger's phone layout, so it shows as on a
 * desktop. Each config it is given shows on it at once.
 */
import { Panel } from "@/features/shell/page";

import { MessengerFrame } from "./messenger-frame";

export function PreviewPane({ config }: { config: Record<string, unknown> | undefined }) {
  return (
    <Panel label="Preview" className="hidden h-full w-[500px] shrink-0 xl:flex">
      <MessengerFrame title="Messenger preview" commands={[["configure", config]]} />
    </Panel>
  );
}
