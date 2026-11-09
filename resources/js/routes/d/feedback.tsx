import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/d/feedback")({
  component: Outlet,
});
