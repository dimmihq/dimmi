import { ArrowCounterClockwise, Files, GitMerge, Globe, ShieldCheck, Sliders } from "@phosphor-icons/react";

import { ChannelLinks, FeatureGrid, ProductHero, Questions, Rows } from "../../components/product-page";
import { ShotStill } from "../../components/shot-still";
import { ShotVideo } from "../../components/shot-video";
import { FinalCta } from "../../sections/final-cta";

const rows = [
  {
    label: "Your domain",
    title: "Forward your email and send replies from your own address.",
    body: "Keep Gmail, Outlook, or Zoho where it is. Forward your support address to dimmi and add three DNS records so replies go out from your domain and land in the inbox.",
    visual: (
      <ShotStill
        name="channel-email-domain"
        alt="Sending domain settings in dimmi listing SPF, DKIM, and DMARC DNS records with copy buttons and forwarding test status"
      />
    ),
  },
  {
    label: "Team replies",
    title: "Stop two teammates from replying to the same email.",
    body: "See when a teammate is already viewing or typing in a thread. Leave private notes for your team and check past orders right beside the reply box.",
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
    label: "Routing",
    title: "Send support@, billing@, and security@ to different teams.",
    body: "Connect as many email addresses as you use. Give each address its own inbox, reply timer, workflow, and AI agent, or keep private addresses visible only to chosen teammates.",
    visual: (
      <ShotStill
        name="channel-email-routing"
        alt="Email channel settings routing support@, billing@, and security@ addresses into dedicated inboxes with their own SLAs, workflows, and support agents"
      />
    ),
  },
  {
    label: "Signatures",
    title: "Plain or branded emails with personal signatures.",
    body: "Send plain text replies or branded emails with file attachments and teammate signatures. If bounce rates climb, dimmi pauses sending to protect your domain.",
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
    title: "Filter out newsletters",
    body: "Automatically close or drop marketing emails and automated alerts before they clutter your inbox.",
  },
  {
    icon: ArrowCounterClockwise,
    title: "Undo send",
    body: "Spot a typo or missing file? Pull the email back before it leaves.",
  },
  {
    icon: GitMerge,
    title: "Merge email and chat",
    body: "Combine a customer's email and live chat into one thread when they message you twice.",
  },
  {
    icon: Files,
    title: "Saved replies",
    body: "Insert common answers with the customer's name filled in, then tag or close the thread in one click.",
  },
  {
    icon: ShieldCheck,
    title: "Daily DNS checks",
    body: "dimmi checks your domain records every day and warns you if anything breaks.",
  },
  {
    icon: Globe,
    title: "Bring your own mail server",
    body: "Self-host with SMTP, Amazon SES, Postmark, Mailgun, SendGrid, Resend, or Cloudflare Email Routing.",
  },
];

const questions = [
  {
    question: "Do we have to leave Google Workspace or Microsoft 365?",
    answer:
      "Keep your current email provider. You forward your support address to dimmi and add DNS records so replies send from your domain, while your old mailbox stays as a backup.",
  },
  {
    question: "Will customers see a dimmi email address when we reply?",
    answer:
      "Once your domain is verified, replies come from your own address like support@yourdomain.com. Before you verify, you can still test replies right away using your forwarding address.",
  },
  {
    question: "Can AI agents answer support emails too?",
    answer:
      "Yes, on the addresses and hours you pick. Agents read your help center, follow your rules, sign off with your signature, and hand over to a person when needed.",
  },
  {
    question: "Do you charge extra for each email address or domain?",
    answer:
      "Connect support@, billing@, or different brand domains for $0 extra. Email threads count under normal dimmi Cloud pricing after 100 free conversations each month, and self-hosting is $0.",
  },
  {
    question: "Should we use dimmi for personal company email?",
    answer:
      "Stick with Gmail, Outlook, or Superhuman for 1-on-1 personal email or cold sales outreach. dimmi is built for shared team support inboxes and customer updates.",
  },
  {
    question: "How does email work when we self-host dimmi?",
    answer:
      "Connect your own mail server or provider: SMTP, Amazon SES, Postmark, Mailgun, SendGrid, Resend, or Cloudflare Email Routing.",
  },
];

export function EmailChannelPage() {
  return (
    <>
      <ProductHero
        eyebrow="Email"
        breadcrumb={[
          { label: "Home", href: "/" },
          { label: "Channels", href: "/channels" },
        ]}
        title="Answer unlimited support@ emails together in one inbox."
        body="Forward your support email to dimmi and send replies from your own domain. See who is replying, leave private notes, and let AI agents help."
        visual={
          <ShotVideo
            name="channel-email-hero"
            poster="channel-email-hero"
            alt="A customer email in Gmail on the left synced live with the dimmi shared inbox on the right, showing internal notes and a reply going out from support@stowline.co"
            bleed
          />
        }
      />
      <Rows title="Everything a shared email inbox should do." rows={rows} />
      <FeatureGrid title="Handy tools for busy email inboxes." items={more} />
      <ChannelLinks current="/channels/email" />
      <Questions body="What teams ask before connecting support@ to dimmi." items={questions} />
      <FinalCta />
    </>
  );
}
