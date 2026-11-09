import { createFileRoute } from "@tanstack/react-router";

import { workspacePages } from "@/features/shell/navigation";
import { FeaturePage } from "@/features/shell/page";

const page = workspacePages.broadcasts;

export const Route = createFileRoute("/d/broadcasts")({
  component: () => <FeaturePage title={page.label} icon={page.icon} description={page.description} />,
});
