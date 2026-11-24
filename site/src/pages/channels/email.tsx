import { ArrowCounterClockwise, Files, GitMerge, Globe, ShieldCheck, Sliders } from "@phosphor-icons/react";

import { FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Your domain",
    title: "Forward your address and verify your domain in minutes.",
    body: "Keep your mail provider and test forwarding with one click. Copy your SPF, DKIM, and DMARC records so replies leave from your own domain and stay out of spam.",
    visual: (
      <ShotStill
        name="channel-email-domain"
        alt="Sending domain settings in dimmi listing SPF, DKIM, and DMARC DNS records with copy buttons and forwarding test status"
      />
    ),
  },
  {
    label: "Shared inbox",
    title: "No more double replies or forwarded threads in Slack.",
    body: "Live presence warns you when a teammate is already viewing or typing. Long replies stay in one clean thread, with private internal notes and customer history beside the composer.",
    visual: (
      <ShotVideo
        name="channel-email-thread"
        poster="channel-email-thread"
        alt="An email thread in the dimmi inbox showing live collision prevention, internal teammate notes, and customer context beside the composer"
        maxWidth={520}
      />
    ),
  },
  {
    label: "Routing & agents",
    title: "Route support@, billing@, and security@ to the right team.",
    body: "Connect as many addresses as you run. Point each one at its own inbox, SLA policy, workflow, and support agent, while private queues stay visible only to chosen teammates.",
    visual: (
      <ShotStill
        name="channel-email-routing"
        alt="Email channel settings routing support@, billing@, and security@ addresses into dedicated inboxes with their own SLAs, workflows, and support agents"
      />
    ),
  },
  {
    label: "Polish & delivery",
    title: "Personal signatures, clean attachments, and broadcasts customers can reply to.",
    body: "Choose plain or branded emails with personal signatures and file attachments. dimmi pauses sending if bounces or spam complaints spike, and broadcast replies open back in your inbox.",
    visual: (
      <ShotStill
        name="channel-email-signature"
        alt="Email reply style and signature editor in dimmi showing plain and branded templates, dynamic variables, attachments, and bounce protection"
      />
    ),
  },
];

const more = [
  {
    icon: Sliders,
    title: "Bulk mail filters",
    body: "Close, keep open, or drop newsletters and automated alerts before they clutter your queue.",
  },
  {
    icon: ArrowCounterClockwise,
    title: "Undo send",
    body: "Pull an outgoing email back before it leaves if you spot a typo or wrong file.",
  },
  {
    icon: GitMerge,
    title: "Merge across channels",
    body: "Combine a customer's email and live chat into one thread when they write in twice.",
  },
  {
    icon: Files,
    title: "Saved replies and variables",
    body: "Insert reusable answers with customer names, then tag, assign, or close the thread in one step.",
  },
  {
    icon: ShieldCheck,
    title: "Live domain monitoring",
    body: "dimmi rechecks your DNS records each day and falls back to forwarding if a record goes missing.",
  },
  {
    icon: Globe,
    title: "Bring your own mail server",
    body: "Self-host with SMTP, Amazon SES, Postmark, Mailgun, SendGrid, Resend, or Cloudflare Email Routing.",
  },
];

const questions = [
  {
    question: "Do we have to move our MX records or leave Google Workspace?",
    answer:
      "Keep Google Workspace, Microsoft 365, Zoho, or Cloudflare where it is. Set up forwarding to dimmi, add DNS records to verify your sending domain, and keep your original mailbox as a backup.",
  },
  {
    question: "Will customers see a dimmi address on our replies?",
    answer:
      "Once your sending domain passes DNS checks, replies leave from your own address like support@yourdomain.com. Before you verify, dimmi sends from your forwarding address so you can test right away.",
  },
  {
    question: "Can our support agents reply to emails too?",
    answer:
      "Yes, on the addresses and hours you pick. Agents read your help center, follow your procedures, sign off with your team signature, and hand over to a teammate with a private summary when a human should step in.",
  },
  {
    question: "Do you charge extra per email address or sending domain?",
    answer:
      "Connect support@, billing@, or separate brand domains with no extra address fees. Email threads use standard dimmi conversation pricing on dimmi Cloud, and self-hosting costs $0.",
  },
  {
    question: "Can dimmi replace Gmail or Superhuman for personal company email?",
    answer:
      "Stick with Gmail, Outlook, or Superhuman for 1-on-1 company mail or cold sales outreach. dimmi is built for shared support queues and customer broadcasts where teams answer together.",
  },
  {
    question: "How does email work if we self-host dimmi?",
    answer:
      "Connect your own mail provider: SMTP, Amazon SES, Postmark, Mailgun, SendGrid, Resend, or Cloudflare Email Routing. With an SMTP relay, dimmi signs outbound mail with DKIM and lets you rotate keys without downtime.",
  },
];

export function EmailChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Email"
        title="Turn support@ into a shared inbox on your own domain."
        body="Forward your support addresses into dimmi and reply from your verified domain. Collision alerts, internal notes, and AI support agents keep every thread moving."
        visual={
          <ShotVideo
            name="channel-email-hero"
            poster="channel-email-hero"
            alt="An email conversation in the dimmi inbox showing a customer thread, internal teammate notes, customer details, and a reply going out from support@acme.com"
            bleed
          />
        }
      />
      <Rows title="Everything a shared Gmail login breaks, fixed in one inbox." rows={rows} />
      <FeatureGrid title="Built for the messy parts of support email." items={more} />
      <Questions body="What teams ask before pointing support@ at dimmi." items={questions} />
      <FinalCta />
    </>
  );
}
