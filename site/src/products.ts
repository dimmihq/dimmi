/*
 * Every product page, grouped the way the navigation's Product menu
 * shows them. The menu, and anything else that lists the products,
 * reads from here.
 */
import {
  BookOpenText,
  CalendarCheck,
  ChartLineUp,
  FlowArrow,
  Lightbulb,
  ListChecks,
  MagicWand,
  Megaphone,
  Newspaper,
  type Icon as PhosphorIcon,
  Sparkle,
  Ticket,
  Tray,
} from "@phosphor-icons/react";

export type Product = { name: string; href: string; blurb: string; icon: PhosphorIcon };

export const PRODUCT_GROUPS: { title: string; products: Product[] }[] = [
  {
    title: "AI",
    products: [
      { name: "Agents", href: "/agents", blurb: "Specialized support agents that cite your docs.", icon: Sparkle },
      {
        name: "Procedures",
        href: "/procedures",
        blurb: "Support policies your agents cannot break.",
        icon: ListChecks,
      },
      { name: "Operator", href: "/operator", blurb: "Backstage AI teammate for your support team.", icon: MagicWand },
      {
        name: "Routines",
        href: "/routines",
        blurb: "Operator on a clock for doc sync and weekly QA.",
        icon: CalendarCheck,
      },
    ],
  },
  {
    title: "Support",
    products: [
      { name: "Inbox", href: "/inbox", blurb: "Every channel, unlimited free seats.", icon: Tray },
      { name: "Tickets", href: "/tickets", blurb: "Tracked work linked to engineering.", icon: Ticket },
      { name: "Workflows", href: "/workflows", blurb: "Visual routing, SLAs, and API steps.", icon: FlowArrow },
      { name: "Reports", href: "/reports", blurb: "Support metrics you can click into.", icon: ChartLineUp },
    ],
  },
  {
    title: "Engage",
    products: [
      { name: "Help center", href: "/help-center", blurb: "Docs for customers and your agents.", icon: BookOpenText },
      { name: "Broadcasts", href: "/broadcasts", blurb: "Updates people can reply to.", icon: Megaphone },
      { name: "Changelog", href: "/changelog", blurb: "Release notes in app and email.", icon: Newspaper },
      { name: "Feedback", href: "/feedback", blurb: "Feature votes and public roadmap.", icon: Lightbulb },
    ],
  },
];

export const PRODUCT_PATHS = new Set(PRODUCT_GROUPS.flatMap((group) => group.products.map((p) => p.href)));
