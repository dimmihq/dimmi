import { useEffect, useRef, useState } from "react";

import { cx } from "@/utils/cx";

import { Code } from "../components/code";
import { Intro, MonoLink, Panel, Section } from "../components/container";
import { GITHUB_URL } from "../links";
import { TerminalScene } from "../scenes/terminal-scene";
import { useStartInView } from "../scenes/use-scene";

const TAB_DURATION = 4200;

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
    name: "Go",
    code: `import dimmi "github.com/dimmihq/dimmi-go"

client, _ := dimmi.New(
  dimmi.WithToken(os.Getenv("DIMMI_TOKEN")),
)

res, _ := client.API.ConversationsIndexWithResponse(
  ctx, nil,
)`,
  },
  {
    name: "Python",
    code: `from dimmi import Dimmi

client = Dimmi(token="dmi_...")

# Paginate open conversations with automatic retries.
for thread in client.paginate("/conversations"):
  print(thread["id"], thread["status"])`,
  },
  {
    name: "Events",
    code: `import { loadDimmi } from "@dimmi/browser";

const dimmi = loadDimmi({ appId: "wgt_..." });

// Workflows and audiences act on tracked events.
dimmi.trackEvent("plan_upgraded", { plan: "growth" });`,
  },
];

function CodeCard() {
  const { ref, started, inView } = useStartInView<HTMLDivElement>(0.35);
  const [active, setActive] = useState(0);
  const [paused, setPaused] = useState(false);
  const [cycle, setCycle] = useState(0);
  const pausedRef = useRef(false);
  pausedRef.current = paused;

  useEffect(() => {
    if (!started || !inView || paused) {
      return;
    }
    const timer = setTimeout(() => {
      if (!pausedRef.current) {
        setActive((current) => (current + 1) % snippets.length);
      }
    }, TAB_DURATION);
    return () => clearTimeout(timer);
  }, [started, inView, paused, active, cycle]);

  const choose = (index: number) => {
    setActive(index);
    setCycle((c) => c + 1);
  };

  return (
    <div
      ref={ref}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
      className="w-full overflow-hidden rounded-t-xl border border-b-0 border-border-button-default bg-background-full shadow-[0_24px_60px_-28px_rgb(0_0_0/0.25)]"
    >
      <div role="tablist" aria-label="Code examples" className="flex gap-1 border-b border-separator-border px-3 py-2">
        {snippets.map((snippet, i) => (
          <button
            key={snippet.name}
            type="button"
            role="tab"
            aria-selected={active === i}
            onClick={() => choose(i)}
            className={cx(
              "relative overflow-hidden rounded-md px-2.5 py-1 text-body-medium transition-colors",
              active === i
                ? "bg-background-secondary-default text-text-primary"
                : "text-text-tertiary hover:text-text-secondary",
            )}
          >
            {snippet.name}
            {active === i && started && inView && !paused ? (
              <span
                key={`${active}-${cycle}`}
                aria-hidden
                className="absolute inset-x-0 bottom-0 h-0.5 bg-accent-500"
                style={{ animation: `site-fill-x ${TAB_DURATION}ms linear forwards` }}
              />
            ) : null}
          </button>
        ))}
      </div>
      <div key={snippets[active].name} className="site-code-enter h-[268px]">
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
        title="Open source with nothing held back."
        body="Self-hosting runs the exact same MIT code as dimmi Cloud: SAML SSO, AI playbooks, workflows, and all 11 SDKs. Keep every byte in your own Postgres."
      />
      <div className="grid grid-cols-1 gap-x-6 gap-y-14 lg:grid-cols-2 [&>*]:min-w-0">
        <Card
          anchor="self-host"
          title="Self-host"
          body="Run one script on a Linux server for HTTPS, Postgres, and easy updates with dimmictl. You pay your host, and we take $0."
          link={{ href: GITHUB_URL, label: "View on GitHub" }}
        >
          <TerminalScene />
        </Card>
        <Card
          anchor="developers"
          title="SDKs and API"
          body="Build with typed SDKs for 11 stacks, a REST API, signed webhooks, an MCP server, and llms.txt for coding agents."
          link={{ href: "/docs", label: "Read the docs" }}
        >
          <CodeCard />
        </Card>
      </div>
    </Section>
  );
}
