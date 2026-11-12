import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/d/reports")({
  component: Outlet,
});
