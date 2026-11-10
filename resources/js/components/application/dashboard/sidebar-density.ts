import { createContext } from "react";

/**
 * Row density. `regular` is the Figma 36px row (8px padding around a 20px
 * icon). `compact` is a 32px row with an 18px icon and
 * tighter section spacing, for sidebars with many pages such as settings.
 * The account menu beside the sidebar reads it too, so its rows match the
 * sidebar's.
 */
export type SidebarDensity = "regular" | "compact";

export const DensityContext = createContext<SidebarDensity>("regular");

export const density = {
  regular: { row: "p-2", icon: "size-5", text: "text-body-medium", rail: "w-9", rows: "gap-1", heading: "h-7" },
  compact: { row: "px-2 py-1.5", icon: "size-[18px]", text: "text-body-medium", rail: "w-8", rows: "gap-0.5", heading: "h-6" },
} as const;
