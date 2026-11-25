import { doc, ICONS, renderRail, type SceneSpec, type Scheme } from "./theme.ts";

function helpCenterHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; display: flex; flex-direction: column; background: var(--bg);">
      <header style="height: 62px; padding: 0 72px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between;">
        <div style="display: flex; align-items: center; gap: 10px;">
          <div style="width: 28px; height: 28px; border-radius: 8px; background: var(--accent); color: #fff; font-weight: 700; font-size: 14px; display: flex; align-items: center; justify-content: center;">S</div>
          <span style="font-weight: 600; font-size: 15px;">Stowline</span>
          <span style="color: var(--fg-3); font-size: 14px;">/ Help Center</span>
        </div>
        <div style="display: flex; align-items: center; gap: 24px; font-size: 13.5px; color: var(--fg-2);">
          <span style="color: var(--fg); font-weight: 500;">Guides</span>
          <span>Changelog</span>
          <span>Feature requests</span>
          <span class="btn btn-sm">Sign in</span>
        </div>
      </header>
      <div style="padding: 58px 72px 44px; background: var(--bg-sub); border-bottom: 1px solid var(--border); display: flex; flex-direction: column; align-items: center; text-align: center; gap: 18px;">
        <span class="pill pill-accent">${ICONS.sparkle} Ask AI or search 84 articles</span>
        <h1 style="font-size: 38px; font-weight: 600; letter-spacing: -0.03em;">How can we help with your shipments?</h1>
        <div style="width: 640px; height: 54px; border-radius: 14px; background: var(--bg); border: 1px solid var(--border-strong); box-shadow: var(--shadow-panel); display: flex; align-items: center; justify-content: space-between; padding: 0 10px 0 18px;">
          <div style="display: flex; align-items: center; gap: 10px; color: var(--fg-2); font-size: 14.5px;">
            ${ICONS.search}
            <span>How do I dispute a USPS weight adjustment?</span>
          </div>
          <span class="btn btn-primary btn-sm">Ask Sky</span>
        </div>
      </div>
      <div style="padding: 40px 72px; display: grid; grid-template-columns: repeat(3, 1fr); gap: 18px;">
        ${[
          [
            "Getting started",
            "Connect Shopify, print your first batch of labels, and set pickup rules.",
            "14 articles",
          ],
          ["Carriers & rates", "Compare USPS, UPS, FedEx, and DHL rates, dimensions, and surcharges.", "22 articles"],
          [
            "Returns & refunds",
            "Void unused labels, handle carrier weight disputes, and automate return portals.",
            "19 articles",
          ],
          [
            "Customs & duties",
            "Commercial invoices, HS tariff codes, and DDP shipping to Europe and UK.",
            "11 articles",
          ],
          ["Webhooks & API", "Subscribe to tracking updates, print ZPL labels, and sync order status.", "10 articles"],
          ["Account & billing", "Invoices, carrier balance auto-top-up, and team permissions.", "8 articles"],
        ]
          .map(
            ([
              title,
              desc,
              count,
            ]) => `<div class="card" style="padding: 20px; display: flex; flex-direction: column; gap: 8px; box-shadow: var(--shadow-card);">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span style="font-size: 16px; font-weight: 600; letter-spacing: -0.01em;">${title}</span>
              <span class="pill pill-muted">${count}</span>
            </div>
            <p style="font-size: 13.5px; line-height: 1.5; color: var(--fg-2);">${desc}</p>
          </div>`,
          )
          .join("")}
      </div>
    </div>`,
  );
}

function routinesListHtml(scheme: Scheme): string {
  const rows = [
    {
      name: "Pull requests to docs",
      desc: "Check merged PRs in stowline/app against live help articles and draft inline updates.",
      cadence: "Weekdays · 08:30",
      status: "2 pending edits",
      statusClass: "pill-accent",
      lastRun: "Today, 08:30 · 4 PRs scanned",
      cost: "$0.14",
      delivery: "#docs-updates",
      enabled: true,
    },
    {
      name: "Knowledge gap finder",
      desc: "Scan handed-over chats for recurring unanswered questions and draft new articles.",
      cadence: "Mondays · 09:00",
      status: "1 draft article",
      statusClass: "pill-blue",
      lastRun: "Mon, 09:00 · 38 threads",
      cost: "$0.29",
      delivery: "#support-ops",
      enabled: true,
    },
    {
      name: "Conflict and staleness audit",
      desc: "Review help center pages for conflicting carrier limits, old pricing, or stale UI paths.",
      cadence: "1st of month · 07:00",
      status: "Quiet · No issues",
      statusClass: "pill-muted",
      lastRun: "Nov 1 · 84 articles",
      cost: "$0.41",
      delivery: "Quiet when clean",
      enabled: true,
    },
    {
      name: "Weekly QA and procedure tune-up",
      desc: "Digest top handover causes and propose edits to procedures that caused escalations.",
      cadence: "Fridays · 15:00",
      status: "1 procedure diff",
      statusClass: "pill-purple",
      lastRun: "Fri, 15:00 · 142 threads",
      cost: "$0.33",
      delivery: "#support-leads",
      enabled: true,
    },
  ];
  return doc(
    scheme,
    `<div class="app-shell">
      ${renderRail("routines")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header" style="padding: 0 28px; height: 64px;">
          <div style="display: flex; align-items: center; gap: 12px;">
            <span style="font-size: 16px; font-weight: 600; letter-spacing: -0.015em;">Operator Routines</span>
            <span class="pill pill-accent">4 active</span>
            <span class="pill pill-muted">3 changesets waiting review</span>
          </div>
          <div style="display: flex; align-items: center; gap: 10px;">
            <span class="btn btn-sm">Run history</span>
            <span class="btn btn-primary btn-sm">+ New routine</span>
          </div>
        </div>
        <div style="padding: 28px 36px; display: flex; flex-direction: column; gap: 22px; overflow: hidden;">
          <div style="display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px;">
            ${[
              ["Runs this month", "34", "22 quiet · 12 proposed edits"],
              ["Edits applied", "19", "91% approval rate"],
              ["Pending review", "3", "2 help articles · 1 procedure"],
              ["Monthly spend", "$6.42", "Cap: $50.00 / month"],
            ]
              .map(
                ([label, val, sub]) => `<div class="card-sub" style="padding: 16px 18px;">
                <div style="font-size: 12px; color: var(--fg-3);">${label}</div>
                <div style="font-size: 26px; font-weight: 600; letter-spacing: -0.02em; margin-top: 4px;">${val}</div>
                <div style="font-size: 12px; color: var(--fg-2); margin-top: 4px;">${sub}</div>
              </div>`,
              )
              .join("")}
          </div>
          <div class="card" style="padding: 0; overflow: hidden;">
            <div style="display: grid; grid-template-columns: 2.3fr 1fr 1.1fr 1.15fr 0.7fr 0.45fr; padding: 11px 20px; border-bottom: 1px solid var(--border); background: var(--bg-sub); font-size: 11.5px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">
              <span>Routine</span>
              <span>Schedule</span>
              <span>Pending</span>
              <span>Last run</span>
              <span>Delivery</span>
              <span style="text-align: right;">Active</span>
            </div>
            ${rows
              .map(
                (
                  r,
                  idx,
                ) => `<div style="display: grid; grid-template-columns: 2.3fr 1fr 1.1fr 1.15fr 0.7fr 0.45fr; align-items: center; padding: 18px 20px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""}">
                <div style="padding-right: 18px;">
                  <div style="font-size: 14.5px; font-weight: 600; display: flex; align-items: center; gap: 8px;">${r.name}</div>
                  <div style="font-size: 12.5px; color: var(--fg-2); margin-top: 3px; line-height: 1.4;">${r.desc}</div>
                </div>
                <div style="font-size: 13px; color: var(--fg); font-weight: 500;">${r.cadence}</div>
                <div><span class="pill ${r.statusClass}">${r.status}</span></div>
                <div>
                  <div style="font-size: 12.5px; color: var(--fg);">${r.lastRun}</div>
                  <div class="mono" style="font-size: 11px; color: var(--fg-3); margin-top: 2px;">Cost ${r.cost}</div>
                </div>
                <div class="mono" style="font-size: 12px; color: var(--fg-2);">${r.delivery}</div>
                <div style="display: flex; justify-content: flex-end;"><span class="switch ${r.enabled ? "on" : ""}"></span></div>
              </div>`,
              )
              .join("")}
          </div>
          <div style="display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border);">
            <div style="display: flex; align-items: center; gap: 10px; font-size: 13px; color: var(--accent-fg);">
              ${ICONS.shield}
              <span><strong>Human approval required:</strong> Routines only propose reviewable changesets. Declined edits are remembered and never re-proposed.</span>
            </div>
            <span class="btn btn-sm">View audit log</span>
          </div>
        </div>
      </main>
    </div>`,
  );
}

function routinePrDiffHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 16px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 14px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="pill pill-accent">${ICONS.gitPr} PR #418 merged</span>
          <span style="font-size: 13px; font-weight: 600;">Voiding USPS labels</span>
        </div>
        <span style="font-size: 12px; color: var(--fg-3);">08:30 AM · Pull requests to docs</span>
      </div>
      <div class="card-sub" style="padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 12px;">
        <div style="font-size: 12.5px; color: var(--fg-2); line-height: 1.45;">
          <strong style="color: var(--fg);">Why Operator proposed this:</strong> PR #418 raised the automatic USPS void window from 5 days to 14 days and added bulk void in <span class="mono" style="font-size: 11.5px;">Shipments → Labels</span>.
        </div>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 10px 14px; background: var(--bg-sub); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; font-size: 12px;">
          <span class="mono" style="color: var(--fg-2);">help-center / returns-and-refunds / void-usps-label</span>
          <span class="pill pill-amber">Pending review</span>
        </div>
        <div style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; font-size: 13.5px; line-height: 1.55;">
          <div style="color: var(--fg-2);">Unused USPS labels can be voided directly from your shipment details page before the first carrier scan.</div>
          <div style="padding: 8px 12px; border-radius: 8px; background: var(--red-soft); border-left: 3px solid #ef4444; color: var(--red-fg);">
            <span class="mono" style="font-weight: 700; margin-right: 6px;">-</span>Labels must be voided within <strong>5 days</strong> of purchase. Each label must be voided individually.
          </div>
          <div style="padding: 8px 12px; border-radius: 8px; background: var(--accent-soft); border-left: 3px solid #10b981; color: var(--accent-fg);">
            <span class="mono" style="font-weight: 700; margin-right: 6px;">+</span>Labels can be voided within <strong>14 days</strong> of purchase. To void multiple labels at once, open <strong>Shipments → Labels</strong>, select the rows, and click <strong>Bulk void</strong>.
          </div>
          <div style="color: var(--fg-2);">Refunds return to your Stowline balance once USPS confirms the label was never scanned (14 to 21 days).</div>
        </div>
        <div style="padding: 12px 16px; border-top: 1px solid var(--border); background: var(--bg-sub); display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 12px; color: var(--fg-3);">Undo stays available after applying</span>
          <div style="display: flex; gap: 8px;">
            <span class="btn btn-sm">Decline</span>
            <span class="btn btn-primary btn-sm">${ICONS.check} Apply edit</span>
          </div>
        </div>
      </div>
    </div>`,
  );
}

function routineProcedureDiffHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 16px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 14px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="pill pill-purple">${ICONS.sparkle} Procedure update</span>
          <span style="font-size: 13px; font-weight: 600;">Carrier weight dispute</span>
        </div>
        <span style="font-size: 12px; color: var(--fg-3);">Weekly QA · 6 handovers</span>
      </div>
      <div class="card-sub" style="padding: 12px 14px; font-size: 12.5px; color: var(--fg-2); line-height: 1.45;">
        <strong style="color: var(--fg);">Pattern spotted:</strong> 6 customers sent scale photos without a tape measure, causing Sky to escalate instead of asking for one follow-up photo.
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 10px 14px; background: var(--bg-sub); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; font-size: 12px;">
          <span style="font-weight: 600;">Step 2 · Verify required evidence</span>
          <span class="pill pill-muted">Sky · Support agent</span>
        </div>
        <div style="padding: 14px 16px; display: flex; flex-direction: column; gap: 9px; font-size: 13.5px; line-height: 1.5;">
          <div style="color: var(--fg-2);">1. Run <span class="pill pill-lime" style="padding: 0 7px; font-size: 12px;">⚡ Look up a parcel</span> to confirm the carrier adjustment charge.</div>
          <div style="padding: 8px 12px; border-radius: 8px; background: var(--red-soft); border-left: 3px solid #ef4444; color: var(--red-fg);">
            <span class="mono" style="font-weight: 700; margin-right: 6px;">-</span>2. If the customer's photo is missing dimensions, hand over to <span class="pill pill-orange" style="padding: 0 6px; font-size: 11.5px;">Billing</span> immediately.
          </div>
          <div style="padding: 8px 12px; border-radius: 8px; background: var(--accent-soft); border-left: 3px solid #10b981; color: var(--accent-fg);">
            <span class="mono" style="font-weight: 700; margin-right: 6px;">+</span>2. Ask once for a single photo showing the sealed box on a scale beside a tape measure. Only escalate to <span class="pill pill-orange" style="padding: 0 6px; font-size: 11.5px;">Billing</span> if the customer already discarded the packaging.
          </div>
          <div style="color: var(--fg-2);">3. Explain the 30-day carrier review timeline without promising an automatic refund.</div>
        </div>
        <div style="padding: 12px 16px; border-top: 1px solid var(--border); background: var(--bg-sub); display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 12px; color: var(--fg-3);">Runs against 12 simulations before publishing</span>
          <div style="display: flex; gap: 8px;">
            <span class="btn btn-sm">Decline</span>
            <span class="btn btn-primary btn-sm">${ICONS.check} Apply to draft</span>
          </div>
        </div>
      </div>
    </div>`,
  );
}

function routineThreadHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 22px 24px; display: flex; flex-direction: column; gap: 14px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="pill pill-accent">${ICONS.sparkle} Operator</span>
          <span style="font-size: 13.5px; font-weight: 600;">Pull requests to docs · Nov 24</span>
        </div>
        <span class="pill pill-muted">Shared thread · 2 teammates</span>
      </div>
      <div style="display: flex; gap: 10px; align-items: flex-start;">
        <span class="avatar av-emerald">${ICONS.sparkle}</span>
        <div style="flex: 1; display: flex; flex-direction: column; gap: 10px;">
          <div style="font-size: 12px; color: var(--fg-3);"><strong style="color: var(--fg);">Operator</strong> · 8:30 AM · Skipped 5 internal refactors &amp; tests</div>
          <div style="font-size: 13.5px; line-height: 1.5; color: var(--fg);">Found 1 customer-facing change in <span class="mono" style="font-size: 12px;">PR #418</span> and drafted an inline edit for <strong>Voiding USPS labels</strong>.</div>
          <div class="card-sub" style="padding: 12px 14px; display: flex; align-items: center; justify-content: space-between; gap: 10px;">
            <div>
              <div style="font-size: 13px; font-weight: 600;">Update USPS void window (5d → 14d) + bulk void</div>
              <div style="font-size: 12px; color: var(--fg-3); margin-top: 2px;">Help Center · 1 article · +2 lines, -1 line</div>
            </div>
            <div style="display: flex; gap: 6px;">
              <span class="btn btn-sm">Review</span>
              <span class="btn btn-primary btn-sm">Apply</span>
            </div>
          </div>
        </div>
      </div>
      <div style="display: flex; gap: 10px; align-items: flex-start; padding-top: 4px;">
        <span class="avatar av-rose">P</span>
        <div style="flex: 1; display: flex; flex-direction: column; gap: 6px;">
          <div style="font-size: 12px; color: var(--fg-3);"><strong style="color: var(--fg);">Priya Moreau</strong> · 8:42 AM</div>
          <div style="padding: 10px 13px; border-radius: 11px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 13px; line-height: 1.45;">
            Also mention that bulk void is capped at 50 labels per batch, then update the draft.
          </div>
        </div>
      </div>
      <div style="display: flex; gap: 10px; align-items: flex-start;">
        <span class="avatar av-emerald">${ICONS.sparkle}</span>
        <div style="flex: 1; display: flex; flex-direction: column; gap: 6px;">
          <div style="font-size: 12px; color: var(--fg-3);"><strong style="color: var(--fg);">Operator</strong> · 8:42 AM</div>
          <div style="font-size: 13px; line-height: 1.45; color: var(--fg-2);">Updated the pending changeset to note the 50-label batch limit. Ready to apply whenever you are.</div>
        </div>
      </div>
      <div style="padding: 10px 14px; border-radius: 10px; border: 1px solid var(--border); background: var(--bg-sub); font-size: 12.5px; color: var(--fg-3); display: flex; justify-content: space-between; align-items: center;">
        <span>Reply to Operator in this shared thread...</span>
        <span class="pill pill-muted">⌘↵</span>
      </div>
    </div>`,
  );
}

function routineGuardrailsHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 16px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Routine guardrails</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Pull requests to docs</div>
        </div>
        <span class="btn btn-sm">Dry run now</span>
      </div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
        <div class="card-sub" style="padding: 12px 14px;">
          <div style="font-size: 11.5px; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">Cadence</div>
          <div style="font-size: 13.5px; font-weight: 600; margin-top: 4px;">Weekdays at 08:30</div>
          <div style="font-size: 12px; color: var(--fg-2); margin-top: 2px;">America/New_York</div>
        </div>
        <div class="card-sub" style="padding: 12px 14px;">
          <div style="font-size: 11.5px; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">Monthly budget cap</div>
          <div style="font-size: 13.5px; font-weight: 600; margin-top: 4px;">$25.00 / month</div>
          <div style="font-size: 12px; color: var(--accent-fg); margin-top: 2px;">$2.18 spent so far</div>
        </div>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        ${[
          ["Propose changesets only", "Never publishes articles or edits live procedures without a click", true],
          ["Quiet when nothing changed", "Skips Slack and email notifications on clean runs", true],
          ["Remember declined edits", "Suppresses rejected suggestions and expires stale drafts after 14d", true],
          ["Auto-pause on repeated errors", "Pauses after 3 consecutive failures or if the owner leaves", true],
        ]
          .map(
            (
              [title, sub, on],
              idx,
            ) => `<div style="padding: 13px 16px; display: flex; align-items: center; justify-content: space-between; gap: 14px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""}">
            <div>
              <div style="font-size: 13.5px; font-weight: 500;">${title}</div>
              <div style="font-size: 12px; color: var(--fg-3); margin-top: 2px;">${sub}</div>
            </div>
            <span class="switch ${on ? "on" : ""}"></span>
          </div>`,
          )
          .join("")}
      </div>
      <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
        <span>Scoped tools: <strong style="color: var(--fg-2);">GitHub (read) · Help Center (propose)</strong></span>
        <span class="pill pill-accent">Owner: Maya Lin</span>
      </div>
    </div>`,
  );
}

export const CORE_SCENES: SceneSpec[] = [
  { name: "help-center", width: 1680, height: 1000, kind: "still", html: helpCenterHtml },
  { name: "routines-list", width: 1680, height: 1000, kind: "still", html: routinesListHtml },
  { name: "routine-pr-diff", width: 720, height: 640, kind: "still", html: routinePrDiffHtml },
  { name: "routine-procedure-diff", width: 720, height: 640, kind: "still", html: routineProcedureDiffHtml },
  { name: "routine-thread", width: 720, height: 640, kind: "still", html: routineThreadHtml },
  { name: "routine-guardrails", width: 720, height: 640, kind: "still", html: routineGuardrailsHtml },
];
