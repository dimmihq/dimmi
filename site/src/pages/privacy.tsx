import { LEGAL_CONTACTS, LEGAL_ENTITY, LEGAL_VERSION, RETENTION_SCHEDULE } from "../legal/constants";
import {
  Clause,
  LegalDocument,
  LegalEntityFooter,
  LegalLink,
  LegalTable,
  NoticeBox,
  PartHeader,
  type PartSummary,
  SubHeading,
} from "../legal/layout";
import { PrivacyDpaPart, PrivacySupportingParts } from "../legal/privacy-parts";

const parts: PartSummary[] = [
  {
    id: "privacy-policy",
    numeral: "I",
    title: "Privacy Policy",
    description: "Controller data, legal bases, EU/US transfers, retention, and regional rights.",
    items: [
      { id: "short-version", label: "1. The short version" },
      { id: "who-we-are", label: "2. Who we are" },
      { id: "whos-who", label: "3. Who's who (roles)" },
      { id: "end-customers", label: "4. If you are an end customer" },
      { id: "what-we-collect", label: "5. What we collect" },
      { id: "legal-bases", label: "6. Purposes & legal bases" },
      { id: "ai-features", label: "7. AI features" },
      { id: "sharing", label: "8. Who we share data with" },
      { id: "transfers", label: "9. Storage & transfers" },
      { id: "retention", label: "10. Retention periods" },
      { id: "security-summary", label: "11. Security & cookies" },
      { id: "rights", label: "12. Your privacy rights" },
      { id: "automated-children", label: "13. Automated decisions & children" },
      { id: "google-github", label: "14. Google, GitHub & self-hosted" },
      { id: "changes-contact", label: "15. Changes, contact & versions" },
    ],
  },
  {
    id: "end-user-notice",
    numeral: "II",
    title: "End-User Notice",
    description: "In-software notice for people chatting through a business's dimmi messenger.",
    items: [
      { id: "eu-controller", label: "II.1 Who controls your data" },
      { id: "eu-collection", label: "II.2 Collection, storage & rights" },
    ],
  },
  {
    id: "cookie-notice",
    numeral: "III",
    title: "Cookie Notice",
    description: "Every first-party cookie and browser storage key. Zero analytics or ads.",
    items: [{ id: "cookies-inventory", label: "III.1 Complete cookie table" }],
  },
  {
    id: "subprocessors",
    numeral: "IV",
    title: "Subprocessors",
    description: "Hetzner, AWS Frankfurt, Cloudflare Clef, Anthropic, OpenAI, Voyage, Paystack.",
    items: [
      { id: "sub-infra", label: "IV.1 Infrastructure & email" },
      { id: "sub-ai", label: "IV.2 AI & search (optional)" },
      { id: "sub-billing", label: "IV.3 Payment (Paystack)" },
      { id: "sub-other", label: "IV.4 Other services & log" },
    ],
  },
  {
    id: "ai-transparency",
    numeral: "V",
    title: "AI Transparency",
    description: "Models, per-call data, default PII masking, zero training, and handover.",
    items: [{ id: "ai-flows", label: "V.1 Models, masking & retention" }],
  },
  {
    id: "security-overview",
    numeral: "VI",
    title: "Security Overview",
    description: "Implemented controls, EU hosting, honest boundaries, and reporting.",
    items: [{ id: "sec-controls", label: "VI.1 Controls & disclosure" }],
  },
  {
    id: "dpa",
    numeral: "VII",
    title: "Data Processing Agreement",
    description: "Global DPA with Annexes 1 to 5 (NDPA, GAID, EU SCCs, UK, Swiss, CCPA).",
    items: [
      { id: "dpa-parties", label: "VII.1 Parties & roles" },
      { id: "dpa-instructions", label: "VII.2 Instructions & AI" },
      { id: "dpa-subprocessors", label: "VII.3 Subprocessors & security" },
      { id: "dpa-incidents", label: "VII.4 DSRs, 72h breach & audits" },
      { id: "dpa-deletion", label: "VII.5 Deletion & transfers" },
      { id: "dpa-gaid", label: "VII.6 Risks, liability & insurance" },
      { id: "annex-1", label: "Annex 1: Processing & retention" },
      { id: "annex-2", label: "Annex 2: Security measures" },
      { id: "annex-3", label: "Annex 3: Subprocessors" },
      { id: "annex-4", label: "Annex 4: Transfer mechanisms" },
      { id: "annex-5", label: "Annex 5: Regional terms" },
    ],
  },
];

export function PrivacyPage() {
  const controllerRetention = RETENTION_SCHEDULE.filter((row) => row.scope === "Controller");

  return (
    <LegalDocument
      path="/privacy"
      title="Privacy Policy & Data Governance"
      subtitle="Everything governing privacy, cookies, subprocessors, AI transparency, security, and our global Data Processing Agreement (DPA) for dimmi Cloud in one complete page."
      parts={parts}
    >
      <NoticeBox title="Controller data vs. customer workspaces" tone="emphasis">
        <p>
          Part I covers personal data that{" "}
          <strong className="font-medium text-text-primary">{LEGAL_ENTITY.name}</strong> controls directly: website
          visitors, dimmi Cloud account holders, billing contacts, and changelog subscribers. Data inside a business
          customer's workspace (support conversations and end-customer profiles) is processed on that business's behalf
          under <LegalLink href="#dpa">Part VII (Data Processing Agreement)</LegalLink>. If you are chatting with a
          business that uses dimmi, jump to{" "}
          <LegalLink href="#end-user-notice">Part II (End-User Privacy Notice)</LegalLink>.
        </p>
      </NoticeBox>

      <PartHeader
        anchor="privacy-policy"
        numeral="I"
        title="Privacy Policy"
        binding="Published privacy notice"
        subtitle="How Hivy Technologies Limited (RC 9601398) collects, uses, stores, transfers, and protects personal data under Nigeria's NDPA 2023 & GAID 2025, GDPR, UK GDPR, Swiss FADP, and CCPA."
      />

      <Clause anchor="short-version" number="01" title="The short version">
        <p>
          {LEGAL_ENTITY.name} ({LEGAL_ENTITY.rcNumber}, Lagos, Nigeria) operates dimmi Cloud. We collect only the
          account, security, and billing details needed to run your workspace and process USD payments through Paystack.
          Your database, files, and backups live on Hetzner servers and object storage in Germany (EU), and
          outbound/inbound email runs through AWS in Frankfurt, Germany. When a workspace enables built-in AI features,
          prompt text goes to US AI providers (Anthropic, OpenAI, Voyage AI, and Cloudflare for the Clef decision
          model). We never sell personal data, we run zero analytics or advertising trackers on our website, docs, and
          dashboard, and no subprocessor trains AI models on Customer Data.
        </p>
      </Clause>

      <Clause anchor="who-we-are" number="02" title="Who we are">
        <LegalTable
          caption="Data controller identification"
          headers={["Field", "Details"]}
          rows={[
            ["Legal name", LEGAL_ENTITY.name],
            [
              "Registration number",
              `${LEGAL_ENTITY.rcNumber} (incorporated ${LEGAL_ENTITY.incorporationDate} under ${LEGAL_ENTITY.incorporationAct})`,
            ],
            ["Registered office", LEGAL_ENTITY.address],
            [
              "Contact (support, privacy, legal, and security)",
              <LegalLink key="support" href={`mailto:${LEGAL_CONTACTS.support}`}>
                {LEGAL_CONTACTS.support}
              </LegalLink>,
            ],
            [
              "Data Protection Officer & EU/UK Representatives",
              "Privacy inquiries are handled directly via support@trydimmi.com. When processing crosses statutory thresholds under the NDPA/GAID (for a formal DPO) or GDPR/UK GDPR Article 27 (for EU/UK representatives), named details will be published here and notified to the NDPC.",
            ],
          ]}
        />
      </Clause>

      <Clause anchor="whos-who" number="03" title="Who's who: our role for each type of data">
        <LegalTable
          caption="Controller and processor roles"
          headers={["Person", "Data involved", "Our legal role", "Governing section"]}
          rows={[
            [
              "Website & docs visitors",
              "Minimal server access logs and local theme preference (zero analytics or ad trackers).",
              "Data Controller",
              "Part I (Privacy Policy)",
            ],
            [
              "Workspace owners & team members",
              "Sign-up profile, credentials, 2FA/passkeys, sessions, security/audit logs, browser push subscriptions, Paystack billing details.",
              "Data Controller",
              "Part I (Privacy Policy)",
            ],
            [
              "Changelog subscribers",
              "Email address submitted via double opt-in.",
              "Data Controller",
              "Part I (Privacy Policy)",
            ],
            [
              "Business customers' end customers & workspace content",
              "Support conversations, contacts, attachments, emails, Meta/Slack messages, help center interactions, tickets, broadcasts, and AI logs.",
              "Data Processor (the business customer is Data Controller)",
              <LegalLink key="dpa" href="#dpa">
                Part VII (DPA)
              </LegalLink>,
            ],
          ]}
        />
      </Clause>

      <Clause anchor="end-customers" number="04" title="If you are a customer's end customer">
        <p>
          If you contacted a business through a dimmi web messenger, support email, WhatsApp, Messenger, Instagram,
          Slack channel, or help center, that business is the Data Controller of your information. Please contact that
          business directly to exercise your rights. If you write to{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>, we will forward
          your request to the business promptly. See{" "}
          <LegalLink href="#end-user-notice">Part II (End-User Notice)</LegalLink>.
        </p>
      </Clause>

      <Clause anchor="what-we-collect" number="05" title="What personal data we collect as controller">
        <LegalTable
          caption="Personal data collected as controller"
          headers={["Who", "Data collected", "Source"]}
          rows={[
            [
              "Team members",
              "Full name, email address, Argon2id password hash, avatar image, locale, and timezone.",
              "Provided directly at sign-up, invitation acceptance, or profile update.",
            ],
            [
              "Team members",
              "Two-factor authentication secrets, WebAuthn passkey public keys, and hashed recovery codes.",
              "Configured by you in account security settings.",
            ],
            [
              "Team members",
              "Google or GitHub account ID, name, email address, and profile avatar URL (no provider access tokens stored).",
              "Received from Google or GitHub when you sign in with them.",
            ],
            [
              "Team members",
              "Active session records: full IP address, browser user agent, device recognition identifier, and approximate country.",
              "Collected automatically when you sign in and use the dashboard.",
            ],
            [
              "Team members",
              "Security event log and workspace audit log: actions performed, timestamps, IP address, and user agent.",
              "Generated automatically for security accountability.",
            ],
            [
              "Team members",
              "Browser web-push subscription endpoint and encryption keys.",
              "Collected only if you opt in to browser notifications.",
            ],
            [
              "Workspace owners & billing contacts",
              "Billing name, business email, billing address, payment references/tokens, transaction records, and tax invoices (cards processed directly by Paystack).",
              "Provided when adding a payment method or signing an Order Form.",
            ],
            [
              "Website & docs visitors",
              "Standard HTTP server access logs (IP address, user agent, requested URL, timestamp) and Cloudflare Turnstile bot-check signals on sign-up/sign-in forms.",
              "Collected automatically by our servers and Turnstile to stop abuse.",
            ],
            [
              "Changelog subscribers",
              "Email address and double opt-in confirmation timestamp.",
              "Provided directly through the subscription form.",
            ],
          ]}
        />
      </Clause>

      <Clause anchor="legal-bases" number="06" title="Why we use personal data and our legal bases">
        <LegalTable
          caption="Purposes and legal bases under NDPA and GDPR"
          headers={["Purpose", "Data used", "Lawful basis (NDPA s.25 & GDPR Art. 6)"]}
          rows={[
            [
              "Creating, authenticating, and operating your account and workspace",
              "Profile details, credentials, SSO/Google/GitHub sign-in data, sessions",
              "Performance of a contract (NDPA s.25(1)(b)(i); GDPR Art. 6(1)(b))",
            ],
            [
              "Billing, invoicing, Paystack payments, and spend cap alerts",
              "Workspace owner and billing contact details, usage counts, invoices",
              "Contract performance and tax law compliance (NDPA s.25(1)(b)(i)-(ii); GDPR Art. 6(1)(b), (c))",
            ],
            [
              "Account security, fraud prevention, Turnstile bot checks, audit logs, and AUP enforcement",
              "Sessions, IP addresses, security/audit logs, Turnstile signals, server logs",
              "Legitimate interests (NDPA s.25(1)(b)(v); GDPR Art. 6(1)(f))",
            ],
            [
              "Support replies, legal notices, and statutory rights requests",
              "Contact emails, account metadata, correspondence",
              "Contract performance, legitimate interests, or legal obligation",
            ],
            [
              "Changelog emails and browser push notifications",
              "Double opt-in subscriber email; browser push endpoint",
              "Consent (NDPA s.25(1)(a), GAID Art. 18(1)(a); GDPR Art. 6(1)(a))",
            ],
            [
              "Tax, corporate, data protection, and lawful authority compliance",
              "Billing/tax records, audit logs, compelled disclosures",
              "Legal obligation (NDPA s.25(1)(b)(ii); GDPR Art. 6(1)(c))",
            ],
          ]}
        />
      </Clause>

      <Clause anchor="ai-features" number="07" title="AI features">
        <p>
          AI features (dimmi agents, procedures, Operator, and routines) process Customer Data under{" "}
          <LegalLink href="#dpa">Part VII (DPA)</LegalLink> and{" "}
          <LegalLink href="/terms#pt-ai">Product-Specific Terms Section II.1</LegalLink>. Built-in answer models
          (Anthropic, OpenAI), embeddings (Voyage AI), and decision checks (Cloudflare Clef) never train on Customer
          Data, "Hide personal data" masking is on by default, and workspace admins can disable AI or use BYO keys at
          any time. See <LegalLink href="#ai-transparency">Part V (AI Transparency)</LegalLink>.
        </p>
      </Clause>

      <Clause anchor="sharing" number="08" title="Who we share personal data with">
        <p>
          We never sell personal data and never share it for cross-context behavioral advertising. We disclose data only
          to: (a) listed subprocessors in <LegalLink href="#subprocessors">Part IV</LegalLink> (Hetzner, AWS Frankfurt,
          Cloudflare, Paystack, and AI providers when enabled; AWS SES scans email in transit for spam and malware); (b)
          independent services where triggered (Google/GitHub sign-in, Let's Encrypt for custom domains, browser push
          relays); (c) competent legal authorities under valid process; or (d) an affiliate or successor entity upon
          corporate reorganization or asset transfer with notice.
        </p>
      </Clause>

      <Clause anchor="transfers" number="09" title="Where data is stored and international transfers">
        <p>
          All primary databases, files, and backups sit on Hetzner servers and object storage in Germany (EU). Raw
          inbound email is held at AWS in Frankfurt for up to 31 days. No US AI vendor holds a copy of our database.
          Here is the exact geography of every data flow:
        </p>
        <LegalTable
          caption="Data storage locations and cross-border transfers"
          headers={["Data flow", "Destination country", "Vendor / party", "Transfer safeguard"]}
          rows={[
            [
              "Primary database, files, cache, logs & malware scanning",
              "Germany (Falkenstein, EU)",
              "Hetzner Online GmbH",
              "EU GDPR processor under Hetzner DPA; EU/EEA hosting commitment; NDPA s.41 contractual basis.",
            ],
            [
              "PostgreSQL backups & WAL archives (30 days)",
              "Germany (Nuremberg, nbg1, EU)",
              "Hetzner Online GmbH",
              "EU GDPR processor under Hetzner DPA; NDPA s.41 contractual basis.",
            ],
            [
              "Outbound & inbound email (SES, S3, SNS)",
              "Germany (Frankfurt, AWS eu-central-1, EU)",
              "Amazon Web Services EMEA SARL",
              "AWS GDPR DPA with EU SCCs and EU-US Data Privacy Framework; NDPA s.41 contractual basis.",
            ],
            [
              "Built-in AI inference, embeddings & Clef checks (only when AI is on)",
              "United States (& Cloudflare global edge)",
              "Anthropic, OpenAI, Voyage AI, Cloudflare",
              "Vendor DPAs with EU SCCs (2021/914), UK Addendum, and NDPA s.41 contractual safeguards (Cloudflare also DPF-certified).",
            ],
            [
              "Payment processing & billing",
              "Nigeria",
              "Paystack Payments Limited",
              "Payment processor established in Nigeria; billing data is not transferred abroad by dimmi.",
            ],
            [
              "Engineering, operations & support access",
              "Nigeria",
              `${LEGAL_ENTITY.name} personnel`,
              "EU SCCs (2021/914), UK Addendum & Swiss FADP in Annex 4; strict need-to-know access.",
            ],
          ]}
        />
      </Clause>

      <Clause anchor="retention" number="10" title="How long we keep controller data">
        <p>
          Under Article 49(3) of the GAID 2025, where no law prescribes a longer retention period, personal data is not
          kept beyond 6 months after its purpose is accomplished, subject to active legal claims. Processor retention
          periods for workspace data appear in <LegalLink href="#annex-1">DPA Annex 1</LegalLink>.
        </p>
        <LegalTable
          caption="Controller data retention schedule"
          headers={["Data category", "Retention period", "Details"]}
          rows={controllerRetention.map((row) => [row.category, row.period, row.notes])}
        />
      </Clause>

      <Clause anchor="security-summary" number="11" title="Security and cookies">
        <p>
          We protect data using TLS in transit, Argon2id password hashing, passkeys and 2FA, SAML/OIDC SSO,
          per-workspace credential encryption, PostgreSQL row-level security, ClamAV scanning, and 365-day audit logs
          (full details and honest boundaries in <LegalLink href="#security-overview">Part VI</LegalLink>). We use only
          strictly necessary first-party cookies and local storage with zero analytics or ads (full inventory in{" "}
          <LegalLink href="#cookie-notice">Part III</LegalLink>).
        </p>
      </Clause>

      <Clause anchor="rights" number="12" title="Your privacy rights by region">
        <p>
          We respond to verified privacy requests within{" "}
          <strong className="font-medium text-text-primary">30 days</strong>. Email{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> from your account
          email to exercise any right below.
        </p>

        <SubHeading>Nigeria (NDPA 2023 &amp; GAID 2025)</SubHeading>
        <p>
          Under NDPA Sections 34 to 38, you have the right to access, correct, erase, restrict, or port your personal
          data, object to processing (absolutely for direct marketing), withdraw consent as easily as giving it, and
          avoid solely automated decisions with legal or similarly significant effects. You may send us a Standard
          Notice to Address Grievance (SNAG) at{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>, complain to the{" "}
          <strong className="font-medium text-text-primary">Nigeria Data Protection Commission (NDPC)</strong>{" "}
          (ndpc.gov.ng), or seek judicial redress before the nearest Federal or State High Court (GAID Art. 47).
        </p>

        <SubHeading>EEA, United Kingdom, and Switzerland (GDPR, UK GDPR &amp; FADP)</SubHeading>
        <p>
          You hold full rights of access, rectification, erasure, restriction, portability, objection, and consent
          withdrawal, plus the right to complain to your EU supervisory authority, the UK ICO (ico.org.uk), or the Swiss
          FDPIC.
        </p>

        <SubHeading>California and other US states (CCPA / CPRA &amp; state laws)</SubHeading>
        <p>
          You have the right to know, delete, and correct personal information, and not to face discrimination for
          exercising privacy rights. We do not sell or share personal information as defined by the CCPA. You may also
          contact your state Attorney General or the California Privacy Protection Agency (CPPA).
        </p>

        <SubHeading>Everywhere else</SubHeading>
        <p>
          We honor equivalent local data protection rights under applicable laws (including Brazil's LGPD, South
          Africa's POPIA, Kenya's DPA, Ghana's DPA, and Canada's PIPEDA).
        </p>
      </Clause>

      <Clause anchor="automated-children" number="13" title="Automated decisions and children">
        <p>
          {LEGAL_ENTITY.name} makes no solely automated decisions with legal or similarly significant effects on website
          visitors or account holders. dimmi Cloud is a B2B platform strictly for individuals aged 18 and older (never
          directed at children under 13 under US COPPA or minors under 18 under NDPA Section 31). If you believe a child
          submitted personal data, email{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> for prompt deletion.
        </p>
      </Clause>

      <Clause anchor="google-github" number="14" title="Sign-in with Google and GitHub, and self-hosted dimmi">
        <p>
          <strong className="font-medium text-text-primary">Google &amp; GitHub sign-in:</strong> We receive only your
          account ID, full name, email address, and profile avatar solely to sign you in. We retain no provider OAuth
          tokens, never use Google user data for advertising, and comply with the Google API Services User Data Policy
          (including Limited Use).
        </p>
        <p>
          <strong className="font-medium text-text-primary">Self-hosted dimmi:</strong> Self-hosted installations send
          zero telemetry or data to {LEGAL_ENTITY.name}. We have no access to them; the self-host operator is solely
          responsible for its privacy practices (see{" "}
          <LegalLink href="/terms#self-hosting-notice">Self-Hosting Notice</LegalLink>).
        </p>
      </Clause>

      <Clause anchor="changes-contact" number="15" title="Changes, contact, complaints, and version history">
        <p>
          We provide at least 30 days' advance email and in-app notice for material updates to this Privacy Policy. For
          questions, privacy rights requests, or complaints, email{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> or write to{" "}
          {LEGAL_ENTITY.name}, {LEGAL_ENTITY.address}. We aim to resolve every complaint within 30 days, and you always
          retain the right to complain to the NDPC or your local data protection regulator.
        </p>
        <LegalTable
          caption="Privacy Policy & DPA version archive"
          headers={["Version", "Effective", "Summary"]}
          rows={[
            [
              `v${LEGAL_VERSION.version}`,
              LEGAL_VERSION.effectiveDate,
              "Initial release of dimmi Cloud Privacy Policy, End-User Notice, Cookie Notice, Subprocessor List, AI Transparency, Security Overview, and Global DPA.",
            ],
          ]}
        />
      </Clause>

      <PrivacySupportingParts />
      <PrivacyDpaPart />
      <LegalEntityFooter />
    </LegalDocument>
  );
}
