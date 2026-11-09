import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/d/help-center")({
  component: Outlet,
});
