import {
  RiArrowRightLine,
  RiCheckLine,
  RiFileCopyLine,
  RiGithubFill,
  RiServerLine,
  RiShieldCheckLine,
  RiTerminalBoxLine,
} from "@remixicon/react";
import { useState } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { cx } from "@/utils/cx";

import { Code } from "../components/code";
import { Intro, MonoLink, Panel, Section, Tag } from "../components/container";
import { Questions } from "../components/product-page";
import { ProductShot } from "../components/product-shot";
import { APP_URL, GITHUB_URL } from "../links";
import { FREE_CONVERSATIONS } from "../pricing/prices";
import { SERVERS } from "../pricing/servers";
import { TerminalScene } from "../scenes/terminal-scene";
import { FinalCta } from "../sections/final-cta";

const INSTALL_COMMAND = `curl -fsSL https://github.com/dimmihq/dimmi/releases/latest/download/install.sh \\
  | bash -s -- --domain support.example.com --email ops@example.com`;

const COMPOSE_SNIPPET = `# Pull the published image and compose stack
git clone https://github.com/dimmihq/dimmi.git
cd dimmi/deploy/selfhost
cp .env.example .env

# Run ghcr.io/dimmihq/dimmi with Postgres, Valkey, and Caddy
docker compose -f compose.yaml up -d`;

const UPGRADE_SNIPPET = `# Check status, back up Postgres, or pull the latest release
dimmictl status
dimmictl backup
dimmictl upgrade`;

const CLOUD_VS_SELF_HOST = [
  {
    area: "Codebase and features",
    selfHosted: "Full MIT codebase: shared inbox, AI agents, procedures, workflows, tickets, SAML SSO, and audit logs.",
    cloud: "Identical MIT codebase with every feature unlocked on day one.",
  },
  {
    area: "License and monthly fee",
    selfHosted: "$0 to dimmi forever. You pay only your VPS provider and your own model API bill.",
    cloud: `${FREE_CONVERSATIONS} free conversations every month, unlimited seats, then pay per conversation.`,
  },
  {
    area: "Hosting and data residency",
    selfHosted: "Your own Linux server, Postgres database, and local or S3 object storage.",
    cloud: "Managed European servers (Hetzner and AWS Frankfurt) with automatic backups.",
  },
  {
    area: "Email, Slack, and channels",
    selfHosted: "Bring your own SMTP/inbound domain, Meta app, and Slack app credentials.",
    cloud: "Managed inbound/outbound email, one-click Slack and Meta OAuth ready out of the box.",
  },
  {
    area: "AI model keys",
    selfHosted: "Connect your own OpenAI, Anthropic, or compatible provider keys directly.",
    cloud: "Use dimmi managed models or plug in your own API keys to drop AI fees to the human rate.",
  },
  {
    area: "Upgrades and TLS",
    selfHosted: "Caddy renews HTTPS automatically. Run dimmictl upgrade to pull a new release.",
    cloud: "Zero-downtime releases, monitoring, and security patches handled by our team.",
  },
] as const;

const QUESTIONS = [
  {
    question: "What hardware do I need to self-host dimmi?",
    answer:
      "A single Ubuntu or Debian Linux server with 2 vCPU, 4 GB of RAM, and 40 GB of disk is the minimum. Pick 4 vCPU and 8 GB of RAM if you want attachment malware scanning turned on.",
  },
  {
    question: "How do upgrades and backups work?",
    answer:
      "The installer includes dimmictl, a small CLI wrapper around the stack. Run dimmictl backup for a Postgres dump and dimmictl upgrade to pull the newest container tag with database migrations.",
  },
  {
    question: "Is any feature held back from the self-hosted edition?",
    answer:
      "No. Self-hosting runs the exact same MIT code as dimmi Cloud. SAML SSO, audit logs, AI procedures, Operator, visual workflows, and all 12 SDKs are included with zero enterprise license key.",
  },
  {
    question: "Can I run dimmi on Kubernetes?",
    answer:
      "You can deploy the ghcr.io/dimmihq/dimmi image to Kubernetes alongside Postgres and Valkey, and the container exposes /up for probes. There is no public Helm chart today, so most teams start with install.sh or Docker Compose.",
  },
  {
    question: "How does AI work on a self-hosted server?",
    answer:
      "Add your own OpenAI, Anthropic, or compatible API key under workspace settings. dimmi calls your model provider straight from your server and never meters or marks up your AI usage.",
  },
];

function trackInstallCopy(location: string) {
  const wt = (window as unknown as { webTraffic?: { track?: (name: string, data?: Record<string, string>) => void } })
    .webTraffic;
  wt?.track?.("install_copy", { location });
}

function trackGitHubClick(location: string) {
  const wt = (window as unknown as { webTraffic?: { track?: (name: string, data?: Record<string, string>) => void } })
    .webTraffic;
  wt?.track?.("github_click", { location });
}

function CopyInstallBar() {
  const [copied, setCopied] = useState(false);

  async function copyCommand() {
    try {
      await navigator.clipboard.writeText(INSTALL_COMMAND);
    } catch {
      // Clipboard API may be blocked in non-HTTPS local previews; still record copy intent.
    }
    setCopied(true);
    trackInstallCopy("self-host");
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="overflow-hidden rounded-xl border border-border-button-default bg-background-full shadow-xs">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-separator-border px-4 py-2.5">
        <span className="inline-flex items-center gap-2 font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
          <RiTerminalBoxLine className="size-4 text-accent-600 dark:text-accent-400" aria-hidden />
          One-line Linux installer
        </span>
        <button
          type="button"
          onClick={copyCommand}
          className="inline-flex items-center gap-1.5 rounded-md border border-border-button-default bg-background-secondary-default px-2.5 py-1 text-[13px] leading-5 font-medium text-text-primary transition-colors hover:bg-background-secondary-hover"
        >
          {copied ? (
            <>
              <RiCheckLine className="size-4 text-accent-600 dark:text-accent-400" aria-hidden />
              Copied
            </>
          ) : (
            <>
              <RiFileCopyLine className="size-4 text-foreground-icon-secondary" aria-hidden />
              Copy command
            </>
          )}
        </button>
      </div>
      <Code code={INSTALL_COMMAND} />
    </div>
  );
}

export function SelfHostPage() {
  return (
    <main>
      <section className="relative bg-background-full">
        <div className="relative mx-auto w-full max-w-[1200px] overflow-hidden border-x border-separator-border">
          <div
            aria-hidden
            className="pointer-events-none absolute inset-0 opacity-60 [background-image:linear-gradient(var(--color-separator-border)_1px,transparent_1px),linear-gradient(90deg,var(--color-separator-border)_1px,transparent_1px)] [background-size:64px_64px] [mask-image:radial-gradient(60%_50%_at_50%_30%,black,transparent)]"
          />
          <div className="relative flex flex-col items-center gap-6 px-5 pt-20 pb-14 text-center sm:px-12 md:pt-28">
            <nav aria-label="Breadcrumb" className="flex items-center gap-2 text-[13px] leading-5 text-text-tertiary">
              <a href="/" className="transition-colors hover:text-text-primary">
                Home
              </a>
              <span aria-hidden>/</span>
              <span className="text-text-secondary" aria-current="page">
                Self-host
              </span>
            </nav>
            <Tag className="self-center">Self-hosted · MIT</Tag>
            <h1 className="max-w-3xl text-[60px] leading-[1.04] font-medium tracking-[-0.035em] text-balance text-text-primary max-md:text-[46px] max-sm:text-[36px]">
              Own your customer support stack on one Linux server.
            </h1>
            <p className="max-w-2xl text-[18px] leading-7 text-pretty text-text-secondary">
              dimmi self-hosts on one Linux server under the MIT license for $0 to dimmi, with the exact same features
              as dimmi Cloud. Run the shared inbox, AI agents, help center, and workflows inside your own Postgres
              database in about 5 minutes.
            </p>
            <div className="mt-2 flex flex-wrap justify-center gap-3">
              <ButtonLink href="/docs/self-hosting/" trailingIcon={RiArrowRightLine}>
                Read self-hosting docs
              </ButtonLink>
              <ButtonLink
                href={GITHUB_URL}
                variant="secondary"
                leadingIcon={RiGithubFill}
                onClick={() => trackGitHubClick("self-host-hero")}
              >
                Star on GitHub
              </ButtonLink>
            </div>
            <p className="font-mono text-[12px] leading-5 text-text-tertiary">
              Published <time dateTime="2026-11-27">Nov 27, 2026</time> · Updated{" "}
              <time dateTime="2026-11-27">Nov 27, 2026</time> · Checked by Frantz Kati
            </p>
          </div>
          <div className="site-panel relative border-t border-separator-border px-5 py-10 sm:px-12">
            <div className="mx-auto flex max-w-4xl flex-col gap-8">
              <CopyInstallBar />
              <div className="flex justify-center">
                <TerminalScene />
              </div>
            </div>
          </div>
        </div>
      </section>

      <Section frameClassName="flex flex-col gap-12">
        <Intro
          title="What your server costs each month."
          body="You pay your hosting provider directly and send dimmi $0. Every size runs the Go app binary, Postgres with pgvector, Valkey, and Caddy with automatic HTTPS."
        />
        <div className="grid grid-cols-1 gap-6 md:grid-cols-3 [&>*]:min-w-0">
          {SERVERS.map((server, idx) => (
            <div
              key={server.name}
              className={cx(
                "flex flex-col justify-between gap-6 rounded-xl border p-6",
                idx === 1
                  ? "border-accent-500/50 bg-background-secondary-default/50"
                  : "border-separator-border bg-background-full",
              )}
            >
              <div className="flex flex-col gap-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="inline-flex items-center gap-2 font-mono text-[12px] leading-5 font-medium tracking-[0.04em] text-text-tertiary uppercase">
                    <RiServerLine className="size-4 text-accent-600 dark:text-accent-400" aria-hidden />
                    {server.name}
                  </span>
                  {idx === 1 ? (
                    <span className="rounded-full bg-accent-500/15 px-2.5 py-0.5 font-mono text-[11px] font-medium text-accent-700 dark:text-accent-300">
                      Most teams
                    </span>
                  ) : null}
                </div>
                <div className="flex items-baseline gap-2">
                  <span className="text-[36px] leading-none font-medium tracking-[-0.03em] text-text-primary">
                    {server.price}
                  </span>
                  <span className="text-[14px] text-text-tertiary">/ month</span>
                </div>
                <p className="text-[14px] leading-6 text-text-tertiary">{server.where}</p>
                <p className="text-[15px] leading-6 text-pretty text-text-secondary">{server.fits}</p>
              </div>
              <dl className="grid grid-cols-3 gap-3 border-t border-separator-border pt-4">
                {server.specs.map(([label, value]) => (
                  <div key={label} className="flex flex-col gap-0.5">
                    <dt className="font-mono text-[11px] tracking-[0.04em] text-text-tertiary uppercase">{label}</dt>
                    <dd className="text-[14px] font-medium text-text-primary">{value}</dd>
                  </div>
                ))}
              </dl>
            </div>
          ))}
        </div>
        <div className="flex flex-wrap items-center gap-6">
          <MonoLink href="/docs/self-hosting/install/">Full install guide</MonoLink>
          <MonoLink href="/pricing">Compare with dimmi Cloud pricing</MonoLink>
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-12">
        <Intro
          title="Standard containers, zero mystery scripts."
          body="Prefer running your own Docker Compose stack? Pull ghcr.io/dimmihq/dimmi straight from GitHub Container Registry using deploy/selfhost/compose.yaml."
        />
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 [&>*]:min-w-0">
          <div className="overflow-hidden rounded-xl border border-separator-border bg-background-full">
            <div className="border-b border-separator-border px-4 py-2.5 font-mono text-[12px] tracking-[0.04em] text-text-tertiary uppercase">
              deploy/selfhost/compose.yaml
            </div>
            <Code code={COMPOSE_SNIPPET} />
          </div>
          <div className="overflow-hidden rounded-xl border border-separator-border bg-background-full">
            <div className="border-b border-separator-border px-4 py-2.5 font-mono text-[12px] tracking-[0.04em] text-text-tertiary uppercase">
              Day-two ops with dimmictl
            </div>
            <Code code={UPGRADE_SNIPPET} />
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-6">
          <MonoLink href="/docs/self-hosting/upgrade/">Upgrade guide</MonoLink>
          <MonoLink href="/docs/self-hosting/backups/">Backup guide</MonoLink>
          <MonoLink href="/docs/self-hosting/configuration/">Configuration reference</MonoLink>
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-12">
        <Intro
          title="dimmi Cloud vs self-hosted."
          body="We do not hold back SAML SSO, audit logs, AI procedures, or workflows for paid plans. Pick self-hosting when you want data ownership; pick Cloud when you want us to run the servers."
        />
        <div className="overflow-x-auto rounded-xl border border-separator-border">
          <table className="w-full min-w-[640px] border-collapse text-left text-[15px] leading-6">
            <thead>
              <tr className="border-b border-separator-border bg-background-secondary-default/60">
                <th className="px-5 py-3.5 font-mono text-[12px] font-medium tracking-[0.04em] text-text-tertiary uppercase">
                  Area
                </th>
                <th className="px-5 py-3.5 font-mono text-[12px] font-medium tracking-[0.04em] text-accent-700 uppercase dark:text-accent-300">
                  Self-hosted (MIT)
                </th>
                <th className="px-5 py-3.5 font-mono text-[12px] font-medium tracking-[0.04em] text-text-tertiary uppercase">
                  dimmi Cloud
                </th>
              </tr>
            </thead>
            <tbody>
              {CLOUD_VS_SELF_HOST.map((row) => (
                <tr key={row.area} className="border-b border-separator-border last:border-b-0">
                  <th scope="row" className="px-5 py-4 align-top font-medium text-text-primary">
                    {row.area}
                  </th>
                  <td className="px-5 py-4 align-top text-text-secondary">{row.selfHosted}</td>
                  <td className="px-5 py-4 align-top text-text-secondary">{row.cloud}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="flex flex-wrap items-center gap-6">
          <MonoLink href="/docs/cloud-vs-self-hosted/">Read the full Cloud vs self-hosted breakdown</MonoLink>
          <MonoLink href="/inbox">See the shared inbox</MonoLink>
          <MonoLink href="/agents">See AI support agents</MonoLink>
        </div>
      </Section>

      <Section frameClassName="flex flex-col gap-12">
        <Intro
          title="The exact workspace you get on day one."
          body="No watered-down community shell. Your self-hosted server boots straight into the full shared inbox, AI agent studio, workflows, tickets, and help center."
        />
        <Panel className="px-5 pt-10 sm:px-10">
          <ProductShot
            name="inbox"
            alt="dimmi shared inbox showing live customer conversations, details sidebar, and teammate controls"
            bleed
          />
        </Panel>
        <div className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-separator-border bg-background-secondary-default/40 px-6 py-5">
          <div className="flex items-start gap-3">
            <RiShieldCheckLine className="mt-0.5 size-5 shrink-0 text-accent-600 dark:text-accent-400" aria-hidden />
            <div>
              <p className="text-[15px] font-medium text-text-primary">MIT licensed from top to bottom.</p>
              <p className="text-[14px] leading-6 text-text-secondary">
                Inspect the Go server, React dashboard, and deployment scripts on GitHub before you deploy.
              </p>
            </div>
          </div>
          <div className="flex flex-wrap gap-3">
            <ButtonLink
              href={GITHUB_URL}
              variant="secondary"
              size="small"
              leadingIcon={RiGithubFill}
              onClick={() => trackGitHubClick("self-host-proof")}
            >
              Star on GitHub
            </ButtonLink>
            <ButtonLink href={`${APP_URL}/signup`} size="small" trailingIcon={RiArrowRightLine}>
              Try dimmi Cloud free
            </ButtonLink>
          </div>
        </div>
      </Section>

      <Questions items={QUESTIONS} />
      <FinalCta />
    </main>
  );
}
