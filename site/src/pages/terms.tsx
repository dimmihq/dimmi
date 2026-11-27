import { LEGAL_CONTACTS, LEGAL_ENTITY, LEGAL_VERSION } from "../legal/constants";
import {
  Clause,
  LegalDocument,
  LegalEntityFooter,
  LegalLink,
  LegalList,
  LegalTable,
  NoticeBox,
  PartHeader,
  type PartSummary,
} from "../legal/layout";
import { TermsAupAndSelfHosting, TermsProductSchedule } from "../legal/terms-parts";

const parts: PartSummary[] = [
  {
    id: "core-terms",
    numeral: "I",
    title: "Core Terms of Service",
    description: "Contract parties, fees, IP, liability caps, switching, and Lagos arbitration.",
    items: [
      { id: "parties", label: "1. Parties & acceptance" },
      { id: "structure", label: "2. Structure & precedence" },
      { id: "definitions", label: "3. Definitions" },
      { id: "service", label: "4. The service" },
      { id: "accounts", label: "5. Accounts & security" },
      { id: "acceptable-use-ref", label: "6. Acceptable use" },
      { id: "channels", label: "7. Channels & platforms" },
      { id: "ai-features", label: "8. AI features" },
      { id: "customer-data", label: "9. Customer data" },
      { id: "data-protection", label: "10. Data protection & security" },
      { id: "free-and-beta", label: "11. Free use & beta features" },
      { id: "fees", label: "12. Fees & payment" },
      { id: "ip", label: "13. Intellectual property" },
      { id: "publicity", label: "14. Publicity" },
      { id: "confidentiality", label: "15. Confidentiality" },
      { id: "warranties", label: "16. Warranties & disclaimers" },
      { id: "indemnities", label: "17. Indemnities" },
      { id: "liability", label: "18. Limitation of liability" },
      { id: "term", label: "19. Term & termination" },
      { id: "export", label: "20. Export & deletion" },
      { id: "switching", label: "21. Provider switching" },
      { id: "compliance", label: "22. Compliance & requests" },
      { id: "governing-law", label: "23. Governing law & disputes" },
      { id: "general", label: "24. General & version history" },
    ],
  },
  {
    id: "product-terms",
    numeral: "II",
    title: "Product-Specific Terms",
    description: "AI agents, email pauses, Meta & Slack rules, web messenger, APIs, and beta.",
    items: [
      { id: "pt-ai", label: "II.1 AI agents & Operator" },
      { id: "pt-email", label: "II.2 Email & broadcasts" },
      { id: "pt-meta", label: "II.3 WhatsApp, Messenger & IG" },
      { id: "pt-slack", label: "II.4 Slack" },
      { id: "pt-messenger", label: "II.5 Web messenger" },
      { id: "pt-api", label: "II.6 API, SDKs, webhooks & MCP" },
      { id: "pt-beta", label: "II.7 Beta features" },
    ],
  },
  {
    id: "acceptable-use",
    numeral: "III",
    title: "Acceptable Use Policy",
    description: "Prohibited uses, anti-spam, AI safeguards, restricted data, and abuse reporting.",
    items: [
      { id: "aup-security", label: "III.1 Unlawful content & abuse" },
      { id: "aup-messaging", label: "III.2 Email, Meta & Slack" },
      { id: "aup-ai-sensitive", label: "III.3 AI & sensitive data" },
      { id: "aup-enforcement", label: "III.4 Enforcement & abuse reports" },
    ],
  },
  {
    id: "self-hosting-notice",
    numeral: "IV",
    title: "Self-Hosting Notice",
    description: "MIT open-source boundary, what installs send us, and trademark rules.",
    items: [
      { id: "sh-mit", label: "IV.1 MIT license & install data" },
      { id: "sh-exclusion", label: "IV.2 Cloud exclusion & brand" },
    ],
  },
];

export function TermsPage() {
  return (
    <LegalDocument
      path="/terms"
      title="Terms of Service"
      subtitle="Everything governing your business use of dimmi Cloud in one complete agreement: Core Terms, Product-Specific Terms, Acceptable Use Policy, and our Self-Hosting Notice."
      parts={parts}
    >
      <NoticeBox
        title="Important notice on liability caps, disclaimers, and indemnities (FCCPA ss. 127 to 129)"
        tone="emphasis"
      >
        <p>
          Before creating a workspace or using dimmi Cloud, please review Clauses 16 (Warranties and Disclaimers), 17
          (Indemnities), and 18 (Limitation of Liability):
        </p>
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Liability cap:</strong> Each party's aggregate liability
              is capped at fees paid or payable in the 12 months before the first event giving rise to the claim (US$500
              floor), with a 2x cap (US$1,000 floor) for data protection breaches, confidentiality breaches, and
              indemnities. Indemnities: each party defends the other against the third-party claims listed in Clause 17,
              within that 2x cap.
            </>,
            <>
              <strong className="font-medium text-text-primary">Non-excludable liability:</strong> Nothing limits
              liability for fraud, willful misconduct, gross negligence, payment obligations, or statutory rights that
              cannot lawfully be excluded.
            </>,
            <>
              <strong className="font-medium text-text-primary">AI output &amp; third-party channels:</strong> AI
              replies may contain errors and carry no accuracy warranty; email, Meta, and Slack availability depends on
              third parties outside our control.
            </>,
            <>
              <strong className="font-medium text-text-primary">Plain-English notes:</strong> The short accent-bordered
              note beneath each clause is strictly for readability and has no legal force. The numbered clause text
              governs.
            </>,
          ]}
        />
      </NoticeBox>

      <PartHeader
        anchor="core-terms"
        numeral="I"
        title="Core Terms of Service"
        binding="Binding contract (clickwrap)"
        subtitle="The main commercial agreement between Hivy Technologies Limited (RC 9601398) and every business operating a workspace on dimmi Cloud."
      />

      <Clause
        anchor="parties"
        number="01"
        title="Parties and acceptance"
        summary="You are contracting with Hivy Technologies Limited (RC 9601398, Nigeria) on behalf of a business. You must be at least 18 and authorized to bind that business."
      >
        <p>
          These Terms of Service (the <strong className="font-medium text-text-primary">"Terms"</strong>) form a legally
          binding agreement between <strong className="font-medium text-text-primary">{LEGAL_ENTITY.name}</strong>, a
          private company limited by shares incorporated in the {LEGAL_ENTITY.country} on{" "}
          {LEGAL_ENTITY.incorporationDate} under registration number{" "}
          <strong className="font-medium text-text-primary">{LEGAL_ENTITY.rcNumber}</strong> with its registered office
          at {LEGAL_ENTITY.address} (<strong className="font-medium text-text-primary">"dimmi"</strong>,{" "}
          <strong className="font-medium text-text-primary">"we"</strong>, or{" "}
          <strong className="font-medium text-text-primary">"us"</strong>), and the business entity that creates a
          workspace, accepts these Terms, or executes an Order Form referencing them (
          <strong className="font-medium text-text-primary">"Customer"</strong> or{" "}
          <strong className="font-medium text-text-primary">"you"</strong>).
        </p>
        <p>
          By checking the acceptance box at sign-up, creating or accessing a dimmi Cloud Workspace, or signing an Order
          Form, the individual accepting these Terms represents and warrants that: (a) they are at least 18 years of
          age; (b) they have full legal authority to bind Customer to this Agreement; and (c) Customer is entering into
          this Agreement solely for business, commercial, or organizational purposes and not as a personal consumer.
        </p>
        <p>
          These Terms apply solely to <strong className="font-medium text-text-primary">dimmi Cloud</strong>.
          Self-hosted installations of the open-source dimmi codebase are governed exclusively by the MIT license and
          Part IV below (<LegalLink href="#self-hosting-notice">Self-Hosting &amp; Open-Source Notice</LegalLink>).
        </p>
      </Clause>

      <Clause
        anchor="structure"
        number="02"
        title="Structure and order of precedence"
        summary="If two documents conflict, EU/UK transfer clauses win on transfer rules, then a signed Order Form, then Product Terms, then the DPA, then Core Terms, then the AUP."
      >
        <p>
          The complete agreement between the parties (the{" "}
          <strong className="font-medium text-text-primary">"Agreement"</strong>) consists of: (a) any executed Order
          Form; (b) Part II of this page (<LegalLink href="#product-terms">Product-Specific Terms</LegalLink>); (c) our{" "}
          <LegalLink href="/privacy#dpa">Data Processing Agreement (DPA)</LegalLink>, including its annexes and{" "}
          <LegalLink href="/privacy#subprocessors">Subprocessor List</LegalLink>; (d) Part I of this page (Core Terms of
          Service); and (e) Part III of this page (<LegalLink href="#acceptable-use">Acceptable Use Policy</LegalLink>).
        </p>
        <p>
          The plain-English summaries shown beside clauses are for convenience only and are not part of this Agreement.
          If there is a conflict between documents forming this Agreement, the following order of precedence applies:
        </p>
        <LegalList
          items={[
            "First, the EU Standard Contractual Clauses, UK Addendum, or other statutory transfer clauses incorporated under the DPA, solely on data protection and international transfer matters.",
            "Second, the DPA, on data protection and security matters.",
            "Third, an executed Order Form, solely to the extent it expressly overrides a named clause of these Terms.",
            "Fourth, Part II (Product-Specific Terms) for the specific feature or channel in question.",
            "Fifth, Part I (Core Terms of Service).",
            "Sixth, Part III (Acceptable Use Policy).",
          ]}
        />
        <p>
          Any pre-printed, standard, or additional terms contained in a Customer purchase order, vendor portal, or
          invoice acknowledgment are void and have no legal effect, even if accepted administratively by dimmi.
        </p>
      </Clause>

      <Clause
        anchor="definitions"
        number="03"
        title="Definitions"
        summary="Exact meanings for capitalized terms used across our contract. AI inputs and outputs are Customer Data; account credentials and Paystack billing records are Account Data."
      >
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">"Account Data"</strong> means information we collect and
              control to create, secure, administer, and bill a Workspace, including Team Member names, business email
              addresses, password hashes, authentication credentials, sign-in logs, audit records, and Paystack billing
              details. Account Data is excluded from Customer Data and is governed by our{" "}
              <LegalLink href="/privacy">Privacy Policy</LegalLink>.
            </>,
            <>
              <strong className="font-medium text-text-primary">"AI Input"</strong> means any prompt, message, knowledge
              passage, contact attribute, procedure, or action result submitted to an AI Feature on Customer's behalf.
            </>,
            <>
              <strong className="font-medium text-text-primary">"AI Output"</strong> means any reply, summary, draft,
              ticket field, changeset, or suggestion generated by an AI Feature in response to AI Input.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Beta Feature"</strong> means any feature, integration,
              or tool expressly labeled as beta, preview, early access, or experimental.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Channel"</strong> means any communication interface
              through which Customer exchanges messages with End Customers using dimmi Cloud, including the web
              messenger, email, WhatsApp, Facebook Messenger, Instagram, Slack, or API.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Paying Customer"</strong> means a Customer that dimmi
              charged Fees in at least one of the three calendar months before the event in question.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Term"</strong> means the period from Customer's
              acceptance of this Agreement until it ends under Clause 19.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Conversation"</strong> means a single message thread
              between Customer and an End Customer on any Channel, or an internal Operator thread started by a Team
              Member, counted in accordance with Clause 12 and the pricing page.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Customer Data"</strong> means all electronic data,
              messages, attachments, End Customer contact records, tickets, help center articles, knowledge sources,
              workflows, procedures, AI Input, and AI Output submitted to, stored in, or generated within Customer's
              Workspace. Customer Data excludes Account Data, Service Activity Data, and Usage Data.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Documentation"</strong> means dimmi's official product
              documentation published at trydimmi.com/docs, as updated from time to time.
            </>,
            <>
              <strong className="font-medium text-text-primary">"End Customer"</strong> means any individual or entity
              that communicates with Customer through a Channel, reads Customer's help center or changelog, submits
              feedback, or receives broadcasts or series from Customer.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Order Form"</strong> means a written ordering document
              executed by both parties specifying custom pricing, committed volume, support tiers, or modified terms.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Service Activity Data"</strong> means technical data
              about how Team Members use dimmi Cloud: the pages and features they use, the actions they take,
              performance and errors, and masked recordings of their dashboard sessions.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Team Member"</strong> means an employee, contractor, or
              authorized representative invited by Customer to access Customer's Workspace.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Third-Party Platform"</strong> means any external
              service or account that Customer connects to dimmi Cloud under Customer's own contract with that third
              party, including Meta (WhatsApp, Messenger, Instagram), Slack, Google, GitHub, Linear, Jira, Notion,
              Intercom, Zendesk, PagerDuty, incident.io, Rootly, SAML/OIDC identity providers, and Customer's own AI
              provider keys.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Usage Data"</strong> means aggregated, de-identified
              operational, diagnostic, and performance metrics relating to how dimmi Cloud runs (such as request
              latency, error counts, and feature adoption counts) that do not identify Customer, any Team Member, or any
              End Customer and contain no Customer Data content.
            </>,
            <>
              <strong className="font-medium text-text-primary">"Workspace"</strong> means Customer's isolated tenant
              environment within dimmi Cloud.
            </>,
          ]}
        />
      </Clause>

      <Clause
        anchor="service"
        number="04"
        title="The service"
        summary="We provide dimmi Cloud for your business operations. We improve it continuously and will not materially strip away core functionality during a prepaid Order Form term."
      >
        <p>
          Subject to this Agreement and timely payment of applicable fees, dimmi grants Customer a non-exclusive,
          non-transferable, non-sublicensable right during the Term to access and use dimmi Cloud for Customer's
          internal business support, customer communication, and help center operations in accordance with the
          Documentation.
        </p>
        <p>
          We may update, modify, and improve dimmi Cloud from time to time to add functionality, enhance security, or
          comply with law. We will not materially reduce the core functionality of dimmi Cloud during an active prepaid
          Order Form term. If we plan to discontinue a material feature or API route, we will give advance notice by
          email and in-app (and at least 12 months' notice before removing a versioned public REST API route).
        </p>
      </Clause>

      <Clause
        anchor="accounts"
        number="05"
        title="Accounts and security"
        summary="One login per person (seats are free anyway), accurate registration details, keep your credentials and API tokens safe, and alert us promptly if compromised."
      >
        <p>
          Customer must provide accurate, complete, and current registration and billing details and keep them updated.
          Seats on dimmi Cloud are unlimited and free; accordingly, each Team Member must use an individual login
          credential and must not share passwords, passkeys, or personal API tokens with other individuals.
        </p>
        <p>
          Customer is responsible for: (a) maintaining the confidentiality of all Workspace credentials, API tokens,
          OAuth grants, webhook secrets, and connected Third-Party Platform keys; (b) configuring Workspace roles,
          access policies, two-factor authentication, and IP allowlists appropriate to its risk profile; and (c) all
          activity occurring under its Workspace and Team Member accounts. Customer must notify dimmi promptly at{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> upon becoming aware
          of any unauthorized access or suspected compromise of Workspace credentials.
        </p>
      </Clause>

      <Clause
        anchor="acceptable-use-ref"
        number="06"
        title="Acceptable use"
        summary="Follow Part III (Acceptable Use Policy) and ensure your end customers get all legally required notices and give valid consents."
      >
        <p>
          Customer and its Team Members must comply with Part III of this document (
          <LegalLink href="#acceptable-use">Acceptable Use Policy</LegalLink>), which is incorporated into this
          Agreement. Customer is solely responsible for providing all privacy notices and obtaining all consents,
          permissions, and legal authorizations required by applicable law from End Customers and Team Members before
          collecting, recording, messaging, importing, or processing their personal data through dimmi Cloud.
        </p>
        <p>
          If Customer or any content in Customer's Workspace violates the AUP or applicable law, dimmi may remove or
          disable access to the offending content or suspend the affected Channel or Workspace in accordance with Clause
          19.
        </p>
      </Clause>

      <Clause
        anchor="channels"
        number="07"
        title="Channels and third-party platforms"
        summary="When you connect Meta, Slack, or external integrations, you are the sender under your own contract with them. We do not control their uptime or policy actions."
      >
        <p>
          dimmi Cloud enables Customer to connect Third-Party Platforms such as WhatsApp Business Platform, Facebook
          Messenger, Instagram, Slack, custom email domains, issue trackers, knowledge tools, and BYO AI provider keys.
          Customer's use of each Third-Party Platform is governed solely by Customer's separate agreement with that
          provider. Customer represents that it holds all required opt-in consents and complies with each platform's
          policies, and acknowledges that Customer (not dimmi) is the legal sender and initiator of every outbound
          message, email, and broadcast sent through its Workspace.
        </p>
        <p>
          Third-Party Platforms may modify, throttle, restrict, or terminate API access or suspend Customer's account at
          any time. dimmi makes no availability or deliverability guarantee for Third-Party Platforms and owes no refund
          or service credit if a Third-Party Platform limits or terminates access due to Customer's conduct, policy
          violations, or platform changes. Additional channel rules appear in{" "}
          <LegalLink href="#product-terms">Part II (Product-Specific Terms)</LegalLink>.
        </p>
      </Clause>

      <Clause
        anchor="ai-features"
        number="08"
        title="AI features"
        summary="Neither dimmi nor our built-in AI subprocessors train models on your data. AI can make mistakes, so you control its knowledge, procedures, and disclosures."
      >
        <p>
          dimmi Cloud includes optional artificial intelligence features, including customer-facing dimmi agents,
          procedures, Operator, and routines (<strong className="font-medium text-text-primary">"AI Features"</strong>),
          governed by <LegalLink href="#pt-ai">Section II.1 of the Product-Specific Terms</LegalLink>.
        </p>
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">No model training:</strong> Neither dimmi nor any
              subprocessor listed in our <LegalLink href="/privacy#subprocessors">Subprocessor List</LegalLink> uses
              Customer Data to train, fine-tune, or improve any foundation or shared AI model.
            </>,
            <>
              <strong className="font-medium text-text-primary">Probabilistic output:</strong> AI Output is generated
              probabilistically and may be inaccurate, incomplete, or unsuitable for a given situation. Customer is
              solely responsible for reviewing its knowledge sources, procedures, actions, and AI Output before relying
              on them.
            </>,
            <>
              <strong className="font-medium text-text-primary">Disclosure and human oversight:</strong> Customer-facing
              AI agents are labelled as AI in the web messenger and say they are an AI whenever asked; on other Channels
              Customer gives any first-contact AI notice the law requires. Customer must maintain a working handover
              path to a human Team Member on every active AI Channel.
            </>,
          ]}
        />
      </Clause>

      <Clause
        anchor="customer-data"
        number="09"
        title="Customer data"
        summary="You own all your Customer Data. You grant us only the limited license needed to run, secure, and support your workspace. We never sell it or use it for ads."
      >
        <p>
          As between the parties, Customer retains all right, title, and interest in and to Customer Data, including AI
          Input and AI Output. Customer grants dimmi a worldwide, non-exclusive, royalty-free license during the Term
          (and any post-termination export window) solely to host, copy, process, transmit, and display Customer Data as
          necessary to: (a) provide, maintain, secure, and support dimmi Cloud in accordance with Customer's
          instructions and configuration; (b) prevent or address technical, security, or abuse issues; and (c) comply
          with applicable law.
        </p>
        <p>
          dimmi will never sell Customer Data, use Customer Data for targeted advertising, or share Customer Data for
          the benefit of other customers. dimmi may generate and own Usage Data to operate, measure, and improve service
          reliability and capacity, provided Usage Data is permanently aggregated and de-identified and never reveals
          Customer Data or identifies Customer or any individual.
        </p>
        <p>
          dimmi collects Service Activity Data and uses it only to operate, support, analyze, and improve dimmi. Service
          Activity Data is not Customer Data and never includes End Customers. dimmi never sells it or uses it for
          advertising. The <LegalLink href="/privacy">Privacy Policy</LegalLink> says what it holds and how long dimmi
          keeps it.
        </p>
      </Clause>

      <Clause
        anchor="data-protection"
        number="10"
        title="Data protection and security"
        summary="Our global DPA is automatically incorporated into this contract. We act as processor for your workspace data and controller for your account and billing records."
      >
        <p>
          Where dimmi processes personal data within Customer Data on Customer's behalf, Customer acts as Data
          Controller (or processor on behalf of its own clients) and dimmi acts as Data Processor (or subprocessor)
          under our <LegalLink href="/privacy#dpa">Data Processing Agreement</LegalLink>, which is automatically
          incorporated into this Agreement. For Account Data, website visits, and billing contacts, {LEGAL_ENTITY.name}{" "}
          acts as an independent Data Controller as described in our{" "}
          <LegalLink href="/privacy">Privacy Policy</LegalLink>.
        </p>
        <p>
          dimmi maintains the technical and organizational security controls described in our{" "}
          <LegalLink href="/privacy#security-overview">Security Overview</LegalLink> and{" "}
          <LegalLink href="/privacy#annex-2">DPA Annex 2</LegalLink>. We may update those controls over time provided we
          do not materially reduce the overall level of protection for Customer Data.
        </p>
      </Clause>

      <Clause
        anchor="free-and-beta"
        number="11"
        title="Free use and beta features"
        summary="The 100 free monthly conversations and optional Beta Features are provided as-is without SLA commitments. We give 30 days' notice before changing the free allowance."
      >
        <p>
          Every Workspace receives a monthly free allowance of 100 Conversations (and months totaling under US$10 are
          not charged) as described in Clause 12. Free-tier usage and optional Beta Features are provided strictly "AS
          IS" and "AS AVAILABLE," without warranties, service-level commitments, or IP indemnification from dimmi, and
          subject to the liability caps in Clause 18.
        </p>
        <p>
          We may modify or discontinue any Beta Feature at any time, and we may adjust the free monthly Conversation
          allowance on at least 30 days' advance notice by email and in-app.
        </p>
      </Clause>

      <Clause
        anchor="fees"
        number="12"
        title="Fees and payment"
        summary="Unlimited free seats, 100 free conversations every month, graduated conversation pricing up to 10,000/month, and $0.50 per 1,000 broadcast emails. Billed monthly in arrears in USD via Paystack."
      >
        <p>
          dimmi Cloud uses transparent usage-based pricing with unlimited free Team Member seats, free help centers, and
          free changelogs, as published on our <LegalLink href="/pricing">Pricing</LegalLink> page (or in an executed
          Order Form):
        </p>
        <LegalTable
          caption="dimmi Cloud monthly conversation rates"
          headers={[
            "Monthly volume band",
            "AI conversation (built-in models)",
            "Human conversation (or AI with BYO key)",
          ]}
          rows={[
            ["First 100 conversations", "Free (applied to AI first, then human)", "Free"],
            ["101 to 500", "10¢ per conversation", "5¢ per conversation"],
            ["501 to 1,000", "8¢ per conversation", "4¢ per conversation"],
            ["1,001 to 2,500", "8¢ per conversation", "2¢ per conversation"],
            ["2,501 to 5,000", "6¢ per conversation", "1.5¢ per conversation"],
            ["5,001 to 10,000", "6¢ per conversation", "1¢ per conversation"],
            ["Above 10,000", "Custom pricing via Order Form", "Custom pricing via Order Form"],
          ]}
        />
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Conversation counting:</strong> Each Conversation counts
              once until it has remained quiet for 7 consecutive days. A Conversation counts at the AI rate if a dimmi
              agent replies at least once (or for Operator threads started by a Team Member), and every 10 AI replies
              within the same Conversation count as one additional Conversation. Outbound messages remain free until the
              End Customer replies. Bringing your own AI provider API key prices AI Conversations at the human rate.
            </>,
            <>
              <strong className="font-medium text-text-primary">Broadcast and series email:</strong> Charged at US$0.50
              per 1,000 outbound broadcast or series emails sent.
            </>,
            <>
              <strong className="font-medium text-text-primary">Minimum billing threshold and payment method:</strong>{" "}
              No payment card is required to start; there is no expiring trial period. A valid payment method via
              Paystack is required at Conversation 101 in a calendar month or before sending your first email broadcast.
              Any calendar month whose total charges are under US$10.00 is waived and not billed.
            </>,
            <>
              <strong className="font-medium text-text-primary">Spend caps:</strong> Workspaces can set monthly spend
              caps with pre-cap alerts. The AI spend cap defaults to US$25 per month until changed by Customer. When an
              AI spend cap is reached, automated AI replies pause while human inbox replies continue uninterrupted.
            </>,
            <>
              <strong className="font-medium text-text-primary">Billing, currency, and payment processor:</strong>{" "}
              Self-serve charges are billed monthly in arrears in United States Dollars (USD) and processed exclusively
              through <strong className="font-medium text-text-primary">Paystack Payments Limited</strong> (or via
              invoice under an executed Order Form). Customer authorizes dimmi and Paystack to charge Customer's
              designated payment method for accrued usage charges.
            </>,
            <>
              <strong className="font-medium text-text-primary">Taxes and withholding:</strong> All stated fees exclude
              value-added tax (including Nigerian VAT at 7.5% where applicable under the Nigeria Tax Act 2025), sales
              tax, GST, and similar transaction taxes, which Customer is responsible for paying (excluding taxes on
              dimmi's net income). If Customer is legally required to deduct or withhold tax from any payment, Customer
              will gross up the payment so dimmi receives the full invoiced net amount and provide official withholding
              tax receipts promptly.
            </>,
            <>
              <strong className="font-medium text-text-primary">Late payment and billing disputes:</strong> Undisputed
              overdue amounts accrue interest at 1.5% per month (or the maximum rate permitted by law, if lower), and
              dimmi may suspend paid features after 15 days' written notice of non-payment. Customer must raise any
              good-faith billing dispute in writing to{" "}
              <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> within 60 days
              of the invoice or charge date, or the charge is deemed accepted.
            </>,
            <>
              <strong className="font-medium text-text-primary">Refunds and price changes:</strong> Except where
              expressly stated in this Agreement (such as pro-rata refunds upon objection to a materially adverse terms
              change or subprocessor addition, or warranty termination), fees paid are non-refundable. We will provide
              at least 60 days' advance email and in-app notice before any published price increase takes effect.
            </>,
          ]}
        />
      </Clause>

      <Clause
        anchor="ip"
        number="13"
        title="Intellectual property"
        summary="We own the dimmi Cloud platform and brand. You own your Customer Data and AI Output. If you send us product feedback, we can use it without owning it."
      >
        <p>
          As between the parties, {LEGAL_ENTITY.name} owns and retains all right, title, and interest in and to dimmi
          Cloud, our Documentation, trademarks, logos, and proprietary cloud infrastructure. Nothing in this Agreement
          transfers any intellectual property ownership in dimmi Cloud to Customer.
        </p>
        <p>
          Customer owns all right, title, and interest in and to Customer Data and, to the fullest extent permitted by
          applicable law, all AI Output generated specifically for Customer. Customer acknowledges that due to the
          nature of machine learning, similar outputs may be generated independently for other users from different
          inputs.
        </p>
        <p>
          If Customer or any Team Member voluntarily submits suggestions, bug reports, or enhancement ideas regarding
          dimmi Cloud (<strong className="font-medium text-text-primary">"Feedback"</strong>), Customer grants dimmi a
          non-exclusive, perpetual, irrevocable, royalty-free license to use and incorporate that Feedback into our
          products without obligation or attribution. We do not claim ownership of your Feedback. Open-source components
          and dimmi SDKs are licensed separately under their published open-source licenses (including MIT).
        </p>
      </Clause>

      <Clause
        anchor="publicity"
        number="14"
        title="Publicity"
        summary="We never use your name or logo without asking first. You can withdraw permission at any time."
      >
        <p>
          dimmi will not use Customer's business name or logo on our website, customer lists, or marketing materials
          without Customer's prior written consent (email is sufficient). Where Customer consents, dimmi will follow any
          brand guidelines Customer provides. Customer may withdraw consent at any time by emailing{" "}
          <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>, and we will remove
          Customer's name and logo within 10 business days.
        </p>
      </Clause>

      <Clause
        anchor="confidentiality"
        number="15"
        title="Confidentiality"
        summary="Both sides protect each other's non-public business and technical information for 3 years (and trade secrets indefinitely)."
      >
        <p>
          <strong className="font-medium text-text-primary">"Confidential Information"</strong> means non-public
          business, financial, technical, security, or pricing information disclosed by one party to the other that is
          marked confidential or would reasonably be understood to be confidential given its nature and circumstances.
          Customer Data is Customer's Confidential Information; non-public pricing, security reports, and roadmap
          details are dimmi's Confidential Information.
        </p>
        <p>
          The receiving party will: (a) use Confidential Information solely to exercise its rights and perform its
          obligations under this Agreement; (b) protect it with at least the same degree of care it uses for its own
          confidential information (and never less than reasonable care); and (c) disclose it only to personnel,
          affiliates, subprocessors, or professional advisors who need to know it and are bound by written
          confidentiality obligations at least as protective as this Clause.
        </p>
        <p>
          Confidentiality duties exclude information that is or becomes public without breach, was rightfully known
          before disclosure, is independently developed without use of Confidential Information, or is rightfully
          received from a third party without restriction. A party may disclose Confidential Information where compelled
          by law or court order, provided it gives prompt advance notice (where legally permitted) and discloses only
          the legally required minimum. Confidentiality obligations survive termination for 3 years, and indefinitely
          for trade secrets while they remain trade secrets under applicable law.
        </p>
      </Clause>

      <Clause
        anchor="warranties"
        number="16"
        title="Warranties and disclaimers"
        summary="We warrant dimmi Cloud works materially as documented and that we won't knowingly introduce malware. Everything else, including AI accuracy and third-party deliverability, is disclaimed."
      >
        <p>
          dimmi warrants to Paying Customers that during the Term: (a) dimmi Cloud will perform materially in accordance
          with the Documentation; and (b) dimmi will use industry-standard measures so that dimmi Cloud does not
          knowingly introduce viruses, Trojan horses, or malicious code into Customer's systems. If dimmi breaches this
          warranty, Customer's sole and exclusive remedy (and dimmi's sole obligation) is for dimmi to use commercially
          reasonable efforts to correct the non-conformity within 30 days of written notice, or if dimmi cannot do so,
          for Customer to terminate the affected service and receive a refund or credit of the Fees paid or payable for
          the affected period.
        </p>
        <p className="font-medium text-text-primary">
          TO THE MAXIMUM EXTENT PERMITTED BY APPLICABLE LAW (INCLUDING SECTIONS 127 TO 129 OF THE FCCPA), EXCEPT AS
          EXPRESSLY STATED IN THIS CLAUSE 16, DIMMI CLOUD, ALL AI FEATURES, AI OUTPUT, BETA FEATURES, AND DOCUMENTATION
          ARE PROVIDED "AS IS" AND "AS AVAILABLE." DIMMI DISCLAIMS ALL OTHER WARRANTIES, EXPRESS, IMPLIED, OR STATUTORY,
          INCLUDING IMPLIED WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, TITLE, NON-INFRINGEMENT,
          UNINTERRUPTED OPERATION, AI OUTPUT ACCURACY OR COMPLETENESS, EMAIL OR MESSAGE DELIVERABILITY, AND THE
          AVAILABILITY OF THIRD-PARTY PLATFORMS.
        </p>
      </Clause>

      <Clause
        anchor="indemnities"
        number="17"
        title="Indemnities"
        summary="For paying customers, we defend third-party IP infringement claims against dimmi Cloud itself. You defend us against claims caused by your data, missing consents, or AUP breaches."
      >
        <p>
          <strong className="font-medium text-text-primary">17.1 By dimmi:</strong> For Paying Customers (excluding Beta
          Features), dimmi will defend Customer against any unaffiliated third-party legal claim alleging that
          Customer's authorized use of dimmi Cloud directly infringes a third party's valid patent, copyright, or
          trademark, and will pay damages finally awarded by a court of competent jurisdiction or agreed in a
          dimmi-approved settlement. If such a claim arises or appears likely, dimmi may procure the right to continue
          using dimmi Cloud, modify it to be non-infringing, or terminate the affected service with a refund or credit
          of the Fees paid or payable for the affected period. dimmi has no obligation under this Clause 17.1 for claims
          arising from: (a) Customer Data, knowledge imports, or AI Output; (b) Third-Party Platforms or BYO AI
          providers; (c) unauthorized modifications or combinations with non-dimmi software; or (d) use in breach of
          this Agreement.
        </p>
        <p>
          <strong className="font-medium text-text-primary">17.2 By Customer:</strong> Customer will defend dimmi, its
          affiliates, officers, and personnel against any unaffiliated third-party claim, regulatory fine, or proceeding
          arising out of or relating to: (a) Customer Data or Customer's knowledge imports; (b) Customer's failure to
          provide required notices or obtain valid consent from End Customers (including under anti-spam laws, Meta or
          Slack rules, chat recording or wiretap statutes such as California Penal Code Section 631, NDPA, GAID, or
          GDPR); or (c) Customer's breach of the Acceptable Use Policy or Product-Specific Terms, and will pay resulting
          finally awarded damages, statutory penalties, and approved settlement amounts.
        </p>
        <p>
          <strong className="font-medium text-text-primary">17.3 Procedure:</strong> The indemnified party must promptly
          notify the indemnifying party in writing of the claim, grant sole control of defense and settlement (provided
          no settlement admits fault or imposes non-monetary obligations on the indemnified party without consent), and
          provide reasonable cooperation at the indemnifying party's expense.
        </p>
      </Clause>

      <Clause
        anchor="liability"
        number="18"
        title="Limitation of liability"
        summary="Neither party pays indirect damages. General liability is capped at 12 months' fees (US$500 floor), with a 2x super-cap for data protection and confidentiality. Fraud, willful misconduct, and gross negligence are uncapped. An Order Form can set higher caps."
      >
        <p className="font-medium text-text-primary">
          18.1 EXCLUSION OF INDIRECT DAMAGES: TO THE MAXIMUM EXTENT PERMITTED BY APPLICABLE LAW, NEITHER PARTY WILL BE
          LIABLE TO THE OTHER FOR ANY INDIRECT, INCIDENTAL, SPECIAL, CONSEQUENTIAL, EXEMPLARY, OR PUNITIVE DAMAGES, OR
          FOR ANY LOSS OF PROFITS, REVENUE, GOODWILL, BUSINESS OPPORTUNITY, OR DATA (OTHER THAN DIRECT RECOVERY COSTS
          SUBJECT TO CLAUSE 18.2), ARISING OUT OF OR RELATED TO THIS AGREEMENT, EVEN IF ADVISED OF THE POSSIBILITY OF
          SUCH DAMAGES.
        </p>
        <p className="font-medium text-text-primary">
          18.2 GENERAL LIABILITY CAP: EXCEPT FOR THE SUPER-CAP IN CLAUSE 18.3 AND THE CARVE-OUTS IN CLAUSE 18.4, EACH
          PARTY'S TOTAL AGGREGATE LIABILITY ARISING OUT OF OR RELATED TO THIS AGREEMENT (WHETHER IN CONTRACT, TORT, OR
          OTHERWISE) WILL NOT EXCEED THE GREATER OF: (A) THE TOTAL FEES PAID OR PAYABLE BY CUSTOMER TO DIMMI UNDER THIS
          AGREEMENT DURING THE 12 MONTHS IMMEDIATELY PRECEDING THE FIRST EVENT GIVING RISE TO LIABILITY; OR (B) US$500.
        </p>
        <p className="font-medium text-text-primary">
          18.3 DATA PROTECTION, CONFIDENTIALITY AND INDEMNITY SUPER-CAP: FOR CLAIMS ARISING FROM A PARTY'S BREACH OF ITS
          CONFIDENTIALITY OBLIGATIONS (CLAUSE 15), ITS DATA PROTECTION AND SECURITY OBLIGATIONS UNDER THE DPA, OR ITS
          INDEMNIFICATION OBLIGATIONS (CLAUSE 17), EACH PARTY'S AGGREGATE LIABILITY WILL NOT EXCEED THE GREATER OF TWO
          TIMES (2X) THE FEES PAID OR PAYABLE BY CUSTOMER DURING THE 12 MONTHS IMMEDIATELY PRECEDING THE FIRST EVENT
          GIVING RISE TO LIABILITY, OR US$1,000. THIS CAP APPLIES IN PLACE OF, NOT IN ADDITION TO, THE CAP IN CLAUSE
          18.2, AND BOTH CAPS APPLY TO ALL CLAIMS IN AGGREGATE.
        </p>
        <p>
          <strong className="font-medium text-text-primary">18.4 Uncapped carve-outs:</strong> Nothing in this Agreement
          limits or excludes either party's liability for: (a) fraud or fraudulent misrepresentation; (b) willful
          misconduct or gross negligence (including as preserved under Section 129 of the FCCPA); (c) death or personal
          injury caused by negligence; (d) Customer's payment obligations under Clause 12; or (e) any liability that
          cannot lawfully be limited or excluded under applicable law (including statutory data-subject rights under the
          NDPA or EU Standard Contractual Clauses).
        </p>
        <p>
          <strong className="font-medium text-text-primary">18.5 Higher caps by Order Form:</strong> The caps in Clauses
          18.2 and 18.3 are the defaults for self-serve use. dimmi and Customer may agree higher caps, including fixed
          amounts, in an executed Order Form, and those agreed caps then apply in place of the defaults.
        </p>
      </Clause>

      <Clause
        anchor="term"
        number="19"
        title="Term, suspension, and termination"
        summary="Runs month to month until you cancel or an Order Form ends. Either party can terminate for uncured material breach after 30 days. We may suspend immediately for emergencies, abuse, or vendor demands."
      >
        <p>
          <strong className="font-medium text-text-primary">19.1 Term:</strong> This Agreement begins when Customer
          first accepts it and continues until all Workspaces and Order Forms are terminated. Self-serve Customers may
          cancel or delete their Workspace at any time in Workspace settings, effective immediately upon completion of
          the current billing month for accrued usage.
        </p>
        <p>
          <strong className="font-medium text-text-primary">19.2 Termination for cause:</strong> Either party may
          terminate this Agreement or an affected Order Form upon written notice if the other party commits a material
          breach and fails to cure it within 30 days after receiving written notice specifying the breach, or if the
          other party becomes insolvent, enters liquidation, or ceases business operations.
        </p>
        <p>
          <strong className="font-medium text-text-primary">19.3 Suspension:</strong> dimmi may suspend access to all or
          part of dimmi Cloud: (a) on 15 days' written notice for undisputed overdue fees; (b) automatically for bulk
          email sending (broadcasts, series, and rating emails) if Customer's Workspace exceeds the published bounce or
          complaint thresholds set out in <LegalLink href="#pt-email">Section II.2</LegalLink> (5% hard bounces or 0.1%
          spam complaints over 7 days once 100 emails are sent, or 0.3% complaints on an individual broadcast), while
          teammate and agent replies continue; or (c) immediately, with prompt notice where lawful, if necessary to stop
          an active security threat, illegal content, material AUP breach, or a mandatory takedown or cutoff demand from
          an upstream provider such as Meta or AWS. We will narrow any suspension to the affected Channel or feature
          where practical and restore access promptly once the issue is resolved.
        </p>
      </Clause>

      <Clause
        anchor="export"
        number="20"
        title="Export and deletion"
        summary="You can export your data during the term and for 30 days after termination. Live data is then permanently deleted within 30 days, and backups roll off within 30 days."
      >
        <p>
          Customer may export Customer Data at any time during the Term using dimmi's built-in export tools and REST
          API, and for 30 days following termination or Workspace deletion: during that time the Workspace owner can
          restore the Workspace and export from it. After that 30-day period, dimmi permanently purges Customer Data
          from live systems (including database rows, raw email files, help center images, and knowledge uploads) within
          30 days, and database backups roll off automatically on their 30-day retention cycle, matching section VII.5
          of our <LegalLink href="/privacy#dpa-deletion">DPA</LegalLink>.
        </p>
      </Clause>

      <Clause
        anchor="switching"
        number="21"
        title="Provider switching (EU Data Act applied globally)"
        summary="Every customer gets clear switching rights: at most 2 months' notice, a 30-day transition window, full data retrieval in open formats, and $0 switching fees."
      >
        <p>
          To support data portability and comply with Chapter VI of the European Union Data Act (Regulation (EU)
          2023/2854), dimmi extends the following switching rights to all Customers globally, not only those established
          in the EU:
        </p>
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Notice and transition period:</strong> Customer may
              initiate switching to another provider or to a self-hosted dimmi instance by giving notice (never
              exceeding 2 months, and immediate for self-serve monthly Customers), followed by a 30-day transition
              period during which dimmi Cloud remains available to complete migration.
            </>,
            <>
              <strong className="font-medium text-text-primary">Exportable data and open formats:</strong> Customer can
              export conversations, messages, contacts, companies, tickets, help center articles, knowledge files, and
              configuration via our REST API and export tools in structured, machine-readable formats (JSON, CSV, and
              Markdown), plus deploy the exact same MIT-licensed dimmi codebase on its own infrastructure.
            </>,
            <>
              <strong className="font-medium text-text-primary">Retrieval window and zero switching charges:</strong>{" "}
              Customer has at least 30 days after transition ends to retrieve exported data. dimmi charges US$0 in
              switching or data egress fees. (Fixed-term Order Form commitments remain subject to agreed early
              termination terms where lawful.)
            </>,
          ]}
        />
      </Clause>

      <Clause
        anchor="compliance"
        number="22"
        title="Compliance, sanctions, and lawful requests"
        summary="You confirm you are not on a sanctions list or in an embargoed country, and you acknowledge we must comply with lawful government or law enforcement orders."
      >
        <p>
          Each party will comply with all anti-bribery, anti-corruption, export control, and economic sanctions laws
          applicable to its performance under this Agreement, including sanctions administered by Nigeria, the United
          Nations, the European Union, the United Kingdom, and the United States (OFAC). Customer represents that
          neither Customer nor its controlling owners are designated on any restricted-party list and that Customer will
          not use dimmi Cloud in or for the benefit of any comprehensively embargoed jurisdiction.
        </p>
        <p>
          Customer acknowledges that {LEGAL_ENTITY.name} is subject to Nigerian law, including lawful disclosure,
          preservation, and cyber-incident reporting obligations under the Cybercrimes (Prohibition, Prevention, etc.)
          Act 2015 (as amended 2024). Where dimmi receives a legally binding government or law enforcement request for
          Customer Data, dimmi handles it under{" "}
          <LegalLink href="/privacy#dpa-deletion">Section VII.5 of the DPA</LegalLink> (notifying Customer unless
          prohibited by law, challenging overbroad requests, and disclosing only the required minimum).
        </p>
      </Clause>

      <Clause
        anchor="governing-law"
        number="23"
        title="Governing law and dispute resolution"
        summary="Nigerian law governs. We negotiate in good faith for 30 days first; unresolved disputes go to binding arbitration in Lagos under Nigeria's Arbitration and Mediation Act 2023. An Order Form can name a different law and seat."
      >
        <p>
          This Agreement and any dispute or claim arising out of or in connection with it (including non-contractual
          disputes) are governed by and construed in accordance with the laws of the{" "}
          <strong className="font-medium text-text-primary">Federal Republic of Nigeria</strong>, without regard to
          conflict-of-laws principles.
        </p>
        <p>
          Before initiating formal proceedings, either party must deliver written notice of the dispute to the other
          party (for dimmi, to <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink>
          ), and senior representatives of both parties will negotiate in good faith for 30 days to resolve it.
        </p>
        <p>
          If the dispute is not resolved within 30 days, it will be referred to and finally resolved by binding
          arbitration seated in <strong className="font-medium text-text-primary">Lagos, Nigeria</strong>, conducted in
          English by a single arbitrator under the{" "}
          <strong className="font-medium text-text-primary">Arbitration and Mediation Act 2023</strong> of Nigeria (or
          the Lagos Court of Arbitration rules where agreed by the parties). Nothing in this Clause prevents either
          party from seeking urgent interim or injunctive relief from a court of competent jurisdiction to protect its
          intellectual property or Confidential Information, nor does it restrict any non-waivable statutory right of a
          data subject to bring claims before a Federal or State High Court under Article 47 of the GAID 2025 or before
          an EU/UK supervisory authority or court under the Standard Contractual Clauses.
        </p>
        <p>
          dimmi and Customer may agree a different governing law, arbitration seat, or arbitration rules in an executed
          Order Form (for example, the laws of England and Wales with arbitration seated in London under the LCIA
          Rules). Where they do, the Order Form governs in place of the first and third paragraphs of this Clause.
        </p>
      </Clause>

      <Clause
        anchor="general"
        number="24"
        title="General provisions and version history"
        summary="We can assign this contract to a successor or affiliate on notice. We give 30 days' notice for material terms changes, and you can cancel with a pro-rata refund if you object."
      >
        <LegalList
          items={[
            <>
              <strong className="font-medium text-text-primary">Assignment:</strong> Neither party may assign this
              Agreement without the other party's prior written consent, except that: (a) {LEGAL_ENTITY.name} may assign
              or novate this Agreement in whole or in part to an affiliate, newly formed operating subsidiary, or
              successor in connection with a corporate reorganization, redomiciliation, merger, or sale of assets upon
              written notice to Customer and without requiring Customer's consent; and (b) Customer may assign this
              Agreement in connection with a merger, acquisition, or sale of substantially all of its assets upon prompt
              written notice to dimmi, provided the assignee is not a direct competitor of dimmi and agrees in writing
              to be bound by this Agreement.
            </>,
            <>
              <strong className="font-medium text-text-primary">Changes to these Terms:</strong> We may update these
              Terms, the Product-Specific Terms, or the AUP from time to time. For material changes, we will provide at
              least 30 days' advance notice by email to Workspace owners and in-app before the new version takes effect
              (and 60 days for price increases). If a change is materially adverse to Customer and not required by law
              or a Third-Party Platform, Customer may terminate the affected service by written notice before the
              effective date and receive a pro-rata refund of any prepaid, unused fees. Continued use after the
              effective date constitutes acceptance.
            </>,
            <>
              <strong className="font-medium text-text-primary">Notices:</strong> Legal notices to dimmi must be sent by
              email to <LegalLink href={`mailto:${LEGAL_CONTACTS.support}`}>{LEGAL_CONTACTS.support}</LegalLink> with a
              copy (for formal dispute notices) to {LEGAL_ENTITY.name}, {LEGAL_ENTITY.address}. Legal notices to
              Customer will be sent to the Workspace owner's email address on file or posted in-app.
            </>,
            <>
              <strong className="font-medium text-text-primary">Force majeure:</strong> Neither party is liable for
              delay or failure to perform (other than payment obligations) caused by events beyond its reasonable
              control, including natural disasters, war, civil unrest, power or internet backbone failures, or
              government actions.
            </>,
            <>
              <strong className="font-medium text-text-primary">
                Subcontracting, severability &amp; entire agreement:
              </strong>{" "}
              dimmi may use subprocessors in accordance with the DPA and remains responsible for their performance. If
              any provision is held unenforceable, the remainder stays in full force. Failure to enforce a right is not
              a waiver. This Agreement is the entire agreement between the parties and is governed in English. Clauses
              9, 12 (accrued fees), 13, 15, 16, 17, 18, 20, 22, 23, and 24 survive termination.
            </>,
          ]}
        />
        <LegalTable
          caption="Terms of Service version archive"
          headers={["Version", "Effective", "Summary"]}
          rows={[
            [
              `v${LEGAL_VERSION.version}`,
              LEGAL_VERSION.effectiveDate,
              "Initial release of dimmi Cloud Terms of Service, Product-Specific Terms, Acceptable Use Policy, and Self-Hosting Notice.",
            ],
          ]}
        />
      </Clause>

      <TermsProductSchedule />
      <TermsAupAndSelfHosting />
      <LegalEntityFooter />
    </LegalDocument>
  );
}
