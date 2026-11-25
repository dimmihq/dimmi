import { doc, ICONS, renderRail, type SceneSpec, type Scheme, TIMELINE_SCRIPT_HELPER } from "./theme.ts";

function slackHeroHtml(scheme: Scheme): string {
  const isDark = scheme === "dark";
  const slBg = isDark ? "#1a1d21" : "#ffffff";
  const slSide = isDark ? "#19171d" : "#3f0e40";
  const slBorder = isDark ? "#2c2f34" : "#e2e2e2";
  const slText = isDark ? "#d1d2d3" : "#1d1c1d";
  const slSub = isDark ? "#9a9b9e" : "#616061";
  return doc(
    scheme,
    `<div class="app-shell" style="gap: 12px;">
      <!-- Left: Authentic Slack workspace + Slack Connect channel -->
      <div class="pane" style="width: 610px; flex-shrink: 0; flex-direction: row; background: ${slBg}; border-color: ${slBorder};">
        <div style="width: 56px; background: ${slSide}; display: flex; flex-direction: column; align-items: center; padding: 14px 0; gap: 10px; flex-shrink: 0;">
          <div style="width: 34px; height: 34px; border-radius: 9px; background: #ffffff; display: flex; align-items: center; justify-content: center;">${ICONS.slack}</div>
          <div style="width: 34px; height: 34px; border-radius: 9px; background: rgba(255,255,255,0.16); color: #fff; font-weight: 700; font-size: 14px; display: flex; align-items: center; justify-content: center;">N</div>
          <div style="flex: 1;"></div>
          <span class="avatar avatar-sm av-amber" style="border-radius: 7px;">D</span>
        </div>
        <div style="flex: 1; display: flex; flex-direction: column; min-width: 0;">
          <div style="height: 54px; padding: 0 18px; border-bottom: 1px solid ${slBorder}; display: flex; align-items: center; justify-content: space-between;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span style="font-size: 15px; font-weight: 700; color: ${slText};"># ext-stowline-northwind</span>
              <span class="pill pill-purple" style="font-size: 11px;">Slack Connect</span>
            </div>
            <span style="font-size: 11.5px; color: ${slSub};">2 orgs</span>
          </div>
          <div style="flex: 1; padding: 20px 18px; display: flex; flex-direction: column; gap: 16px;">
            <div style="display: flex; gap: 11px;">
              <span class="avatar avatar-lg av-amber" style="border-radius: 7px;">D</span>
              <div style="flex: 1;">
                <div style="font-size: 13.5px; margin-bottom: 4px; color: ${slText};"><strong>Devon Brooks</strong> <span style="font-size: 11.5px; color: ${slSub};">Northwind · 10:21 AM</span></div>
                <div style="font-size: 14px; line-height: 1.5; color: ${slText};">Hey! Our batch ZPL print endpoint started returning <span class="mono" style="font-size: 12px; padding: 1px 5px; border-radius: 4px; background: ${isDark ? "#222529" : "#f8f8f8"}; border: 1px solid ${slBorder}; color: #e01e5a;">422 Invalid weight</span> on 20 orders after our Shopify sync.</div>
                <div id="slack-eyes" style="margin-top: 8px; display: inline-flex; align-items: center; gap: 6px; padding: 3px 9px; border-radius: 999px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 12px; color: var(--accent-fg); opacity: 0;">
                  👀 1 · Claimed by Priya in dimmi
                </div>
              </div>
            </div>

            <div id="slack-reply-thread" style="margin-left: 44px; padding-left: 14px; border-left: 2px solid ${slBorder}; display: flex; flex-direction: column; gap: 14px;">
              <div id="sl-r1" style="display: flex; gap: 10px; opacity: 0;">
                <span class="avatar av-rose" style="border-radius: 7px;">P</span>
                <div>
                  <div style="font-size: 13px; margin-bottom: 3px; color: ${slText};"><strong>Priya Moreau</strong> <span class="pill pill-muted" style="font-size: 10px; padding: 0 6px;">Stowline</span> <span style="font-size: 11.5px; color: ${slSub};">10:22 AM</span></div>
                  <div style="font-size: 13.5px; line-height: 1.5; color: ${slText};">On it Devon! Shopify sent grams instead of ounces on SKU <span class="mono" style="font-size: 12px;">NW-402</span>. Linked ticket <span class="mono" style="color: var(--accent-fg);">#ENG-114</span>.</div>
                </div>
              </div>

              <div id="sl-r2" style="display: flex; gap: 10px; opacity: 0;">
                <span class="avatar av-amber" style="border-radius: 7px;">D</span>
                <div>
                  <div style="font-size: 13px; margin-bottom: 3px; color: ${slText};"><strong>Devon Brooks</strong> <span style="font-size: 11.5px; color: ${slSub};">10:23 AM</span></div>
                  <div style="font-size: 13.5px; line-height: 1.5; color: ${slText};">You caught that fast, fixing the unit mapping on our side now!</div>
                </div>
              </div>
            </div>
          </div>
          <div style="padding: 12px 16px;">
            <div style="border: 1px solid ${slBorder}; border-radius: 10px; padding: 10px 14px; font-size: 12.5px; color: ${slSub}; display: flex; justify-content: space-between; align-items: center;">
              <span>Message #ext-stowline-northwind</span>
              <span class="pill pill-accent" style="font-size: 11px;">Two-way sync</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Right: dimmi shared inbox -->
      ${renderRail("inbox")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="font-size: 15px; font-weight: 600;">Batch ZPL 422 Invalid weight</span>
            <span class="pill pill-purple">${ICONS.slack} #ext-stowline-northwind</span>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="pill pill-amber">${ICONS.clock} 14m SLA left</span>
            <span class="pill pill-blue">#ENG-114</span>
          </div>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px;">
          <div class="card-sub" style="padding: 13px 15px; font-size: 13.5px; line-height: 1.5;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;"><strong style="color: var(--fg);">Devon Brooks</strong> · Slack Connect · 10:21 AM</div>
            Hey! Our batch ZPL print endpoint started returning <span class="mono" style="font-size: 12px;">422 Invalid weight</span> on 20 orders after our Shopify sync.
          </div>

          <div id="dm-note" style="align-self: flex-end; max-width: 82%; padding: 12px 14px; border-radius: 12px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 13px; line-height: 1.45; opacity: 0;">
            <div style="font-size: 11px; font-weight: 600; margin-bottom: 3px;">${ICONS.lock} Private note (not sent to Slack)</div>
            Confirmed in logs: Shopify payload passed <span class="mono">weight: 1450</span> in grams. Linked <span class="mono">#ENG-114</span> for unit auto-detect.
          </div>

          <div id="dm-reply" style="align-self: flex-end; max-width: 82%; padding: 12px 15px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; line-height: 1.5; opacity: 0;">
            <div style="font-size: 11.5px; color: var(--accent-fg); font-weight: 600; margin-bottom: 3px;">Priya Moreau → Synced to Slack thread</div>
            On it Devon! Shopify sent grams instead of ounces on SKU <span class="mono">NW-402</span>. Linked ticket <span class="mono">#ENG-114</span>.
          </div>

          <div id="dm-r2" class="card-sub" style="padding: 12px 15px; font-size: 13.5px; line-height: 1.5; opacity: 0;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;"><strong style="color: var(--fg);">Devon Brooks</strong> · Slack Connect · 10:23 AM</div>
            You caught that fast, fixing the unit mapping on our side now!
          </div>
        </div>
        <div style="padding: 14px 18px; border-top: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center;">
          <span id="sl-composer" style="font-size: 13px; color: var(--fg-3);">Reply to #ext-stowline-northwind...</span>
          <span class="btn btn-primary btn-sm">Send to Slack</span>
        </div>
      </main>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('slack-eyes', t, 0.35, 0.3);
      showPop('dm-note', t, 0.85, 0.35);
      typeText('sl-composer', 'On it Devon! Shopify sent grams instead of ounces on SKU NW-402...', t, 1.35, 2.2);
      showPop('dm-reply', t, 2.35, 0.35);
      showPop('sl-r1', t, 2.5, 0.35);
      showPop('sl-r2', t, 3.45, 0.35);
      showPop('dm-r2', t, 3.6, 0.35);
      if (t >= 2.35) {
        const el = document.getElementById('sl-composer');
        if (el) el.textContent = 'Reply to #ext-stowline-northwind...';
      }
    };
    window.__setTime(4.2);`,
  );
}

function slackSyncHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 20px 22px; display: flex; flex-direction: column; gap: 13px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="pill pill-purple">${ICONS.slack} #support-triage</span>
          <span style="font-size: 13px; font-weight: 600;">Triage card · #312</span>
        </div>
        <span class="pill pill-amber">SLA 14m</span>
      </div>
      <div class="card" style="padding: 14px 16px; border-left: 3px solid var(--accent); display: flex; flex-direction: column; gap: 8px;">
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span style="font-size: 13.5px; font-weight: 600;">Northwind · Batch ZPL 422 error</span>
          <span class="pill pill-accent">Owner: Priya</span>
        </div>
        <div style="font-size: 12.5px; color: var(--fg-2);">Source: <strong>#ext-stowline-northwind</strong> · Linked ticket <strong>#ENG-114</strong></div>
        <div style="display: flex; gap: 6px;">
          <span class="pill pill-muted">👀 Claim</span>
          <span class="pill pill-muted">✅ Close</span>
          <span class="pill pill-muted">🚨 Urgent</span>
        </div>
      </div>
      <div id="ss-1" style="padding: 11px 14px; border-radius: 10px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 12.5px; opacity: 0;">
        <strong>Internal thread reply (hidden from customer):</strong> @eng can we auto-convert grams when weight &gt; 500?
      </div>
      <div id="ss-2" class="card-sub" style="padding: 11px 14px; border-color: var(--accent-border); font-size: 12.5px; opacity: 0;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 3px;">
          <strong style="color: var(--accent-fg);">&gt;&gt; Public reply to customer</strong>
          <span style="color: var(--fg-3);">Synced to dimmi &amp; Slack</span>
        </div>
        Patch is live on your workspace — retry those 20 labels anytime.
      </div>
      <div style="padding: 10px 12px; border-radius: 9px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12px; color: var(--fg-3);">
        Reply here for private notes, or prefix with <span class="mono" style="color: var(--fg);">&gt;&gt;</span> to answer the customer
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('ss-1', t, 0.55, 0.35);
      showPop('ss-2', t, 1.85, 0.4);
    };
    window.__setTime(3.2);`,
  );
}

function slackRoomsHtml(scheme: Scheme): string {
  const modes = [
    ["Shared with a customer", "Tracks external questions in Slack Connect, skips casual thanks.", true],
    ["Community", "Turns help posts into threads while ignoring general chatter.", false],
    ["Help desk", "Internal IT or ops channel with personal request tracking in Slack Home.", false],
    ["Team only", "Operator answers teammates and posts routine approvals here.", false],
  ];
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 14px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">#ext-stowline-northwind</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Choose how dimmi treats messages in this channel</div>
        </div>
        <span class="pill pill-accent">Room mode</span>
      </div>
      <div style="display: flex; flex-direction: column; gap: 9px;">
        ${modes
          .map(
            ([
              name,
              desc,
              selected,
            ]) => `<div class="card" style="padding: 12px 15px; border-color: ${selected ? "var(--accent)" : "var(--border)"}; background: ${selected ? "var(--accent-soft)" : "var(--bg)"}; display: flex; align-items: flex-start; justify-content: space-between; gap: 12px;">
            <div>
              <div style="font-size: 13.5px; font-weight: 600; color: ${selected ? "var(--accent-fg)" : "var(--fg)"};">${name}</div>
              <div style="font-size: 12px; color: var(--fg-2); margin-top: 2px;">${desc}</div>
            </div>
            <span style="width: 18px; height: 18px; border-radius: 999px; border: 2px solid ${selected ? "var(--accent)" : "var(--border-strong)"}; background: ${selected ? "var(--accent)" : "transparent"}; flex-shrink: 0; margin-top: 2px;"></span>
          </div>`,
          )
          .join("")}
      </div>
      <div class="card-sub" style="padding: 11px 14px; display: flex; align-items: center; justify-content: space-between; font-size: 12.5px;">
        <span>Noise filter: skip "thanks", emoji-only, and teammate small talk</span>
        <span class="switch on"></span>
      </div>
    </div>`,
  );
}

function slackOperatorHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 22px 24px; display: flex; flex-direction: column; gap: 14px;">
      <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 9px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 7px;">
            <span class="chip-ai">AI</span>
            <span style="font-size: 13px; font-weight: 600;">Sky drafted a reply · #ext-northwind</span>
          </div>
          <span class="pill pill-amber">Draft for review</span>
        </div>
        <div style="padding: 10px 12px; border-radius: 9px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12.5px; line-height: 1.45; color: var(--fg-2);">
          "We extended USPS bulk voiding to 14 days — select up to 50 labels in Shipments → Labels and click Bulk void."
        </div>
        <div style="display: flex; justify-content: space-between; align-items: center;">
          <span style="font-size: 11.5px; color: var(--fg-3);">Steps back if a teammate replies</span>
          <div style="display: flex; gap: 6px;">
            <span class="btn btn-sm">Edit</span>
            <span class="btn btn-primary btn-sm">Post to thread</span>
          </div>
        </div>
      </div>

      <div class="card" style="padding: 14px 16px; border-color: var(--accent-border); display: flex; flex-direction: column; gap: 9px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 7px;">
            <span class="pill pill-accent">${ICONS.sparkle} Operator</span>
            <span style="font-size: 13px; font-weight: 600;">#support-ops (Team only)</span>
          </div>
          <span style="font-size: 11.5px; color: var(--fg-3);">08:30 AM</span>
        </div>
        <div style="font-size: 13px; line-height: 1.45;">
          <strong>Pull requests to docs</strong> found 1 update from PR #418 for <em>Voiding USPS labels</em>. Ready for your review.
        </div>
        <div style="display: flex; gap: 8px;">
          <span class="btn btn-primary btn-sm">${ICONS.check} Apply edit</span>
          <span class="btn btn-sm">View diff</span>
          <span class="btn btn-sm">Decline</span>
        </div>
      </div>
    </div>`,
  );
}

function slackAlertsHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="font-size: 15px; font-weight: 600;">Routing &amp; disconnect privacy</div>
        <span class="pill pill-accent">3 channels linked</span>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        ${[
          ["#ext-stowline-northwind", "Enterprise Inbox", "#triage-enterprise"],
          ["#ext-stowline-atelier", "EU Support Inbox", "#triage-eu"],
          ["#it-helpdesk", "Internal IT Desk", "#ops-alerts"],
        ]
          .map(
            (
              [ch, inbox, alert],
              idx,
            ) => `<div style="padding: 12px 15px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""} display: grid; grid-template-columns: 1.3fr 1fr 1fr; align-items: center; font-size: 12.5px;">
            <span class="mono" style="font-weight: 600;">${ch}</span>
            <span style="color: var(--fg-2);">${inbox}</span>
            <span class="mono" style="color: var(--accent-fg); text-align: right;">${alert}</span>
          </div>`,
          )
          .join("")}
      </div>
      <div class="card-sub" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 13px; font-weight: 600;">Disconnect &amp; data erasure</span>
          <span class="pill pill-muted">${ICONS.shield} Automatic</span>
        </div>
        <div style="font-size: 12.5px; color: var(--fg-2); line-height: 1.5;">
          Disconnecting Slack revokes stored OAuth tokens immediately, stops listening to every room, and permanently erases Slack messages, files, and IDs after <strong>10 days</strong>.
        </div>
      </div>
    </div>`,
  );
}

function whatsappHeroHtml(scheme: Scheme): string {
  const isDark = scheme === "dark";
  const waHeader = isDark ? "#202c33" : "#075e54";
  const waWall = isDark ? "#0b141a" : "#efeae2";
  const waDot = isDark ? "rgba(255,255,255,0.035)" : "rgba(0,0,0,0.045)";
  const waOut = isDark ? "#005c4b" : "#d9fdd3";
  const waIn = isDark ? "#202c33" : "#ffffff";
  const waText = isDark ? "#e9edef" : "#111b21";
  const waSub = isDark ? "#8696a0" : "#667781";
  const waBar = isDark ? "#202c33" : "#f0f2f5";
  const waInput = isDark ? "#2a3942" : "#ffffff";
  return doc(
    scheme,
    `<div class="app-shell" style="gap: 12px;">
      <!-- Left: Authentic WhatsApp chat UI -->
      <div class="pane" style="width: 600px; flex-shrink: 0; border-color: ${isDark ? "#26343d" : "#d9d4cb"};">
        <div style="height: 60px; padding: 0 18px; background: ${waHeader}; color: #ffffff; display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 12px;">
            <span style="font-size: 18px; opacity: 0.9;">←</span>
            <div style="width: 36px; height: 36px; border-radius: 999px; background: #10b981; color: #fff; font-weight: 700; font-size: 15px; display: flex; align-items: center; justify-content: center;">S</div>
            <div>
              <div style="font-size: 15px; font-weight: 600; display: flex; align-items: center; gap: 6px;">
                Stowline
                <span style="width: 15px; height: 15px; border-radius: 999px; background: #25d366; color: #fff; font-size: 10px; display: inline-flex; align-items: center; justify-content: center;">✓</span>
              </div>
              <div style="font-size: 12px; opacity: 0.82;">Official Business Account · Online</div>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 14px; font-size: 15px; opacity: 0.85;">
            <span>${ICONS.search}</span>
            <span>⋮</span>
          </div>
        </div>

        <div style="flex: 1; padding: 20px 22px; background-color: ${waWall}; background-image: radial-gradient(${waDot} 1.5px, transparent 1.5px); background-size: 22px 22px; display: flex; flex-direction: column; gap: 14px;">
          <div style="align-self: center; padding: 4px 12px; border-radius: 8px; background: ${waIn}; color: ${waSub}; font-size: 11.5px; box-shadow: 0 1px 1px rgb(0 0 0 / 0.08);">
            Today · WhatsApp Business
          </div>

          <div style="align-self: flex-end; max-width: 78%; padding: 10px 13px 8px; border-radius: 12px 2px 12px 12px; background: ${waOut}; color: ${waText}; font-size: 14px; line-height: 1.48; box-shadow: 0 1px 1px rgb(0 0 0 / 0.1);">
            Hi! My parcel <strong>#STW-4910</strong> has been sitting at Lisbon customs since yesterday. Do you need my tax ID?
            <div style="font-size: 11px; color: ${waSub}; margin-top: 4px; text-align: right;">10:42 AM <span style="color: #53bdeb; font-weight: 700;">✓✓</span></div>
          </div>

          <div id="wa-left-ai" style="align-self: flex-start; max-width: 82%; padding: 10px 13px 8px; border-radius: 2px 12px 12px 12px; background: ${waIn}; color: ${waText}; font-size: 14px; line-height: 1.48; box-shadow: 0 1px 1px rgb(0 0 0 / 0.1); opacity: 0;">
            <div style="font-size: 11.5px; font-weight: 600; color: #059669; margin-bottom: 3px;">Stowline Support (AI)</div>
            Yes, DHL needs your Portuguese NIF to clear customs for <strong>#STW-4910</strong>. Reply with your 9-digit NIF here and our EU team will submit it right away.
            <div style="font-size: 11px; color: ${waSub}; margin-top: 4px; text-align: right;">10:42 AM</div>
          </div>

          <div id="wa-left-c2" style="align-self: flex-end; max-width: 66%; padding: 10px 13px 8px; border-radius: 12px 2px 12px 12px; background: ${waOut}; color: ${waText}; font-size: 14px; line-height: 1.48; box-shadow: 0 1px 1px rgb(0 0 0 / 0.1); opacity: 0;">
            Awesome, it's <span class="mono">248910334</span>. Thanks so much!
            <div style="font-size: 11px; color: ${waSub}; margin-top: 4px; text-align: right;">10:43 AM <span style="color: #53bdeb; font-weight: 700;">✓✓</span></div>
          </div>
        </div>

        <div style="padding: 10px 14px; background: ${waBar}; display: flex; align-items: center; gap: 10px;">
          <div style="flex: 1; height: 40px; border-radius: 999px; background: ${waInput}; padding: 0 16px; display: flex; align-items: center; justify-content: space-between; font-size: 13px; color: ${waSub};">
            <span>Message</span>
            <span>${ICONS.paperclip}</span>
          </div>
          <span style="width: 40px; height: 40px; border-radius: 999px; background: #00a884; color: #fff; display: flex; align-items: center; justify-content: center;">${ICONS.send}</span>
        </div>
      </div>

      <!-- Right: dimmi shared inbox -->
      ${renderRail("inbox")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="font-size: 15px; font-weight: 600;">Rosa Costa</span>
            <span class="pill pill-accent">${ICONS.whatsapp} +44 20 7946 0821</span>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="pill pill-accent">${ICONS.clock} 22h 14m left</span>
            <span class="btn btn-sm">Close</span>
          </div>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px;">
          <div class="card-sub" style="padding: 12px 15px; font-size: 13.5px; line-height: 1.5;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;"><strong style="color: var(--fg);">Rosa Costa</strong> · WhatsApp · 10:42 AM</div>
            Hi! My parcel <strong>#STW-4910</strong> has been sitting at Lisbon customs since yesterday. Do you need my tax ID?
          </div>

          <div id="wa-ai" style="align-self: flex-end; max-width: 82%; display: flex; flex-direction: column; gap: 5px; opacity: 0;">
            <div style="display: flex; justify-content: flex-end; align-items: center; gap: 6px; font-size: 11.5px; color: var(--fg-3);">
              <span class="chip-ai">AI</span>
              <span>Sky · Checked parcel #STW-4910</span>
            </div>
            <div style="padding: 12px 15px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; line-height: 1.5;">
              Yes, DHL needs your Portuguese NIF to clear customs for <strong>#STW-4910</strong>. Reply with your 9-digit NIF here and our EU team will submit it right away.
            </div>
          </div>

          <div id="wa-cust2" class="card-sub" style="padding: 12px 15px; font-size: 13.5px; line-height: 1.5; opacity: 0;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;"><strong style="color: var(--fg);">Rosa Costa</strong> · WhatsApp · 10:43 AM</div>
            Awesome, it's <span class="mono">248910334</span>. Thanks so much!
          </div>

          <div id="wa-handover" style="align-self: flex-end; max-width: 84%; padding: 11px 14px; border-radius: 12px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 13px; opacity: 0;">
            <strong>Handed over to EU Ops:</strong> Customer sent NIF 248910334 for DHL customs clearance on #STW-4910.
          </div>
        </div>
        <div style="padding: 14px 18px; border-top: 1px solid var(--border);">
          <div style="padding: 11px 14px; border-radius: 11px; border: 1px solid var(--border); background: var(--bg-sub); display: flex; justify-content: space-between; align-items: center; font-size: 13px; color: var(--fg-3);">
            <span>Type a message or / for approved templates...</span>
            <span class="pill pill-accent">22h 14m free-form</span>
          </div>
        </div>
      </main>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('wa-ai', t, 0.45, 0.35);
      showPop('wa-left-ai', t, 0.6, 0.35);
      showPop('wa-left-c2', t, 1.85, 0.35);
      showPop('wa-cust2', t, 2.0, 0.35);
      showPop('wa-handover', t, 2.95, 0.38);
    };
    window.__setTime(3.8);`,
  );
}

function whatsappTemplatesHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 22px 24px; display: flex; flex-direction: column; gap: 14px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="font-size: 14.5px; font-weight: 600;">Send Meta-approved template</div>
        <span class="pill pill-accent">${ICONS.check} Synced with Meta</span>
      </div>

      <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span class="mono" style="font-size: 13px; font-weight: 600;">shipment_delay_update</span>
          <span class="pill pill-accent">APPROVED · Utility</span>
        </div>
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
          <div style="padding: 8px 10px; border-radius: 8px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12px;">
            <span style="color: var(--fg-3); display: block; font-size: 10.5px;">{{1}} Customer name</span>
            <strong id="wa-v1">Rosa</strong>
          </div>
          <div style="padding: 8px 10px; border-radius: 8px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12px;">
            <span style="color: var(--fg-3); display: block; font-size: 10.5px;">{{2}} Tracking number</span>
            <strong id="wa-v2" class="mono">#STW-4910</strong>
          </div>
        </div>
      </div>

      <div id="wa-preview" class="card-sub" style="padding: 15px 16px; border-color: var(--accent-border); display: flex; flex-direction: column; gap: 9px; opacity: 0;">
        <div style="font-size: 11px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--accent-fg); font-weight: 600;">Live WhatsApp preview</div>
        <div style="font-size: 13.5px; font-weight: 600;">Customs clearance update</div>
        <div style="font-size: 13px; line-height: 1.5; color: var(--fg);">
          Hi <strong>Rosa</strong>, your shipment <strong>#STW-4910</strong> has cleared Lisbon customs and is out for delivery tomorrow morning.
        </div>
        <div style="font-size: 11.5px; color: var(--fg-3);">Stowline Shipping · Reply anytime to chat</div>
        <div style="display: flex; gap: 8px; padding-top: 4px;">
          <span class="btn btn-sm" style="flex: 1; color: var(--accent-fg);">Track parcel ↗</span>
          <span class="btn btn-sm" style="flex: 1;">Need help</span>
        </div>
      </div>

      <div style="display: flex; align-items: center; justify-content: space-between; padding-top: 4px;">
        <span class="mono" style="font-size: 12px; color: var(--fg-3);">Composer: <strong id="wa-slash" style="color: var(--fg);">/shipment_delay</strong></span>
        <span id="wa-send-btn" class="btn btn-primary btn-sm">Send template</span>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      typeText('wa-slash', '/shipment_delay_update', t, 0.15, 1.1);
      typeText('wa-v1', 'Rosa', t, 1.2, 1.6);
      typeText('wa-v2', '#STW-4910', t, 1.65, 2.2);
      showPop('wa-preview', t, 2.25, 0.4);
    };
    window.__setTime(3.4);`,
  );
}

function whatsappSignupHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">WhatsApp Business numbers</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Connected via official Meta Embedded Signup</div>
        </div>
        <span class="btn btn-primary btn-sm">+ Connect Meta</span>
      </div>
      <div style="display: flex; flex-direction: column; gap: 10px;">
        ${[
          ["+1 (415) 555-0192", "US Support", "Phone app coexistence ON", "Verified"],
          ["+44 20 7946 0821", "EU Operations", "Cloud API direct", "Verified"],
        ]
          .map(
            ([
              num,
              inbox,
              mode,
              st,
            ]) => `<div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span class="mono" style="font-size: 14px; font-weight: 600;">${num}</span>
              <span class="pill pill-accent">${ICONS.check} ${st}</span>
            </div>
            <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; color: var(--fg-2);">
              <span>Inbox: <strong style="color: var(--fg);">${inbox}</strong></span>
              <span class="pill pill-muted">${mode}</span>
            </div>
          </div>`,
          )
          .join("")}
      </div>
      <div class="card-sub" style="padding: 13px 15px; border-color: var(--accent-border); background: var(--accent-soft); color: var(--accent-fg); font-size: 12.5px; line-height: 1.45;">
        <strong>Direct Meta billing ($0 markup):</strong> Template messages bill directly to your Meta Business Account. Keep using the WhatsApp Business phone app alongside dimmi anytime.
      </div>
    </div>`,
  );
}

function whatsappWindowHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 16px;">
      <div style="font-size: 15px; font-weight: 600; padding-bottom: 12px; border-bottom: 1px solid var(--border);">24-hour reply clock in the composer</div>

      <div class="card" style="padding: 15px 16px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 13px; font-weight: 600;">Inside 24-hour window</span>
          <span class="pill pill-accent">${ICONS.clock} 18h 42m remaining</span>
        </div>
        <div style="padding: 11px 13px; border-radius: 10px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 13px; color: var(--fg-2); display: flex; justify-content: space-between; align-items: center;">
          <span>Free-form reply or type / for templates...</span>
          <span class="btn btn-primary btn-sm">Send</span>
        </div>
      </div>

      <div class="card" style="padding: 15px 16px; border-color: var(--amber-border); display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 13px; font-weight: 600;">Window closed (24h elapsed)</span>
          <span class="pill pill-amber">${ICONS.lock} Template required</span>
        </div>
        <div style="padding: 12px 14px; border-radius: 10px; background: var(--amber-soft); border: 1px solid var(--amber-border); display: flex; align-items: center; justify-content: space-between; gap: 10px;">
          <div style="font-size: 12.5px; color: var(--amber-fg); line-height: 1.4;">
            Free-form replies reopen when the customer responds. Send a Meta-approved template to follow up now.
          </div>
          <span class="btn btn-sm">Pick template</span>
        </div>
      </div>

      <div style="font-size: 12px; color: var(--fg-3);">Messages never fail silently — dimmi locks free-form input at 0m and unlocks automatically on reply.</div>
    </div>`,
  );
}

function whatsappMediaHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 22px 24px; display: flex; flex-direction: column; gap: 13px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 10px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="avatar avatar-sm av-amber">N</span>
          <span style="font-size: 13.5px; font-weight: 600;">Nadia Okafor</span>
          <span class="pill pill-muted">US Support</span>
        </div>
        <span class="pill pill-blue">Broadcast reply · Oct Promo</span>
      </div>

      <!-- Photo + voice note -->
      <div style="display: flex; gap: 10px; align-items: stretch;">
        <div style="width: 185px; height: 120px; border-radius: 12px; background: linear-gradient(135deg, #d1fae5 0%, #a7f3d0 100%); border: 1px solid var(--border); padding: 12px; display: flex; flex-direction: column; justify-content: space-between; color: #065f46;">
          <span class="mono" style="font-size: 10.5px; font-weight: 600; background: rgba(255,255,255,0.75); padding: 2px 6px; border-radius: 5px; align-self: flex-start;">IMG_4821.jpg</span>
          <div style="font-size: 12px; font-weight: 600;">Parcel on scale · 1 lb 12 oz</div>
        </div>
        <div style="flex: 1; padding: 12px 14px; border-radius: 12px; background: var(--bg-sub); border: 1px solid var(--border); display: flex; flex-direction: column; justify-content: space-between;">
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <span style="font-size: 12px; font-weight: 600;">Voice note</span>
            <span class="mono" style="font-size: 11px; color: var(--fg-3);">0:14</span>
          </div>
          <div style="display: flex; align-items: center; gap: 3px; height: 28px;">
            ${[10, 18, 26, 14, 24, 28, 16, 20, 12, 24, 26, 15, 19, 11, 22, 17, 13, 25, 18, 10]
              .map(
                (h) =>
                  `<span style="width: 4px; height: ${h}px; border-radius: 99px; background: var(--accent);"></span>`,
              )
              .join("")}
          </div>
          <span style="font-size: 11px; color: var(--fg-3);">"Here is the scale photo for #STW-4829..."</span>
        </div>
      </div>

      <div style="padding: 12px 14px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13px; line-height: 1.5;">
        <div style="display: flex; align-items: center; gap: 6px; margin-bottom: 4px; font-size: 11.5px; color: var(--accent-fg); font-weight: 600;">
          <span class="chip-ai">AI</span> Sky · Attached return-label.pdf
        </div>
        Thanks Nadia! I've logged your photo for the USPS dispute and attached your replacement shipping label below.
      </div>

      <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3); padding-top: 4px;">
        <span>Supports photos, video, voice notes, locations &amp; PDFs</span>
        <span class="pill pill-accent">Read ✓✓</span>
      </div>
    </div>`,
  );
}

export const SLACK_WHATSAPP_SCENES: SceneSpec[] = [
  {
    name: "channel-slack-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.6,
    posterTime: 4.1,
    html: slackHeroHtml,
  },
  {
    name: "channel-slack-sync",
    width: 720,
    height: 640,
    kind: "video",
    duration: 3.6,
    posterTime: 3.1,
    html: slackSyncHtml,
  },
  { name: "channel-slack-rooms", width: 720, height: 640, kind: "still", html: slackRoomsHtml },
  { name: "channel-slack-operator", width: 720, height: 640, kind: "still", html: slackOperatorHtml },
  { name: "channel-slack-alerts", width: 720, height: 640, kind: "still", html: slackAlertsHtml },
  {
    name: "channel-whatsapp-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.2,
    posterTime: 3.7,
    html: whatsappHeroHtml,
  },
  {
    name: "channel-whatsapp-templates",
    width: 720,
    height: 640,
    kind: "video",
    duration: 3.8,
    posterTime: 3.3,
    html: whatsappTemplatesHtml,
  },
  { name: "channel-whatsapp-signup", width: 720, height: 640, kind: "still", html: whatsappSignupHtml },
  { name: "channel-whatsapp-window", width: 720, height: 640, kind: "still", html: whatsappWindowHtml },
  { name: "channel-whatsapp-media", width: 720, height: 640, kind: "still", html: whatsappMediaHtml },
];
