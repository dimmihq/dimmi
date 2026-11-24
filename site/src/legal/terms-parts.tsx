import { GITHUB_URL } from "../links";
import { LEGAL_CONTACTS, LEGAL_ENTITY } from "./constants";
import { Clause, LegalLink, LegalList, PartHeader, SubHeading } from "./layout";

export function TermsProductSchedule() {
  return (
    <>
      <PartHeader
        anchor="product-terms"
        numeral="II"
        title="Product-Specific Terms"
        binding="Schedule to Terms of Service"
        subtitle="Additional contractual rules that apply whenever Customer enables AI agents, email broadcasts, Meta channels, Slack, the web messenger, developer APIs, or Beta Features."
      />

      <Clause anchor="pt-ai" number="II.1" title="AI agents, procedures, Operator, and routines">
        <SubHeading>Definition of training</SubHeading>
        <p>
          For the purposes of this Agreement, <strong className="font-medium text-text-primary">"training"</strong>{" "}
          means modifying, updating, or optimizing the weights or parameters of a machine learning model using Customer
          Data. Generating vector embeddings, indexing Customer's own knowledge sources for retrieval, assembling
          prompts, or running inference strictly to serve Customer's own Workspace is not training.
        </p>

        <SubHeading>No training and provider retention</SubHeading>
        <p>
          Neither dimmi nor any AI subprocessor listed in our{" "}
          <LegalLink href="/privacy#subprocessors">Subprocessor List</LegalLink> (Anthropic, OpenAI, Voyage AI, and
          Cloudflare for the Clef decision model) trains models on Customer Data. Voyage AI and Cloudflare (Clef) retain
          zero Customer Data after processing a request. Anthropic and OpenAI do not train on API traffic. They retain
          requests for up to 30 days under their standard commercial API terms and then delete them, subject to
          Anthropic's published trust-and-safety exception allowing retention of flagged abuse content for up to 2
          years. dimmi retains full AI request and response debugging logs in its own German database for 30 days.
        </p>

        <SubHeading>Bring your own AI provider key (BYO key)</SubHeading>
        <p>
          Customer may connect its own API key for a supported AI model provider. When Customer uses a BYO key, AI
          Conversations are billed at dimmi's human Conversation rate, and that AI provider acts strictly as Customer's
          own vendor under Customer's separate agreement with that provider, not as dimmi's subprocessor.
        </p>

        <SubHeading>Accuracy, procedures, and actions</SubHeading>
        <p>
          AI Output is probabilistic and may be inaccurate, incomplete, or inconsistent with Customer's policies.
          Customer is solely responsible for: (a) the accuracy and legality of all knowledge sources, help articles,
          websites, and instructions connected to an agent or Operator; (b) testing procedures and configuring
          confirmation or teammate-approval steps before enabling automated actions; and (c) reviewing changesets and
          drafts proposed by Operator or routines before publishing them.
        </p>
        <p>
          AI actions and workflow HTTP/MCP requests that read from or mutate Customer's own backend systems execute
          solely on Customer's configuration and instruction and at Customer's own risk. Customer warrants that it holds
          all necessary rights and licenses to every website, document, repository, or third-party knowledge base it
          imports into dimmi Cloud.
        </p>

        <SubHeading>Mandatory AI disclosure and human escalation</SubHeading>
        <p>
          In line with Article 50 of the EU AI Act, Meta's messaging rules, and AI vendor policies, dimmi labels its
          customer-facing AI agents as AI in the web messenger, and on every Channel the agent says it is an AI whenever
          an End Customer asks or appears to assume it is a person. On Channels without a messenger interface (such as
          email, WhatsApp, Messenger, Instagram, and Slack), Customer is responsible for telling End Customers that an
          AI may reply, where applicable law requires that notice at the first interaction. Customer must not
          misrepresent a dimmi agent as a human or disable AI identity disclosures where required by applicable law
          (including for End Customers located in the European Union). Customer must maintain a working escalation and
          handover path to a human Team Member on every Channel where an AI agent replies.
        </p>

        <SubHeading>Prohibited solely automated high-impact decisions</SubHeading>
        <p>
          In accordance with Section 37 of the Nigeria Data Protection Act 2023 (NDPA), Article 22 of the GDPR, and
          upstream AI provider usage policies, Customer must not configure any dimmi agent, procedure, or Operator
          routine to make solely automated decisions that produce legal or similarly significant effects on an
          individual (including decisions regarding creditworthiness, lending, employment, housing, insurance
          underwriting, healthcare diagnosis or treatment, or legal status) without qualified human review and an
          accessible mechanism for the affected individual to contest the decision.
        </p>

        <SubHeading>Spend caps and fair use</SubHeading>
        <p>
          AI Features are subject to Workspace spend caps (defaulting to US$25 per month for AI usage) and fair-use rate
          limits. When a Workspace reaches its configured AI spend cap, automated AI agent replies and Operator runs
          pause until Customer raises the cap or the next billing cycle begins; human inbox replies continue unaffected.
        </p>
      </Clause>

      <Clause anchor="pt-email" number="II.2" title="Email channel, broadcasts, and series">
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Customer is the sender:</strong> Customer (not dimmi) is
              the sole legal sender and initiator of every email, broadcast, and series sent from its Workspace under
              CAN-SPAM, Canada's CASL, GDPR/ePrivacy, and Article 18(1)(a) of the Nigerian GAID 2025. Customer must hold
              verifiable prior consent wherever required by law, including express consent for all direct marketing
              emails sent to recipients in Nigeria.
            </>,
            <>
              <strong className="font-medium text-text-primary">List hygiene and mandatory disclosures:</strong>{" "}
              Customer must not upload or email purchased, rented, scraped, or third-party-harvested contact lists.
              Every marketing broadcast and series email must contain an accurate sender identity, Customer's valid
              physical postal address, and a working one-click unsubscribe link (which dimmi inserts automatically).
              Customer must honor unsubscribe requests promptly and never re-subscribe opted-out recipients without new
              affirmative consent.
            </>,
            <>
              <strong className="font-medium text-text-primary">Open and click tracking:</strong> Broadcast and series
              emails always include an open-tracking pixel, and rewrite links for click tracking by default (click
              tracking can be toggled off per broadcast or series step; transactional and support replies carry no
              tracking). Customer is responsible for disclosing email tracking to recipients and obtaining consent where
              required by applicable law.
            </>,
            <>
              <strong className="font-medium text-text-primary">Automated reputation pauses:</strong> To protect shared
              email deliverability and comply with Amazon SES enforcement limits, dimmi automatically pauses bulk email
              sending (broadcasts, series, and satisfaction-rating emails) for a Workspace if, over any rolling 7-day
              window with at least 100 emails sent, its hard bounce rate reaches 5% or its spam complaint rate reaches
              0.1%. An individual broadcast also pauses automatically if its spam complaint rate reaches 0.3%. Direct
              support replies from teammates and agents continue during a bulk sending pause. A Workspace Owner may
              resume bulk sending after remediating list quality.
            </>,
            <>
              <strong className="font-medium text-text-primary">External email infrastructure and scanning:</strong>{" "}
              Outbound and inbound email transits Amazon Web Services (AWS SES, S3, and SNS in Frankfurt, Germany,
              eu-central-1), which scans email in transit for spam, phishing, and malware. dimmi does not guarantee
              inbox placement or delivery to any recipient mailbox.
            </>,
          ]}
        />
      </Clause>

      <Clause anchor="pt-meta" number="II.3" title="WhatsApp, Facebook Messenger, and Instagram">
        <p>
          Customer connects its own Meta business accounts through dimmi's Meta integration. By enabling WhatsApp,
          Facebook Messenger, or Instagram, Customer agrees to the following mandatory flow-down terms required by Meta
          Platforms, Inc.:
        </p>
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Meta policy compliance:</strong> Customer maintains its
              own direct contract with Meta and must comply with the Meta Platform Terms, WhatsApp Business Terms of
              Service, WhatsApp Business Messaging Policy, and Messenger Platform Policy, including collecting valid
              user opt-in consent before messaging, respecting Meta's 24-hour customer-service reply window, using
              Meta-approved templates outside that window, and complying with all prohibited business categories.
            </>,
            <>
              <strong className="font-medium text-text-primary">Prohibited sensitive data:</strong> Customer must not
              request, collect, or transmit payment card numbers, bank account numbers, government-issued identification
              numbers, or protected health information through WhatsApp, Messenger, or Instagram.
            </>,
            <>
              <strong className="font-medium text-text-primary">Restriction on general-purpose AI assistants:</strong>{" "}
              Customer may use dimmi AI agents on WhatsApp solely for customer support and business communications
              incidental to Customer's own products or services, and must not use our integration to offer a standalone
              or general-purpose AI chatbot or assistant. Customer must maintain a clear, working escalation path to a
              human agent.
            </>,
            <>
              <strong className="font-medium text-text-primary">Public privacy policy and Meta takedowns:</strong>{" "}
              Customer must maintain a publicly accessible privacy policy covering its Meta messaging practices. If Meta
              instructs dimmi to restrict, suspend, or terminate access for Customer's account, dimmi must comply
              immediately without liability or refund.
            </>,
          ]}
        />
      </Clause>

      <Clause anchor="pt-slack" number="II.4" title="Slack">
        <p>
          When Customer installs the dimmi Slack application, Customer explicitly authorizes dimmi to access, process,
          and store messages, files, and metadata from the Slack channels Customer selects, and to transmit relevant
          conversation content to the AI subprocessors listed in our{" "}
          <LegalLink href="/privacy#subprocessors">Subprocessor List</LegalLink> when AI Features or Operator are
          enabled in Slack.
        </p>
        <p>
          Customer must comply with Slack's API Terms of Service and Acceptable Use Policy. Upon uninstallation of the
          dimmi app from Customer's Slack workspace (or upon Customer's deletion request), dimmi deletes stored Slack
          workspace tokens and Slack-sourced message data in accordance with Slack's required deletion timeline (within
          14 business days of uninstall, except where Customer has independently exported or retained records as part of
          its own support history under applicable law).
        </p>
      </Clause>

      <Clause anchor="pt-messenger" number="II.5" title="Web messenger">
        <p>
          The dimmi web messenger operates as Customer's support tool embedded on Customer's website or application.
          dimmi acts solely on Customer's instructions and never intercepts or uses web messenger transcripts for our
          own advertising, profiling, or independent commercial purposes.
        </p>
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Chat recording notice and consent:</strong> Customer is
              solely responsible for informing website visitors before a conversation begins that chats are recorded and
              processed by Customer's support service provider (dimmi) and for obtaining affirmative consent where
              required by applicable wiretap, privacy, or communications laws (including California Penal Code Section
              631 and Article 31 of the Nigerian GAID 2025).
            </>,
            <>
              <strong className="font-medium text-text-primary">Loading control and browser storage notice:</strong> The
              web messenger sets no HTTP cookies, but writes a session token, unsent message outbox, and last-seen
              marker to the browser's local storage when loaded. Customer controls when and on which pages the web
              messenger script loads (including gating it behind Customer's own consent manager where required by
              ePrivacy rules) and is responsible for disclosing that local storage in Customer's own privacy and cookie
              notices. Customers may link to our{" "}
              <LegalLink href="/privacy#end-user-notice">End-User Privacy Notice</LegalLink>.
            </>,
          ]}
        />
      </Clause>

      <Clause anchor="pt-api" number="II.6" title="REST API, SDKs, webhooks, and MCP server">
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Rate limits and credentials:</strong> Customer must
              respect published API rate limits, protect API tokens and OAuth credentials from unauthorized exposure,
              and avoid circumventing usage metering or security controls.
            </>,
            <>
              <strong className="font-medium text-text-primary">API stability and deprecation:</strong> Public versioned
              REST API routes (`/api/v1`) and webhook contracts follow an additive-only stability policy. We will give
              at least 12 months' advance notice (including `Deprecation` and `Sunset` headers) before removing any
              versioned public API route.
            </>,
            <>
              <strong className="font-medium text-text-primary">MCP server and third-party AI agents:</strong> When a
              Team Member authorizes a third-party application or external AI agent via OAuth or API token to connect to
              dimmi's MCP server (`/mcp`), that external client inherits the authorizing Team Member's scoped
              permissions and can read or modify Workspace data accordingly. Customer is solely responsible for vetting,
              authorizing, and monitoring every third-party client or agent it connects.
            </>,
            <>
              <strong className="font-medium text-text-primary">Open-source SDKs:</strong> dimmi's client and server
              SDKs (TypeScript, Go, PHP, Laravel, Python, browser, React, Next.js, Vue, Angular, and Svelte) are
              distributed under their respective open-source licenses.
            </>,
          ]}
        />
      </Clause>

      <Clause anchor="pt-beta" number="II.7" title="Beta features">
        <p>
          Beta Features are optional and clearly marked in the interface or Documentation. They may contain defects,
          change without notice, or be discontinued at any time, and are excluded from all warranties, indemnities, and
          service-level commitments.
        </p>
      </Clause>
    </>
  );
}

export function TermsAupAndSelfHosting() {
  return (
    <>
      <PartHeader
        anchor="acceptable-use"
        numeral="III"
        title="Acceptable Use Policy (AUP)"
        binding="Incorporated in Terms of Service"
        subtitle="Permitted and prohibited uses of dimmi Cloud, protecting end users, shared email reputation, and mandatory rules from Hetzner, AWS, Meta, Slack, Anthropic, OpenAI, and Cloudflare."
      />

      <Clause anchor="aup-security" number="III.1" title="Unlawful content and security abuse">
        <p>
          Customer, its Team Members, and authorized integrations must not use dimmi Cloud to store, transmit, publish,
          or facilitate:
        </p>
        <LegalList
          items={[
            "Content that is unlawful, defamatory, fraudulent, deceptive, harassing, violent, sexually exploitative, or infringes any third party's copyright, trademark, privacy, or other legal rights.",
            "Malware, viruses, ransomware, spyware, phishing pages, credential-harvesting links, or malicious attachments.",
            "Denial-of-service attacks, unauthorized network scanning, penetration testing without prior written consent from dimmi, or attempts to probe, bypass, or disrupt dimmi Cloud or any third-party network.",
            "Knowledge imports, web crawls, or data migrations from websites, help centers, or third-party tools (including Zendesk, Intercom, Notion, or GitHub) unless Customer holds the legal right to import and use that content.",
          ]}
        />
      </Clause>

      <Clause anchor="aup-messaging" number="III.2" title="Email, broadcasts, Meta, and Slack rules">
        <LegalList
          items={[
            "Never send unsolicited bulk email (spam) or use purchased, rented, scraped, or third-party-shared mailing lists. Hold verifiable consent wherever required under CAN-SPAM, CASL, GDPR/ePrivacy, TCPA, and GAID Article 18(1)(a) (mandatory consent for direct marketing to Nigerian recipients).",
            "Include accurate sender identification, your valid physical postal address, and a functional unsubscribe link in every marketing broadcast and series, and stay below our sending-reputation thresholds (5% hard bounces and 0.1% spam complaints over 7 days once 100 emails are sent, and 0.3% complaints on a single broadcast).",
            "Hold valid opt-in consent for every WhatsApp, Messenger, and Instagram message, follow Meta's 24-hour window, template rules, and prohibited business categories, never transmit card numbers, government IDs, or health data over Meta channels, and never offer a general-purpose AI assistant over WhatsApp.",
            "Comply with Slack's Acceptable Use Policy and authorize Slack channel access properly.",
            "Notify web messenger visitors before a chat begins that conversations are recorded and processed by a service provider, and collect consent where required by law (including California Penal Code Section 631 and GAID Article 31).",
          ]}
        />
      </Clause>

      <Clause anchor="aup-ai-sensitive" number="III.3" title="AI safeguards, restricted data, and platform integrity">
        <LegalList
          items={[
            "Never present a customer-facing AI agent as human, never remove required AI disclosures (EU AI Act Article 50), always provide a human handover path, and never use AI for solely automated decisions with legal or similarly significant effects (credit, employment, housing, insurance, healthcare, legal status) without qualified human review (NDPA Section 37 and GDPR Article 22).",
            "Never violate Anthropic's Usage Policy or OpenAI's Usage Policies.",
            "Without a separate written agreement, never process HIPAA Protected Health Information (PHI), deploy dimmi Cloud on properties directed at children (under 13 under US COPPA or under 18 without required parental consent under NDPA Section 31), or intentionally store special-category/sensitive personal data in custom attributes (NDPA Section 30).",
            "Never resell, sublicense, or white-label dimmi Cloud as a standalone hosted service without an executed partner agreement, circumvent usage metering or rate limits, or reverse-engineer proprietary dimmi Cloud infrastructure.",
          ]}
        />
      </Clause>

      <Clause anchor="aup-enforcement" number="III.4" title="Enforcement and reporting abuse">
        <p>
          Where practical, we notify Customer of a suspected AUP violation and give a reasonable window to cure it. We
          may immediately remove offending content, pause bulk sending, or suspend a Channel or Workspace without prior
          notice for active security threats, illegal content, severe deliverability harm, or a binding demand from an
          upstream provider (such as Meta, AWS, or Hetzner).
        </p>
        <p>
          To report spam, phishing, infringement, illegal content, or other abuse originating from a dimmi Cloud
          workspace or help center, email{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> with the relevant
          URL, timestamps, headers, or screenshots.
        </p>
      </Clause>

      <PartHeader
        anchor="self-hosting-notice"
        numeral="IV"
        title="Self-Hosting & Open-Source Notice"
        binding="Published notice"
        subtitle="Clear boundary between the MIT-licensed self-hosted dimmi edition and hosted dimmi Cloud."
      />

      <Clause anchor="sh-mit" number="IV.1" title="MIT open-source license and what installs send us">
        <p>
          The open-source dimmi codebase at <LegalLink href={GITHUB_URL}>github.com/dimmihq/dimmi</LegalLink> is
          licensed under the MIT License and provided "AS IS" without warranty, support, data recovery, or service-level
          commitments from {LEGAL_ENTITY.name}. Self-hosted installations send{" "}
          <strong className="font-medium text-text-primary">no data</strong> to {LEGAL_ENTITY.name} today. If a later
          release adds an install count or an update check, it will carry no customer content, be described in the
          release notes and documentation, and have a setting that turns it off.
        </p>
      </Clause>

      <Clause anchor="sh-exclusion" number="IV.2" title="Cloud terms exclusion, operator duties, and trademarks">
        <p>
          Because {LEGAL_ENTITY.name} has no access to self-hosted deployments, we are neither Data Controller nor Data
          Processor for them, and the dimmi Cloud Terms of Service, Privacy Policy, and DPA do not apply. The
          organization operating a self-hosted instance is solely responsible for server security, backups, direct
          vendor contracts and DPAs (email, AI, Meta, Slack), and compliance with applicable laws.
        </p>
        <p>
          The MIT License covers source code only and does not license the{" "}
          <strong className="font-medium text-text-primary">dimmi</strong> name, wordmark, or logo to imply endorsement
          or market a competing hosted service as "dimmi" or "dimmi Cloud." You may truthfully state that your instance
          is "Powered by dimmi." Licensing questions may be sent to{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>.
        </p>
      </Clause>
    </>
  );
}
