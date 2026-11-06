/*
 * What a link to a workspace the person is not in shows: who to ask, and
 * the way back to their own inbox. Nothing about the workspace is shown,
 * not even whether it exists.
 */
import { RiLockLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { EmptyState, Panel } from "@/features/shell/page";

export function NotAMember({ slug }: { slug: string }) {
  return (
    <Panel label="Workspace not available" className="h-full flex-1">
      <EmptyState
        icon={RiLockLine}
        title="You are not in this workspace"
        description={`The link is for “${slug}”. Ask someone in it to invite you.`}
      >
        <Button size="small" variant="secondary" onClick={() => window.location.assign("/d")}>
          Go to your inbox
        </Button>
      </EmptyState>
    </Panel>
  );
}
