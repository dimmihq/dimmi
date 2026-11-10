/*
 * What a thread with Operator is about, shown under its composer beside
 * the reminder that only the team reads it.
 */
import { RiChat3Line, RiGlobalLine } from "@remixicon/react";
import type { ComponentType } from "react";

export interface Scope {
  icon: ComponentType<{ className?: string }>;
  label: string;
}

export const conversationScope: Scope = { icon: RiChat3Line, label: "This conversation" };
export const workspaceScope: Scope = { icon: RiGlobalLine, label: "Whole workspace" };
