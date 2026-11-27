import {
  COOKIES_AND_STORAGE,
  CUSTOMER_CONNECTED_PLATFORMS,
  LEGAL_CONTACTS,
  LEGAL_ENTITY,
  LEGAL_VERSION,
  OTHER_THIRD_PARTY_SERVICES,
  RETENTION_SCHEDULE,
  SECURITY_MEASURES,
  SUBPROCESSORS,
} from "./constants";
import { Clause, LegalLink, LegalList, LegalTable, PartHeader, SubHeading } from "./layout";

export function PrivacySupportingParts() {
  const infra = SUBPROCESSORS.filter((s) => s.category === "Infrastructure & email");
  const ai = SUBPROCESSORS.filter((s) => s.category === "AI & search");
  const billing = SUBPROCESSORS.filter((s) => s.category === "Billing");

  const renderVendorRows = (list: typeof SUBPROCESSORS) =>
    list.map((s) => [
      <div key={s.vendor} className="flex flex-col gap-0.5">
        <span className="font-medium text-text-primary">{s.vendor}</span>
        <span className="text-[12px] text-text-tertiary">{s.entity}</span>
      </div>,
      s.purpose,
      s.dataReceived,
      s.location,
      <div key={`${s.vendor}-when`} className="flex flex-col gap-1">
        <span>{s.appliesWhen}</span>
        <span className="text-[12px] text-text-tertiary">{s.retentionAndTraining}</span>
      </div>,
    ]);

  return (
    <>
      <PartHeader
        anchor="end-user-notice"
        numeral="II"
        title="End-User Privacy Notice"
        binding="In-software notice (GAID Art. 31)"
        subtitle="A plain-English notice for individuals chatting through a business's dimmi web messenger or browsing a dimmi-hosted help center."
      />

      <Clause anchor="eu-controller" number="II.1" title="Who controls your conversation data">
        <p>
          When you chat with a business through a dimmi messenger widget or visit its dimmi-hosted help center,{" "}
          <strong className="font-medium text-text-primary">that business</strong> is the Data Controller and decides
          why your data is collected and how long it is kept. {LEGAL_ENTITY.name} ({LEGAL_ENTITY.rcNumber}, Lagos,
          Nigeria) acts strictly as its Data Processor. We never sell your messages, never use them for ads, and never
          train AI models on them.
        </p>
      </Clause>

      <Clause
        anchor="eu-collection"
        number="II.2"
        title="What the messenger collects, browser storage, AI, and your rights"
      >
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Conversation &amp; session details:</strong> Messages,
              attachments, satisfaction ratings, any details you type or that the business passes from your signed-in
              account, plus the current page URL and title, referring page, up to 10 recent pages viewed on that site,
              browser user agent, bot score, and approximate country from a shortened (network-prefix) IP address.
            </>,
            <>
              <strong className="font-medium text-text-primary">Browser local storage (zero cookies):</strong> The web
              messenger sets no HTTP cookies. It stores a session token, unsent message outbox, and last-seen marker in
              your browser's local storage (up to 180 days, or until cleared) so your chat stays open across pages. A
              restricted help center sets one session cookie (`dimmi_help_reader`, 7 days) only when you sign in.
            </>,
            <>
              <strong className="font-medium text-text-primary">AI replies:</strong> Replies may come from an AI support
              agent, which is labelled as AI in the messenger, says it is an AI whenever you ask, and hands over to a
              person when you ask for one. Conversation text is hosted on our servers in Germany and, when the business
              enables built-in AI, processed by our listed AI subprocessors with default PII masking and no training on
              Customer Data.
            </>,
            <>
              <strong className="font-medium text-text-primary">Exercising your rights:</strong> Contact the business
              you wrote to directly. If you cannot reach them, email{" "}
              <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> naming the
              business and we will forward your request promptly.
            </>,
          ]}
        />
      </Clause>

      <PartHeader
        anchor="cookie-notice"
        numeral="III"
        title="Cookie & Browser Storage Notice"
        binding="Published notice (GAID Arts. 7(l), 19 & ePrivacy)"
        subtitle="Complete inventory of every first-party cookie and browser storage entry used by trydimmi.com, dimmi Cloud, customer help centers, and the web messenger."
      />

      <Clause anchor="cookies-inventory" number="III.1" title="Zero advertising trackers">
        <p>
          dimmi sets{" "}
          <strong className="font-medium text-text-primary">
            zero analytics cookies, zero advertising pixels, and zero cross-site tracking cookies
          </strong>{" "}
          across trydimmi.com, our docs, the dimmi Cloud dashboard, and the web messenger. Every entry below is
          first-party and strictly necessary, a preference, or the dashboard's own diagnostics. If we ever introduce
          non-essential cookies in the future, an explicit Accept / Reject consent banner visible without scrolling will
          be deployed first.
        </p>
        <LegalTable
          caption="Complete cookie and browser storage inventory"
          headers={["Surface", "Name", "Type", "Purpose", "Lifetime", "Category"]}
          rows={COOKIES_AND_STORAGE.map((c) => [
            c.surface,
            <code key={c.name} className="font-mono text-[12px] text-text-primary">
              {c.name}
            </code>,
            c.mechanism,
            c.purpose,
            c.lifetime,
            c.category,
          ])}
        />
        <p>
          One feature triggers a third-party browser request without setting dimmi advertising cookies: Cloudflare
          Turnstile on sign-up and sign-in pages (bot protection). Customer-sent broadcast and series emails always
          include a first-party open pixel, and rewrite links to count clicks by default (no IP address or user agent is
          stored; click tracking can be disabled per broadcast). Transactional and support replies carry no tracking.
        </p>
        <p>
          The dashboard records masked sessions of signed-in teammates on a recorder we host ourselves, in the EU, at
          sessions.trydimmi.com. Your browser replaces every character of text and every input with asterisks before
          anything is sent, and leaves out images and media. Each recording carries the teammate's dimmi user ID and
          their workspace's ID and name, never their name or email address.
        </p>
        <p>
          We use these recordings only to operate, support, analyze, and improve dimmi. We never use them for
          advertising, and we never sell or share them. Help centers, the web messenger, and your customers' sites never
          load the recorder, so your customers are never recorded or profiled.
        </p>
        <p>
          We delete every recording automatically after 30 days. When a workspace is deleted, its recordings are deleted
          with it.
        </p>
      </Clause>

      <PartHeader
        anchor="subprocessors"
        numeral="IV"
        title="Subprocessors & Third-Party Platforms"
        binding="Referenced by DPA Annex 3"
        subtitle="Every vendor that processes data for dimmi Cloud in production, plus independent third-party services and platforms connected by customers."
      />

      <Clause anchor="sub-infra" number="IV.1" title="Infrastructure, email, and edge subprocessors">
        <LegalTable
          caption="Infrastructure and email subprocessors"
          headers={["Legal entity", "Purpose", "Data received", "Processing location", "Applies when & retention"]}
          rows={renderVendorRows(infra)}
        />
      </Clause>

      <Clause anchor="sub-ai" number="IV.2" title="AI and search subprocessors (only when AI features are enabled)">
        <p>
          These vendors process Customer Data <strong className="font-medium text-text-primary">only</strong> when a
          workspace turns on built-in AI or semantic search features. If AI is off, none of them receive conversation
          data. If you bring your own API key, that provider is your direct vendor.
        </p>
        <LegalTable
          caption="AI and search subprocessors"
          headers={["Legal entity", "Purpose", "Data received", "Processing location", "Applies when & retention"]}
          rows={renderVendorRows(ai)}
        />
      </Clause>

      <Clause anchor="sub-billing" number="IV.3" title="Payment processor (Account Data only)">
        <p>
          Paystack processes Account and Billing Data for paid workspaces only and never receives End Customer
          conversations.
        </p>
        <LegalTable
          caption="Payment and billing processor"
          headers={["Legal entity", "Purpose", "Data received", "Processing location", "Applies when & retention"]}
          rows={renderVendorRows(billing)}
        />
      </Clause>

      <Clause
        anchor="sub-other"
        number="IV.4"
        title="Independent services & customer-connected platforms (not subprocessors)"
      >
        <SubHeading>Independent third-party services</SubHeading>
        <LegalTable
          caption="Independent third-party services"
          headers={["Service", "Legal entity", "Role", "Data received", "Applies when"]}
          rows={OTHER_THIRD_PARTY_SERVICES.map((t) => [t.name, t.entity, t.role, t.whatItReceives, t.appliesWhen])}
        />
        <p className="text-[13px] text-text-tertiary">
          Domain DNS records are managed on Vercel DNS (no personal data), and source code and container builds are
          hosted on GitHub (no customer data).
        </p>

        <SubHeading>Third-party platforms connected by Customer (not subprocessors)</SubHeading>
        <p>
          When Customer connects an external channel, identity provider, issue tracker, or BYO AI key, Customer holds
          its own agreement with that provider and instructs dimmi to exchange data with it:
        </p>
        <LegalTable
          caption="Customer-connected third-party platforms"
          headers={["Platform", "Purpose", "Contractual relationship"]}
          rows={CUSTOMER_CONNECTED_PLATFORMS.map((p) => [p.platform, p.purpose, p.relationship])}
        />

        <SubHeading>30-day subprocessor change notice &amp; log</SubHeading>
        <p>
          We notify Workspace owners by email at least 30 days before adding a new subprocessor, and record the change
          in the log on this page (with a 30-day objection right under{" "}
          <LegalLink href="#dpa-subprocessors">DPA section VII.3</LegalLink>). Non-owners may subscribe to change
          notices or submit objections via{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>.
        </p>
        <LegalTable
          caption="Subprocessor change log"
          headers={["Version", "Date", "Summary"]}
          rows={[
            [
              `v${LEGAL_VERSION.version}`,
              LEGAL_VERSION.effectiveDate,
              "Initial published subprocessor list: Hetzner (Falkenstein & Nuremberg), AWS (Frankfurt eu-central-1), Cloudflare (Clef, Crawler, Turnstile), Anthropic, OpenAI, Voyage AI, and Paystack.",
            ],
          ]}
        />
      </Clause>

      <PartHeader
        anchor="ai-transparency"
        numeral="V"
        title="AI Transparency"
        binding="Informational overview"
        subtitle="Which models run on dimmi Cloud, what data each call carries, how default PII masking works, and vendor training and retention rules."
      />

      <Clause anchor="ai-flows" number="V.1" title="Models, per-call inputs, masking, and retention">
        <LegalTable
          caption="AI pipelines and data flows"
          headers={["Pipeline", "Provider", "Data sent per call", "Training & retention"]}
          rows={[
            [
              "Customer-facing dimmi agent & Operator",
              "Anthropic & OpenAI (United States)",
              "Conversation messages (no internal notes for customer agents), matched knowledge passages, action results, active procedure instructions, and by default only first name and locale.",
              "No model training. Requests are retained for up to 30 days in standard API logs, then deleted; Anthropic may retain abuse-flagged content up to 2 years.",
            ],
            [
              "Embeddings & semantic search",
              "Voyage AI (United States)",
              "Help articles, uploaded knowledge files, crawled public pages, closed-conversation text, and search queries.",
              "Opted out of all training and retention; inputs deleted immediately after processing.",
            ],
            [
              "Decision & reply quality checks",
              "Cloudflare Workers AI (Clef)",
              "Draft replies, conversation excerpts, and yes-or-no check questions.",
              "Zero data retention and no training on Customer Data, per Cloudflare's Workers AI data terms.",
            ],
          ]}
        />
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Default PII masking (and honest limits):</strong> "Hide
              personal data" is on by default, masking emails, phone numbers, card numbers, bank account numbers, and
              common secrets before prompts reach answer models. Pattern masking is a safeguard, not a 100% guarantee
              for unstructured free text.
            </>,
            <>
              <strong className="font-medium text-text-primary">dimmi debugging logs:</strong> Full AI requests and
              responses are kept inside our German PostgreSQL database for 30 days for workspace troubleshooting, then
              purged automatically.
            </>,
            <>
              <strong className="font-medium text-text-primary">Controls &amp; disclosure:</strong> Admins can disable
              AI features, connect BYO API keys, require human approval before risky procedure actions, and rely on
              default AI identity disclosure (EU AI Act Article 50) plus human handover on every channel.
            </>,
          ]}
        />
      </Clause>

      <PartHeader
        anchor="security-overview"
        numeral="VI"
        title="Security Overview"
        binding="Mirrors DPA Annex 2"
        subtitle="Implemented security controls, hosting locations, honest current boundaries, and vulnerability disclosure."
      />

      <Clause
        anchor="sec-controls"
        number="VI.1"
        title="Implemented controls, boundaries, and vulnerability disclosure"
      >
        <div className="flex flex-col gap-4">
          {SECURITY_MEASURES.map((group) => (
            <div key={group.area} className="rounded-xl border border-separator-border p-5">
              <h4 className="mb-2 text-[15px] font-medium text-text-primary">{group.area}</h4>
              <LegalList items={group.controls} />
            </div>
          ))}
        </div>

        <SubHeading>Honest boundaries and certification status</SubHeading>
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">No SOC 2 or ISO 27001 for dimmi yet:</strong>{" "}
              {LEGAL_ENTITY.name} does not currently hold a SOC 2 or ISO/IEC 27001 certificate. Data-center ISO/IEC
              27001 certification belongs strictly to Hetzner Online GmbH (and AWS for email infrastructure).
            </>,
            <>
              <strong className="font-medium text-text-primary">Encryption scope:</strong> Sensitive workspace
              credentials and Kubernetes secrets are encrypted at rest. Conversation text, uploaded files, and database
              backups are not encrypted at the application level.
            </>,
            <>
              <strong className="font-medium text-text-primary">Malware scanner fail-open behavior:</strong> ClamAV
              scans every attachment on our EU servers; if scanning fails repeatedly, delivery fails open to preserve
              support availability.
            </>,
            <>
              <strong className="font-medium text-text-primary">Vulnerability disclosure:</strong> Report security
              issues privately to{" "}
              <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> (subject:
              "Security Report"). We acknowledge good-faith reports within 2 business days and will not pursue legal
              action against researchers following responsible disclosure.
            </>,
          ]}
        />
      </Clause>
    </>
  );
}

export function PrivacyDpaPart() {
  const processorRetention = RETENTION_SCHEDULE.filter((row) => row.scope === "Processor");
  const customerDataSubprocessors = SUBPROCESSORS.filter((s) => s.category !== "Billing");

  return (
    <>
      <PartHeader
        anchor="dpa"
        numeral="VII"
        title="Data Processing Agreement (DPA)"
        binding="Automatically incorporated in Terms of Service"
        subtitle="Global DPA covering NDPA 2023, GAID 2025 (Art. 34(2)), GDPR (Art. 28 & EU SCCs 2021/914), UK GDPR, Swiss FADP, and US state privacy laws (CCPA/CPRA). Countersigned PDF copies available via support@trydimmi.com."
      />

      <Clause anchor="dpa-parties" number="VII.1" title="Parties, recitals, definitions, and roles">
        <p>
          This Data Processing Agreement (<strong className="font-medium text-text-primary">"DPA"</strong>) takes effect
          automatically between Customer (as recorded in Customer's Workspace or Order Form) and{" "}
          <strong className="font-medium text-text-primary">{LEGAL_ENTITY.name}</strong> ({LEGAL_ENTITY.rcNumber},
          registered office: {LEGAL_ENTITY.address}) as part of the{" "}
          <LegalLink href="/terms">dimmi Cloud Terms of Service</LegalLink> (the{" "}
          <strong className="font-medium text-text-primary">"Principal Agreement"</strong>). Under Article 34(2) of the
          GAID 2025, upon crossing statutory thresholds under NDPA Section 44, {LEGAL_ENTITY.name} will register with
          the NDPC and make registration evidence available on request at{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>.
        </p>
        <p>
          <strong className="font-medium text-text-primary">Roles and definitions:</strong> For{" "}
          <strong className="font-medium text-text-primary">Customer Personal Data</strong> (Personal Data within
          Customer Data, excluding Account Data), Customer is Controller (or Processor for its clients) and dimmi is
          Processor (or subprocessor). For Account Data, {LEGAL_ENTITY.name} is an independent Controller under Part I
          above. <strong className="font-medium text-text-primary">"Applicable Data Protection Law"</strong> includes
          the NDPA 2023, GAID 2025, GDPR, UK GDPR, Swiss FADP, CCPA/CPRA, and laws named in Annex 5.{" "}
          <strong className="font-medium text-text-primary">"Security Incident"</strong> means a confirmed security
          breach leading to accidental or unlawful destruction, loss, alteration, unauthorized disclosure of, or access
          to Customer Personal Data (excluding unsuccessful probes or pings).
        </p>
      </Clause>

      <Clause anchor="dpa-instructions" number="VII.2" title="Documented instructions, lawful basis, and AI processing">
        <p>
          <strong className="font-medium text-text-primary">Instructions &amp; CCPA prohibitions:</strong> dimmi
          Processes Customer Personal Data solely on Customer's documented instructions (the Principal Agreement, this
          DPA, and Customer's Workspace configuration). dimmi will never "sell" or "share" Customer Personal Data, never
          retain, use, or disclose it outside the direct business relationship or specified service purposes, and will
          promptly notify Customer if an instruction appears to infringe Applicable Data Protection Law.
        </p>
        <p>
          <strong className="font-medium text-text-primary">Customer duties:</strong> Customer warrants a documented
          lawful basis (as stated in Annex 1 per GAID Art. 34(2)) and all required notices and consents (including for
          chat recording, browser storage, email tracking, Meta/Slack messaging, and AI processing).
        </p>
        <p>
          <strong className="font-medium text-text-primary">AI processing:</strong> Built-in AI providers are bound as
          Subprocessors in Annex 3. Neither dimmi nor any AI Subprocessor trains models (modifies model weights) on
          Customer Personal Data. Each AI Subprocessor keeps Customer Personal Data only for the retention period stated
          for it in the Subprocessor List (Part IV), including any safety exception named there, and deletes it
          afterwards. Default PII masking is enabled, and Customer can disable AI or use BYO keys at any time.
        </p>
      </Clause>

      <Clause
        anchor="dpa-subprocessors"
        number="VII.3"
        title="Personnel, security, subprocessors, and third-party platforms"
      >
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Personnel &amp; security:</strong> Staff with access to
              Customer Personal Data are bound by written confidentiality duties, trained in data protection, and
              limited to need-to-know access. dimmi maintains the controls in{" "}
              <LegalLink href="#annex-2">Annex 2</LegalLink> without materially lowering overall protection.
            </>,
            <>
              <strong className="font-medium text-text-primary">Subprocessors (30-day notice &amp; objection):</strong>{" "}
              Customer grants general written authorization for the Subprocessors in{" "}
              <LegalLink href="#annex-3">Annex 3</LegalLink>. dimmi gives at least 30 days' advance email notice, and
              records the change on the Subprocessor List, before authorizing any new Subprocessor (except emergencies,
              with prompt notice). Customer may object on reasonable data-protection grounds within 30 days to{" "}
              <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>; if unresolved
              within 30 days, Customer may terminate the affected service for a pro-rata refund of prepaid unused fees.
              dimmi binds every Subprocessor in writing and remains liable for its performance.
            </>,
            <>
              <strong className="font-medium text-text-primary">Third-party platforms:</strong> Meta, Slack, BYO AI
              providers, webhooks, actions, MCP clients, SSO providers, and Customer-connected integrations are not
              dimmi Subprocessors and are governed by Customer's own contracts.
            </>,
          ]}
        />
      </Clause>

      <Clause
        anchor="dpa-incidents"
        number="VII.4"
        title="Data subject requests, 72-hour incident notice, DPIAs, and audits"
      >
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Data Subject requests:</strong> dimmi provides
              self-service export, correction, redaction, and erasure tools, forwards direct End Customer requests to
              Customer promptly, and gives reasonable further assistance.
            </>,
            <>
              <strong className="font-medium text-text-primary">Security Incidents (72 hours):</strong> dimmi notifies
              Customer without undue delay and within{" "}
              <strong className="font-medium text-text-primary">72 hours</strong> of becoming aware of a confirmed
              Security Incident affecting Customer Personal Data, providing the elements required by GAID Article 33 and
              GDPR Article 33(3) (nature, affected scope, contact {LEGAL_CONTACTS.support}, likely consequences, and
              mitigation). Notice is not an admission of fault.
            </>,
            <>
              <strong className="font-medium text-text-primary">Impact assessments &amp; regulators:</strong> dimmi
              provides reasonable help with DPIAs, transfer assessments, and consultations with the NDPC, EU
              authorities, and UK ICO.
            </>,
            <>
              <strong className="font-medium text-text-primary">Audits:</strong> Security documentation and written
              questionnaire answers first; if insufficient for statutory compliance, Customer (or a non-competitor
              auditor under NDA) may conduct one remote or on-site audit per 12 months on 30 days' notice at Customer's
              cost (unless material non-compliance is found). Regulator-ordered audits are never restricted.
            </>,
          ]}
        />
      </Clause>

      <Clause
        anchor="dpa-deletion"
        number="VII.5"
        title="Return, deletion, international transfers, and government access"
      >
        <p>
          <strong className="font-medium text-text-primary">Return &amp; deletion:</strong> Export is available
          throughout the Term and for 30 days after termination or Workspace deletion. Live data (including database
          rows, raw email files, help center images, and knowledge uploads) is purged within 30 days thereafter, and
          backups roll off within 30 days (subject to statutory retention carve-outs such as Cybercrimes Act Section 38
          where applicable).
        </p>
        <p>
          <strong className="font-medium text-text-primary">Transfers &amp; government access requests:</strong> Primary
          hosting sits in Germany (Falkenstein and Nuremberg), email transits Germany (Frankfurt), optional AI inference
          runs in the United States/Cloudflare edge, and dimmi personnel access systems from Nigeria. Cross-border
          transfers are governed automatically by <LegalLink href="#annex-4">Annex 4</LegalLink>. If a government or law
          enforcement authority compels disclosure (including under the Nigerian Cybercrimes Act 2015 as amended 2024),
          dimmi redirects the authority to Customer where possible, notifies Customer unless prohibited by law,
          challenges overbroad demands, and discloses only the required minimum.
        </p>
      </Clause>

      <Clause anchor="dpa-gaid" number="VII.6" title="Processing risks, liability, insurance, disputes, and precedence">
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Potential processing risks (GAID Art. 34(2)):</strong>{" "}
              Principal risks include workspace credential compromise, incidental sensitive data in free text,
              cross-border transmission to US AI vendors when AI is on, external email transit risks, and inaccurate AI
              outputs, mitigated by the safeguards in Annex 2, default PII masking, RLS, 2FA/SSO, and IP allowlists.
            </>,
            <>
              <strong className="font-medium text-text-primary">Liability &amp; indemnity:</strong> Governed by Clauses
              17 and 18 of the Principal Agreement (including the 2x data-protection super-cap). Nothing limits direct
              Data Subject rights under the EU SCCs, UK Addendum, or NDPA.
            </>,
            <>
              <strong className="font-medium text-text-primary">Insurance (GAID Art. 34(2)):</strong> As an early-stage
              operating company, {LEGAL_ENTITY.name} does not carry standalone cyber insurance at v
              {LEGAL_VERSION.version} publication; certificates of coverage will be provided on request once bound for
              Order Form customers.
            </>,
            <>
              <strong className="font-medium text-text-primary">Term, force majeure, disputes &amp; precedence:</strong>{" "}
              Follows the Principal Agreement except where Annex 4 transfer clauses or GAID Article 47 fix a different
              law or court forum. On data protection matters, Annex 4 transfer clauses prevail first, then this DPA,
              then the Principal Agreement.
            </>,
          ]}
        />
      </Clause>

      <Clause anchor="annex-1" number="ANNEX 1" title="Details of processing and processor retention">
        <LegalTable
          caption="DPA Annex 1 processing details"
          headers={["Element", "Description"]}
          rows={[
            [
              "Subject matter, duration & nature",
              "Hosting, indexing, vector embedding (when AI/search is on), multi-channel message routing, malware scanning, PII masking, AI inference, export, and deletion for dimmi Cloud during the Term plus the 30-day export/deletion cycle.",
            ],
            [
              "Data subjects",
              "Customer's End Customers (support requesters, website visitors, help center readers, feedback contributors, broadcast recipients) and Customer's Team Members.",
            ],
            [
              "Personal data categories",
              "Contact profiles, messenger session metadata (page URLs, user agent, truncated IP/country), messages, internal notes, attachments, email headers and raw .eml files, Meta/Slack identifiers and payloads, AI inputs/outputs/memories, help center ratings/searches, broadcast delivery timestamps, and encrypted integration tokens.",
            ],
            [
              "Sensitive data & Customer lawful bases",
              "No special-category data requested; HIPAA PHI, children's data, and structured sensitive attributes prohibited without written agreement. Customer's lawful bases (GAID Art. 34(2)): contract performance, legitimate interests, legal obligation, or consent.",
            ],
            [
              "Frequency & locations",
              "Continuous during active Workspace use. Hosted in Germany (EU); email in Frankfurt (EU); optional AI in United States; dimmi team operations from Nigeria.",
            ],
          ]}
        />
        <SubHeading>Processor retention schedule</SubHeading>
        <LegalTable
          caption="Processor data retention schedule"
          headers={["Data category", "Retention period", "Details"]}
          rows={processorRetention.map((row) => [row.category, row.period, row.notes])}
        />
      </Clause>

      <Clause anchor="annex-2" number="ANNEX 2" title="Technical and organizational security measures">
        <p>
          Annex 2 incorporates in full the implemented technical and organizational security controls and factual
          boundaries set out in <LegalLink href="#security-overview">Part VI (Security Overview)</LegalLink> above.
        </p>
      </Clause>

      <Clause anchor="annex-3" number="ANNEX 3" title="Authorized subprocessors">
        <p>
          Authorized Subprocessors for Customer Personal Data as of {LEGAL_VERSION.effectiveDate} (full details in{" "}
          <LegalLink href="#subprocessors">Part IV</LegalLink>; Paystack handles Account/Billing Data only):
        </p>
        <LegalTable
          caption="Annex 3 authorized subprocessors"
          headers={["Legal entity", "Purpose", "Location", "Applies when"]}
          rows={customerDataSubprocessors.map((s) => [s.entity, s.purpose, s.location, s.appliesWhen])}
        />
      </Clause>

      <Clause anchor="annex-4" number="ANNEX 4" title="International transfer mechanisms">
        <SubHeading>1. EEA (EU Standard Contractual Clauses, Decision 2021/914)</SubHeading>
        <p>
          The EU SCCs (Decision (EU) 2021/914) apply automatically to restricted transfers from the EEA to{" "}
          {LEGAL_ENTITY.name} in Nigeria or third countries:{" "}
          <strong className="font-medium text-text-primary">Module 2</strong> (Controller-to-Processor) or{" "}
          <strong className="font-medium text-text-primary">Module 3</strong> (Processor-to-Processor);{" "}
          <strong className="font-medium text-text-primary">Clause 7</strong> (docking) included;{" "}
          <strong className="font-medium text-text-primary">Clause 9(a)</strong> Option 2 (General Written
          Authorization, 30 days' notice); <strong className="font-medium text-text-primary">Clause 11</strong> optional
          redress omitted; <strong className="font-medium text-text-primary">Clause 13</strong> competent authority per
          Clause 13(a) (or BayLDA, Bavaria, Germany where Customer has no EU establishment);{" "}
          <strong className="font-medium text-text-primary">Clause 17</strong> Option 1 governed by German law;{" "}
          <strong className="font-medium text-text-primary">Clause 18(b)</strong> courts of Germany; Annexes I to III
          completed by Annexes 1 to 3 of this DPA. If the EU-US Data Privacy Framework is invalidated, onward transfers
          automatically rely on Subprocessor EU SCCs.
        </p>

        <SubHeading>2. United Kingdom (UK IDTA Addendum) &amp; Switzerland (FADP)</SubHeading>
        <p>
          For UK GDPR transfers, the UK International Data Transfer Addendum (vB1.0) is incorporated by reference with
          Tables 1 to 3 populated by Annexes 1 to 3 and mutual termination rights in Table 4. For Swiss FADP transfers,
          the EU SCCs apply with references to GDPR including the FADP, references to Member States including
          Switzerland, the FDPIC as supervisory authority, and Swiss court access for Swiss-resident data subjects.
        </p>

        <SubHeading>3. Nigeria cross-border transfers (NDPA ss. 41 to 43)</SubHeading>
        <p>
          For transfers from Nigeria to Germany (Hetzner, AWS Frankfurt) and the United States (Anthropic, OpenAI,
          Voyage AI, Cloudflare), {LEGAL_ENTITY.name} relies on NDPA Section 41(1)(a): each recipient is bound by GDPR
          or by enforceable contractual data protection clauses. We record the basis and our adequacy assessment for
          each destination under NDPA Section 41(2). The NDPC has not yet issued adequacy decisions or approved standard
          contractual clauses; we will adopt them when it does.
        </p>
      </Clause>

      <Clause anchor="annex-5" number="ANNEX 5" title="Regional data protection terms">
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Nigeria (NDPA 2023 &amp; GAID 2025):</strong> Drafted to
              cover the processor terms required by NDPA Section 29 and GAID Article 34(2). Preserves Data Subjects'
              rights to lodge complaints with the NDPC and bring claims before a Federal or State High Court under GAID
              Article 47.
            </>,
            <>
              <strong className="font-medium text-text-primary">
                US State Privacy Laws (CCPA / CPRA &amp; peers):
              </strong>{" "}
              dimmi acts strictly as a "Service Provider" / "Processor" and certifies compliance with Clause VII.2
              prohibitions (no selling, no sharing, no outside-relationship use), notifies Customer if unable to meet
              obligations, and permits reasonable remediation steps.
            </>,
            <>
              <strong className="font-medium text-text-primary">Other jurisdictions:</strong> Where Customer Personal
              Data is subject to Brazil's LGPD, South Africa's POPIA, Kenya's DPA 2019, Ghana's DPA 2012, or Canada's
              PIPEDA, the operative clauses of this DPA are intended to satisfy the written processor/operator contract
              requirements of those statutes.
            </>,
          ]}
        />
      </Clause>
    </>
  );
}
