import { useState } from "react";

import { cx } from "@/utils/cx";

import { Code } from "../components/code";
import { Intro, MonoLink, Panel, Section } from "../components/container";
import { GITHUB_URL } from "../links";
import { TerminalScene } from "../scenes/terminal-scene";

const snippets = [
  {
    name: "React",
    code: `import { DimmiProvider, DimmiButton }
  from "@dimmi/react";

export function App() {
  return (
    <DimmiProvider appId="wgt_...">
      <DimmiButton>Chat with us</DimmiButton>
    </DimmiProvider>
  );
}`,
  },
  {
    name: "Browser",
    code: `import { loadDimmi } from "@dimmi/browser";

const dimmi = loadDimmi({
  appId: "wgt_...",
});

dimmi.showNewMessage("I need help with billing");`,
  },
  {
    name: "Events",
    code: `import { loadDimmi } from "@dimmi/browser";

const dimmi = loadDimmi({ appId: "wgt_..." });

// Workflows and audiences can act on anything you track.
dimmi.trackEvent("plan_upgraded", { plan: "growth" });`,
  },
];

function CodeCard() {
  const [active, setActive] = useState(0);
  return (
    <div className="w-full overflow-hidden rounded-t-xl border border-b-0 border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]">
      <div role="tablist" aria-label="Code examples" className="flex gap-1 border-b border-separator-border px-3 py-2">
        {snippets.map((snippet, i) => (
          <button
            key={snippet.name}
            type="button"
            role="tab"
            aria-selected={active === i}
            onClick={() => setActive(i)}
            className={cx(
              "rounded-md px-2.5 py-1 text-body-medium transition-colors",
              active === i
                ? "bg-background-secondary-default text-text-primary"
                : "text-text-tertiary hover:text-text-secondary",
            )}
          >
            {snippet.name}
          </button>
        ))}
      </div>
      <div className="h-[268px]">
        <Code code={snippets[active].code} />
      </div>
    </div>
  );
}

function Card({
  anchor,
  title,
  body,
  link,
  children,
}: {
  anchor: string;
  title: string;
  body: string;
  link: { href: string; label: string };
  children: React.ReactNode;
}) {
  return (
    <div id={anchor} className="flex scroll-mt-24 flex-col gap-6">
      <Panel className="flex h-[400px] items-end px-6 pt-10 sm:px-10">{children}</Panel>
      <div className="flex flex-col gap-2">
        <h3 className="text-[24px] leading-8 font-medium tracking-[-0.02em] text-text-primary">{title}</h3>
        <p className="max-w-[480px] text-[16px] leading-7 text-pretty text-text-secondary">{body}</p>
        <span className="mt-2">
          <MonoLink href={link.href}>{link.label}</MonoLink>
        </span>
      </div>
    </div>
  );
}

export function SelfHost() {
  return (
    <Section frameClassName="flex flex-col gap-16">
      <Intro
        title="Open source. Actually open."
        body="MIT license, no enterprise edition held back. Run it on our cloud or your servers, and build on it with the SDKs and the API."
      />
      <div className="grid grid-cols-1 gap-x-6 gap-y-14 lg:grid-cols-2 [&>*]:min-w-0">
        <Card
          anchor="self-host"
          title="Self-host"
          body="One command installs it on a server, with HTTPS and updates. It's the same product as our cloud, and we never see a cent."
          link={{ href: GITHUB_URL, label: "View on GitHub" }}
        >
          <TerminalScene />
        </Card>
        <Card
          anchor="developers"
          title="SDKs and API"
          body="Typed SDKs for 11 stacks, a REST API, signed webhooks, and llms.txt for coding agents."
          link={{ href: "/docs", label: "Read the docs" }}
        >
          <CodeCard />
        </Card>
      </div>
    </Section>
  );
}
