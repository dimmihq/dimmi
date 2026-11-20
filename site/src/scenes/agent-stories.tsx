/*
 * The AI agent's three stories, each a scene: answering from the help
 * center, running a procedure that needs an approval, and handing a
 * conversation to the right team with a summary. Drawn with the app's
 * own pieces: BoardUI surfaces, avatars, chips and the agent-thinking
 * indicator.
 */
import { RiArrowRightLine, RiBookOpenLine, RiShieldUserLine, RiStickyNoteLine } from "@remixicon/react";
import { motion } from "motion/react";
import type { ReactNode } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Chip } from "@/components/base/badges/chip";

import { Agent, Customer, Procedure, Source, Thinking } from "./agent-pieces";
import { enter } from "./motion";

/* Each story is a list of frames; frame n shows the first n+1 items. */
export type Story = { channel: string; frames: ReactNode[] };

export const STORIES: Story[] = [
  {
    channel: "Messenger",
    frames: [
      <Customer key="q" name="Hana Kowalski" initials="HK">
        How do I add a teammate to our workspace?
      </Customer>,
      <Thinking key="t" label="Searching your help center" />,
      <Agent key="a" footer={<Source icon={RiBookOpenLine}>Help center · Invite teammates</Source>}>
        Open Settings, then Team, and choose Invite. They get an email link that is good for 7 days, and you pick their
        role before you send it.
      </Agent>,
      <Customer key="r" name="Hana Kowalski" initials="HK">
        Found it, thanks!
      </Customer>,
    ],
  },
  {
    channel: "Email",
    frames: [
      <Customer key="q" name="Omar Haddad" initials="OH">
        We were charged twice for November. Can you refund one?
      </Customer>,
      <Procedure
        key="p1"
        name="Refund a duplicate charge"
        steps={[{ label: "Look up the account", detail: "Northwind Labs · Growth plan", state: "done" }]}
      />,
      <Procedure
        key="p2"
        name="Refund a duplicate charge"
        steps={[
          { label: "Look up the account", detail: "Northwind Labs · Growth plan", state: "done" },
          { label: "Find November charges", detail: "2 × $49.00 on Nov 3", state: "done" },
          { label: "Refund $49.00", detail: "Waiting for approval from Billing", state: "waiting" },
        ]}
      />,
      <Procedure
        key="p3"
        name="Refund a duplicate charge"
        steps={[
          { label: "Look up the account", detail: "Northwind Labs · Growth plan", state: "done" },
          { label: "Find November charges", detail: "2 × $49.00 on Nov 3", state: "done" },
          { label: "Refund $49.00", detail: "Approved by Priya Moreau", state: "approved" },
        ]}
      />,
      <Agent key="a">
        Done. I refunded the duplicate $49.00 charge from November 3. It shows on your card in 5 to 10 days.
      </Agent>,
    ],
  },
  {
    channel: "Slack",
    frames: [
      <Customer key="q" name="Sven Larsen" initials="SL">
        SSO broke for our whole team after we rotated the SAML certificate.
      </Customer>,
      <Thinking key="t" label="Checking your procedures" />,
      <Agent key="a">This needs someone from our identity team. I've passed it on with everything you told me.</Agent>,
      <motion.div
        key="n"
        {...enter}
        className="flex flex-col gap-2.5 rounded-xl border border-amber-300/60 bg-amber-50 px-3.5 py-3 dark:border-amber-500/30 dark:bg-amber-950/40"
      >
        <span className="inline-flex items-center gap-1.5 text-caption-1-semibold text-amber-800 dark:text-amber-300">
          <RiStickyNoteLine className="size-3.5" aria-hidden />
          Handoff note
        </span>
        <p className="text-body-regular text-text-primary">
          Customer rotated their SAML signing certificate today; every sign-in now fails. Not a password issue.
          Enterprise plan, 140 seats.
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <Chip variant="caption" color="rose">
            Priority: high
          </Chip>
          <span className="inline-flex items-center gap-1 text-caption-1-medium text-text-secondary">
            <RiShieldUserLine className="size-3.5" aria-hidden />
            Identity team
            <RiArrowRightLine className="size-3" aria-hidden />
            <Avatar size="xs" initials="PM" color="lime" />
            Priya Moreau
          </span>
        </div>
      </motion.div>,
    ],
  },
];
