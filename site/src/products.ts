/*
 * Every product page, grouped the way the navigation's Product menu
 * shows them. The menu, and anything else that lists the products,
 * reads from here.
 */
import {
  BookOpenText,
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
      { name: "AI agent", href: "/agent", blurb: "Answers, acts and hands over.", icon: Sparkle },
      { name: "Operator", href: "/operator", blurb: "An AI teammate in the inbox.", icon: MagicWand },
      { name: "Procedures", href: "/procedures", blurb: "Steps the agent follows.", icon: ListChecks },
    ],
  },
  {
    title: "Support",
    products: [
      { name: "Inbox", href: "/inbox", blurb: "Every channel in one queue.", icon: Tray },
      { name: "Tickets", href: "/tickets", blurb: "Track work past the chat.", icon: Ticket },
      { name: "Workflows", href: "/workflows", blurb: "Automate routing and replies.", icon: FlowArrow },
      { name: "Reports", href: "/reports", blurb: "Volume, speed and satisfaction.", icon: ChartLineUp },
    ],
  },
  {
    title: "Engage",
    products: [
      { name: "Help center", href: "/help-center", blurb: "Articles people and AI read.", icon: BookOpenText },
      { name: "Broadcasts", href: "/broadcasts", blurb: "Emails that get replies.", icon: Megaphone },
      { name: "Changelog", href: "/changelog", blurb: "Tell customers what shipped.", icon: Newspaper },
      { name: "Feedback", href: "/feedback", blurb: "Collect and rank requests.", icon: Lightbulb },
    ],
  },
];

export const PRODUCT_PATHS = new Set(PRODUCT_GROUPS.flatMap((group) => group.products.map((p) => p.href)));
