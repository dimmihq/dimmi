import { createFileRoute } from "@tanstack/react-router";

import { workspacePages } from "@/features/shell/navigation";
import { FeaturePage } from "@/features/shell/page";

const page = workspacePages.changelog;

export const Route = createFileRoute("/d/changelog")({
  component: () => <FeaturePage title={page.label} icon={page.icon} description={page.description} />,
});
