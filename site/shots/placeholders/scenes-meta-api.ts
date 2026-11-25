import { doc, ICONS, renderRail, type SceneSpec, type Scheme, TIMELINE_SCRIPT_HELPER } from "./theme.ts";

function instagramHeroHtml(scheme: Scheme): string {
  const isDark = scheme === "dark";
  const igBg = isDark ? "#000000" : "#ffffff";
  const igBorder = isDark ? "#262626" : "#dbdbdb";
  const igIn = isDark ? "#262626" : "#efefef";
  const igText = isDark ? "#fafafa" : "#111111";
  const igSub = isDark ? "#a8a8a8" : "#737373";
  return doc(
    scheme,
    `<div class="app-shell" style="gap: 12px;">
      <!-- Left: Authentic Instagram DM UI -->
      <div class="pane" style="width: 600px; flex-shrink: 0; background: ${igBg}; border-color: ${igBorder};">
        <div style="height: 62px; padding: 0 18px; border-bottom: 1px solid ${igBorder}; display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 12px;">
            <span style="font-size: 18px; color: ${igText};">←</span>
            <div style="padding: 2px; border-radius: 999px; background: linear-gradient(45deg, #f09433 0%, #e6683c 25%, #dc2743 50%, #cc2366 75%, #bc1888 100%);">
              <div style="width: 34px; height: 34px; border-radius: 999px; background: #059669; border: 2px solid ${igBg}; color: #fff; font-weight: 700; font-size: 14px; display: flex; align-items: center; justify-content: center;">S</div>
            </div>
            <div>
              <div style="font-size: 14.5px; font-weight: 600; color: ${igText}; display: flex; align-items: center; gap: 5px;">
                stowline.co
                <span style="width: 15px; height: 15px; border-radius: 999px; background: #0095f6; color: #fff; font-size: 9.5px; display: inline-flex; align-items: center; justify-content: center;">✓</span>
              </div>
              <div style="font-size: 12px; color: ${igSub};">Instagram · Active now</div>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 7px; font-size: 12.5px; color: ${igSub}; font-weight: 600;">
            ${ICONS.instagram}
            <span>Instagram</span>
          </div>
        </div>

        <div style="flex: 1; padding: 22px 22px; display: flex; flex-direction: column; gap: 16px;">
          <div style="align-self: center; font-size: 11.5px; color: ${igSub};">Today, 10:18 AM</div>

          <div style="align-self: flex-end; max-width: 80%; display: flex; flex-direction: column; align-items: flex-end; gap: 7px;">
            <div style="font-size: 11.5px; color: ${igSub};">You replied to their story</div>
            <div style="display: flex; align-items: center; gap: 10px;">
              <div style="width: 58px; height: 84px; border-radius: 12px; background: linear-gradient(135deg, #059669 0%, #0f766e 100%); padding: 7px; color: #fff; font-size: 9.5px; font-weight: 600; display: flex; flex-direction: column; justify-content: flex-end; border: 1px solid ${igBorder};">
                Emerald mailers
              </div>
              <div style="width: 3px; height: 70px; border-radius: 99px; background: ${igBorder};"></div>
            </div>
            <div style="padding: 11px 15px; border-radius: 20px 20px 6px 20px; background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); color: #ffffff; font-size: 14px; line-height: 1.45;">
              Love these!! Can I swap the 200-pack in my order <strong>#STW-4829</strong> before it ships out?
            </div>
          </div>

          <div id="ig-left-ai" style="align-self: flex-start; max-width: 82%; display: flex; align-items: flex-end; gap: 8px; opacity: 0;">
            <div style="width: 26px; height: 26px; border-radius: 999px; background: #059669; color: #fff; font-size: 11px; font-weight: 700; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">S</div>
            <div style="padding: 11px 15px; border-radius: 20px 20px 20px 6px; background: ${igIn}; color: ${igText}; font-size: 14px; line-height: 1.45;">
              Order <strong>#STW-4829</strong> hasn't been picked yet! I can pause fulfillment and swap the 200-pack to recycled emerald polymailers with zero price difference. Want me to confirm that?
            </div>
          </div>

          <div id="ig-left-c2" style="align-self: flex-end; max-width: 68%; opacity: 0;">
            <div style="padding: 10px 15px; border-radius: 20px 20px 6px 20px; background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 100%); color: #ffffff; font-size: 14px;">
              Yes please! That's amazing 🙌
            </div>
            <div style="font-size: 11px; color: ${igSub}; text-align: right; margin-top: 4px;">Seen</div>
          </div>
        </div>

        <div style="padding: 12px 16px;">
          <div style="height: 44px; border-radius: 999px; border: 1px solid ${igBorder}; padding: 0 14px 0 8px; display: flex; align-items: center; justify-content: space-between; font-size: 13.5px; color: ${igSub};">
            <div style="display: flex; align-items: center; gap: 10px;">
              <span style="width: 30px; height: 30px; border-radius: 999px; background: ${igIn}; display: flex; align-items: center; justify-content: center;">${ICONS.instagram}</span>
              <span>Message...</span>
            </div>
            <span style="font-size: 12px; font-weight: 600; color: #0095f6;">Send</span>
          </div>
        </div>
      </div>

      <!-- Right: dimmi shared inbox -->
      ${renderRail("inbox")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="font-size: 15px; font-weight: 600;">@maya.chen</span>
            <span class="pill pill-purple">${ICONS.instagram} @stowline.co</span>
          </div>
          <span class="pill pill-accent">${ICONS.clock} 23h 40m AI · 6d 23h human</span>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px;">
          <div style="display: flex; flex-direction: column; gap: 6px;">
            <div style="padding: 7px 11px; border-radius: 9px; background: var(--purple-soft); border: 1px solid var(--purple-border); font-size: 12px; color: var(--purple-fg); align-self: flex-start;">
              ${ICONS.instagram} Replied to Story: "Emerald mailers restocked" ↗
            </div>
            <div class="card-sub" style="padding: 12px 15px; font-size: 13.5px; line-height: 1.5;">
              Love these!! Can I swap the 200-pack in my order <strong>#STW-4829</strong> before it ships out?
            </div>
          </div>

          <div id="ig-ai" style="align-self: flex-end; max-width: 82%; display: flex; flex-direction: column; gap: 5px; opacity: 0;">
            <div style="display: flex; justify-content: flex-end; align-items: center; gap: 6px; font-size: 11.5px; color: var(--fg-3);">
              <span class="chip-ai">AI</span>
              <span>Orders &amp; Returns · Checked Shopify #STW-4829</span>
            </div>
            <div style="padding: 12px 15px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; line-height: 1.5;">
              Order <strong>#STW-4829</strong> hasn't been picked yet! I can pause fulfillment and swap the 200-pack to recycled emerald polymailers with zero price difference. Want me to confirm that?
            </div>
          </div>

          <div id="ig-cust2" class="card-sub" style="padding: 11px 15px; font-size: 13.5px; opacity: 0;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 3px;"><strong style="color: var(--fg);">@maya.chen</strong> · Instagram DM</div>
            Yes please! That's amazing 🙌
          </div>

          <div id="ig-note" style="align-self: flex-end; max-width: 82%; padding: 11px 14px; border-radius: 12px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 13px; opacity: 0;">
            <strong>${ICONS.lock} Private note · @warehouse:</strong> Swapped SKU on #STW-4829 to PM-200-EMR and released hold.
          </div>
        </div>
        <div style="padding: 14px 18px; border-top: 1px solid var(--border);">
          <div style="padding: 11px 14px; border-radius: 11px; border: 1px solid var(--border); background: var(--bg-sub); display: flex; justify-content: space-between; align-items: center; font-size: 13px; color: var(--fg-3);">
            <span>Reply to @maya.chen on Instagram...</span>
            <span class="pill pill-muted">Photos · Audio · PDF</span>
          </div>
        </div>
      </main>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('ig-ai', t, 0.45, 0.35);
      showPop('ig-left-ai', t, 0.6, 0.35);
      showPop('ig-left-c2', t, 1.85, 0.35);
      showPop('ig-cust2', t, 2.0, 0.35);
      showPop('ig-note', t, 2.9, 0.38);
    };
    window.__setTime(3.8);`,
  );
}

function instagramThreadHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 20px 22px; display: flex; flex-direction: column; gap: 12px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 10px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="avatar avatar-sm av-rose">M</span>
          <span style="font-size: 13.5px; font-weight: 600;">@maya.chen</span>
          <span class="pill pill-muted">Order #STW-4829 · Paid</span>
        </div>
        <span class="pill pill-purple">${ICONS.instagram} Instagram DM</span>
      </div>

      <div class="card-sub" style="padding: 11px 14px; font-size: 13px; line-height: 1.45;">
        Can I add 100 thermal labels to #STW-4829 before the warehouse packs it?
      </div>

      <div id="igt-note" style="padding: 11px 14px; border-radius: 10px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 12.5px; opacity: 0;">
        <strong>${ICONS.lock} Internal note:</strong> @ops confirmed bin #14 isn't sealed yet — added thermal pack!
      </div>

      <div id="igt-reply" style="padding: 12px 14px; border-radius: 11px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13px; line-height: 1.45; opacity: 0;">
        <div style="font-size: 11px; font-weight: 600; color: var(--accent-fg); margin-bottom: 3px;">Priya Moreau → Sent to Instagram DM</div>
        Done! Added the 100 thermal labels to #STW-4829 and emailed your updated receipt to maya@chenstudio.co.
      </div>

      <div style="padding: 10px 12px; border-radius: 9px; background: var(--bg-sub); border: 1px solid var(--border); display: flex; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
        <span>Merged with Email &amp; Messenger · 6 past orders</span>
        <span style="color: var(--accent-fg);">Synced</span>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('igt-note', t, 0.55, 0.35);
      showPop('igt-reply', t, 1.85, 0.4);
    };
    window.__setTime(3.2);`,
  );
}

function instagramConnectHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Instagram Business accounts</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Connected via official Meta OAuth</div>
        </div>
        <span class="btn btn-primary btn-sm">+ Connect account</span>
      </div>
      <div style="display: flex; flex-direction: column; gap: 10px;">
        ${[
          ["@stowline.co", "US Brand Inbox", "Orders & Returns AI", "Allow access to messages ✓"],
          ["@stowline.eu", "EU Brand Inbox", "EU Support Team", "Allow access to messages ✓"],
        ]
          .map(
            ([
              handle,
              inbox,
              agent,
              perms,
            ]) => `<div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span style="font-size: 14px; font-weight: 600; display: flex; align-items: center; gap: 7px;">${ICONS.instagram} ${handle}</span>
              <span class="pill pill-accent">${ICONS.check} Active</span>
            </div>
            <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; color: var(--fg-2);">
              <span>Inbox: <strong style="color: var(--fg);">${inbox}</strong> · ${agent}</span>
              <span class="pill pill-muted">${perms}</span>
            </div>
          </div>`,
          )
          .join("")}
      </div>
      <div class="card-sub" style="padding: 12px 14px; font-size: 12.5px; color: var(--fg-2);">
        No shared phone logins or per-channel add-on fees. Route each brand handle to its own team and saved replies.
      </div>
    </div>`,
  );
}

function instagramMediaHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 22px 24px; display: flex; flex-direction: column; gap: 13px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 10px; border-bottom: 1px solid var(--border);">
        <span style="font-size: 14px; font-weight: 600;">Story reply &amp; grouped photos</span>
        <span class="pill pill-purple">Edited in place</span>
      </div>
      <div style="padding: 8px 12px; border-radius: 9px; background: var(--purple-soft); border: 1px solid var(--purple-border); font-size: 12px; color: var(--purple-fg);">
        ${ICONS.instagram} Story reply · "Spring packaging drop" ↗
      </div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
        <div style="height: 112px; border-radius: 11px; background: linear-gradient(135deg, #fce7f3 0%, #fbcfe8 100%); border: 1px solid var(--border); padding: 10px; display: flex; flex-direction: column; justify-content: flex-end; color: #831843; font-size: 12px; font-weight: 600;">Damaged corner photo 1/2</div>
        <div style="height: 112px; border-radius: 11px; background: linear-gradient(135deg, #e0e7ff 0%, #c7d2fe 100%); border: 1px solid var(--border); padding: 10px; display: flex; flex-direction: column; justify-content: flex-end; color: #312e81; font-size: 12px; font-weight: 600;">Shipping label photo 2/2</div>
      </div>
      <div style="display: inline-flex; align-items: center; gap: 8px; padding: 9px 12px; border-radius: 10px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12.5px; font-weight: 500; align-self: flex-end;">
        ${ICONS.paperclip} return-label-STW-4829.pdf <span style="color: var(--fg-3);">Sent to DM</span>
      </div>
      <div style="padding: 11px 14px; border-radius: 10px; background: var(--bg-sub); border: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center; font-size: 12px;">
        <span style="color: var(--fg-2);">Meta reply clocks</span>
        <div style="display: flex; gap: 6px;">
          <span class="pill pill-accent">21h 14m AI</span>
          <span class="pill pill-blue">6d 21h Teammate</span>
        </div>
      </div>
    </div>`,
  );
}

function instagramAgentsHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="avatar avatar-sm av-emerald">${ICONS.sparkle}</span>
          <span style="font-size: 15px; font-weight: 600;">Orders &amp; Returns agent</span>
          <span class="chip-ai">AI</span>
        </div>
        <span class="pill pill-accent">Assigned to @stowline.co</span>
      </div>
      <div class="card-sub" style="padding: 13px 15px; display: flex; flex-direction: column; gap: 9px; font-size: 13px; line-height: 1.5;">
        <div style="font-size: 11.5px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">Active guidance</div>
        <div>• Look up order status with <span class="pill pill-lime" style="padding: 0 7px; font-size: 11.5px;">⚡ Shopify order lookup</span> before replying.</div>
        <div>• For size swaps or damaged items, follow <span class="pill pill-purple" style="padding: 0 7px; font-size: 11.5px;">≡ Instant return label</span>.</div>
        <div>• Escalate wholesale inquiries over $1,000 straight to <span class="pill pill-orange" style="padding: 0 7px; font-size: 11.5px;">VIP Sales</span>.</div>
      </div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
        <div class="card" style="padding: 13px 15px;">
          <div style="font-size: 11.5px; color: var(--fg-3);">Escalation workflow</div>
          <div style="font-size: 13.5px; font-weight: 600; margin-top: 3px;">Damaged item → Priority</div>
          <div style="font-size: 12px; color: var(--fg-2); margin-top: 2px;">Tags order &amp; alerts #ops</div>
        </div>
        <div class="card" style="padding: 13px 15px;">
          <div style="font-size: 11.5px; color: var(--fg-3);">SLA policy</div>
          <div style="font-size: 13.5px; font-weight: 600; margin-top: 3px;">15m first reply</div>
          <div style="font-size: 12px; color: var(--accent-fg); margin-top: 2px;">98.4% met this week</div>
        </div>
      </div>
    </div>`,
  );
}

function facebookHeroHtml(scheme: Scheme): string {
  const isDark = scheme === "dark";
  const fbBg = isDark ? "#1c1e21" : "#ffffff";
  const fbBorder = isDark ? "#2f3237" : "#e4e6eb";
  const fbIn = isDark ? "#303236" : "#f0f0f0";
  const fbText = isDark ? "#e4e6eb" : "#050505";
  const fbSub = isDark ? "#b0b3b8" : "#65676b";
  const fbBlue = "#0084ff";
  return doc(
    scheme,
    `<div class="app-shell" style="gap: 12px;">
      <!-- Left: Authentic Facebook Messenger UI -->
      <div class="pane" style="width: 600px; flex-shrink: 0; background: ${fbBg}; border-color: ${fbBorder};">
        <div style="height: 62px; padding: 0 18px; border-bottom: 1px solid ${fbBorder}; display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 11px;">
            <div style="position: relative;">
              <div style="width: 38px; height: 38px; border-radius: 999px; background: #059669; color: #fff; font-weight: 700; font-size: 16px; display: flex; align-items: center; justify-content: center;">S</div>
              <span style="position: absolute; right: 0; bottom: 0; width: 11px; height: 11px; border-radius: 999px; background: #31a24c; border: 2px solid ${fbBg};"></span>
            </div>
            <div>
              <div style="font-size: 15px; font-weight: 600; color: ${fbText}; display: flex; align-items: center; gap: 6px;">
                Stowline
                <span style="width: 15px; height: 15px; border-radius: 999px; background: ${fbBlue}; color: #fff; font-size: 9.5px; display: inline-flex; align-items: center; justify-content: center;">✓</span>
              </div>
              <div style="font-size: 12px; color: ${fbSub};">Active now · Facebook Page</div>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 8px; color: ${fbBlue}; font-weight: 600; font-size: 13px;">
            ${ICONS.messenger}
            <span>Messenger</span>
          </div>
        </div>

        <div style="flex: 1; padding: 22px 22px; display: flex; flex-direction: column; gap: 16px;">
          <div style="align-self: center; text-align: center; display: flex; flex-direction: column; align-items: center; gap: 4px; padding-bottom: 8px;">
            <div style="font-size: 12px; font-weight: 600; color: ${fbSub};">You're messaging Stowline on Messenger</div>
            <div style="font-size: 11.5px; color: ${fbSub};">Today, 11:04 AM</div>
          </div>

          <div style="align-self: flex-end; max-width: 78%; padding: 11px 16px; border-radius: 20px 20px 6px 20px; background: ${fbBlue}; color: #ffffff; font-size: 14px; line-height: 1.48;">
            Hi Stowline! Do your USPS Priority discounted rates cover San Juan, Puerto Rico, and can I download a rate sheet?
          </div>

          <div id="fb-left-reply" style="align-self: flex-start; max-width: 82%; display: flex; align-items: flex-end; gap: 8px; opacity: 0;">
            <div style="width: 28px; height: 28px; border-radius: 999px; background: #059669; color: #fff; font-size: 12px; font-weight: 700; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">S</div>
            <div style="display: flex; flex-direction: column; gap: 7px;">
              <div style="padding: 11px 16px; border-radius: 20px 20px 20px 6px; background: ${fbIn}; color: ${fbText}; font-size: 14px; line-height: 1.48;">
                Yes! Puerto Rico uses domestic USPS Priority rates with zero customs forms needed. I attached our PR rate sheet PDF right below.
              </div>
              <div style="padding: 10px 14px; border-radius: 16px; background: ${fbIn}; border: 1px solid ${fbBorder}; display: inline-flex; align-items: center; gap: 9px; align-self: flex-start; font-size: 12.5px; font-weight: 600; color: ${fbText};">
                <span style="width: 26px; height: 26px; border-radius: 999px; background: ${fbBlue}; color: #fff; display: flex; align-items: center; justify-content: center;">${ICONS.paperclip}</span>
                usps-pr-rates-2026.pdf
              </div>
            </div>
          </div>
        </div>

        <div style="padding: 12px 16px; border-top: 1px solid ${fbBorder}; display: flex; align-items: center; gap: 10px;">
          <span style="width: 30px; height: 30px; border-radius: 999px; background: ${fbBlue}; color: #fff; font-weight: 700; font-size: 16px; display: flex; align-items: center; justify-content: center;">+</span>
          <div style="flex: 1; height: 38px; border-radius: 999px; background: ${fbIn}; padding: 0 16px; display: flex; align-items: center; font-size: 13.5px; color: ${fbSub};">Aa</div>
          <span style="font-size: 20px; color: ${fbBlue};">👍</span>
        </div>
      </div>

      <!-- Right: dimmi shared inbox -->
      ${renderRail("inbox")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="font-size: 15px; font-weight: 600;">Daniel Morales</span>
            <span class="pill pill-blue">${ICONS.messenger} Stowline Page</span>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="pill pill-accent">${ICONS.clock} 23h 18m window</span>
            <span class="btn btn-sm">Close</span>
          </div>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px;">
          <div class="card-sub" style="padding: 12px 15px; font-size: 13.5px; line-height: 1.5;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;"><strong style="color: var(--fg);">Daniel Morales</strong> · Facebook Messenger · 11:04 AM</div>
            Hi Stowline! Do your USPS Priority discounted rates cover San Juan, Puerto Rico, and can I download a rate sheet?
          </div>

          <div id="fb-note" style="align-self: flex-end; max-width: 82%; padding: 11px 14px; border-radius: 12px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 13px; opacity: 0;">
            <strong>${ICONS.lock} Private note:</strong> Daniel also emailed us last month about Shopify rate sync. Linked his profile.
          </div>

          <div id="fb-reply" style="align-self: flex-end; max-width: 82%; padding: 12px 15px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; line-height: 1.5; opacity: 0;">
            <div style="font-size: 11.5px; color: var(--accent-fg); font-weight: 600; margin-bottom: 4px;">Maya Lin → Sent to Facebook Messenger</div>
            <div>Yes! Puerto Rico uses domestic USPS Priority rates with zero customs forms needed. I attached our PR rate sheet PDF right below.</div>
            <div style="margin-top: 8px;">
              <span style="display: inline-flex; align-items: center; gap: 6px; padding: 5px 9px; border-radius: 8px; background: var(--bg); border: 1px solid var(--border); font-size: 12px;">${ICONS.paperclip} usps-pr-rates-2026.pdf</span>
            </div>
          </div>
        </div>
        <div style="padding: 14px 18px; border-top: 1px solid var(--border);">
          <div style="padding: 11px 14px; border-radius: 11px; border: 1px solid var(--border); background: var(--bg-sub); display: flex; justify-content: space-between; align-items: center; font-size: 13px; color: var(--fg-3);">
            <span id="fb-comp">Reply to Daniel Morales on Messenger...</span>
            <span class="btn btn-primary btn-sm">Send</span>
          </div>
        </div>
      </main>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('fb-note', t, 0.45, 0.35);
      typeText('fb-comp', 'Yes! Puerto Rico uses domestic USPS Priority rates...', t, 1.05, 2.15);
      showPop('fb-reply', t, 2.35, 0.38);
      showPop('fb-left-reply', t, 2.5, 0.38);
      if (t >= 2.35) {
        const el = document.getElementById('fb-comp');
        if (el) el.textContent = 'Reply to Daniel Morales on Messenger...';
      }
    };
    window.__setTime(3.5);`,
  );
}

function facebookThreadHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 20px 22px; display: flex; flex-direction: column; gap: 12px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 10px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="avatar avatar-sm av-violet">E</span>
          <span style="font-size: 13.5px; font-weight: 600;">Elena Vance</span>
          <span class="pill pill-blue">${ICONS.messenger} Messenger</span>
        </div>
        <span class="pill pill-muted">2 past threads (Email, WA)</span>
      </div>
      <div class="card-sub" style="padding: 11px 14px; font-size: 13px; line-height: 1.45;">
        Hey! Could you resend the invoice PDF for order #4802? My accountant needs it today.
      </div>
      <div id="fbt-note" style="padding: 11px 14px; border-radius: 10px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 12.5px; opacity: 0;">
        <strong>${ICONS.lock} Internal note:</strong> Matched Elena's email profile — pulling invoice #4802 PDF now.
      </div>
      <div id="fbt-reply" style="padding: 12px 14px; border-radius: 11px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13px; line-height: 1.45; opacity: 0;">
        <div style="font-size: 11px; font-weight: 600; color: var(--accent-fg); margin-bottom: 3px;">Priya Moreau → Posted back to Messenger</div>
        Here is Invoice #4802 attached as a PDF — let us know if your accountant needs a VAT line too!
      </div>
      <div style="padding: 10px 12px; border-radius: 9px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12px; color: var(--fg-3); display: flex; justify-content: space-between;">
        <span>No Meta Business Suite tab needed</span>
        <span style="color: var(--accent-fg);">Delivered ✓</span>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('fbt-note', t, 0.55, 0.35);
      showPop('fbt-reply', t, 1.85, 0.4);
    };
    window.__setTime(3.2);`,
  );
}

function facebookConnectHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Facebook Pages</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Meta Business Portfolio · Official OAuth</div>
        </div>
        <span class="btn btn-primary btn-sm">+ Attach Page</span>
      </div>
      <div style="display: flex; flex-direction: column; gap: 10px;">
        ${[
          ["Stowline", "US Support Inbox", "Token valid · Webhook healthy"],
          ["Stowline Europe", "EU Support Inbox", "Token valid · Webhook healthy"],
        ]
          .map(
            ([
              page,
              inbox,
              health,
            ]) => `<div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span style="font-size: 14px; font-weight: 600; display: flex; align-items: center; gap: 7px;">${ICONS.messenger} ${page}</span>
              <span class="pill pill-accent"><span class="dot dot-green"></span>Healthy</span>
            </div>
            <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; color: var(--fg-2);">
              <span>Routed to <strong style="color: var(--fg);">${inbox}</strong></span>
              <span style="font-size: 12px; color: var(--accent-fg);">${health}</span>
            </div>
          </div>`,
          )
          .join("")}
      </div>
      <div class="card-sub" style="padding: 12px 14px; font-size: 12.5px; color: var(--fg-2);">
        Live connection health checks monitor Page permissions and alert your team before expired tokens drop messages.
      </div>
    </div>`,
  );
}

function facebookRoutingHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="font-size: 15px; font-weight: 600;">Multi-brand routing, files &amp; windows</div>
        <span class="pill pill-blue">2 Pages active</span>
      </div>
      <div class="card-sub" style="padding: 13px 15px; display: flex; flex-direction: column; gap: 8px;">
        <div style="font-size: 12px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">Supported attachments in thread</div>
        <div style="display: flex; flex-wrap: wrap; gap: 8px;">
          <span class="pill pill-muted">${ICONS.paperclip} invoice-4829.pdf</span>
          <span class="pill pill-muted">parcel-photo.jpg</span>
          <span class="pill pill-muted">unboxing.mp4</span>
          <span class="pill pill-muted">voice-note.m4a</span>
        </div>
      </div>
      <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 13px; font-weight: 600;">Meta reply window clock</span>
          <span class="pill pill-accent">${ICONS.clock} 22h 05m AI · 6d 22h human</span>
        </div>
        <div style="height: 8px; border-radius: 99px; background: var(--bg-muted); overflow: hidden;">
          <div style="width: 84%; height: 100%; background: var(--accent); border-radius: 99px;"></div>
        </div>
        <div style="font-size: 12px; color: var(--fg-2);">Long answers automatically split across Messenger messages in order without losing text.</div>
      </div>
    </div>`,
  );
}

function facebookAgentsHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 14px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span class="chip-ai">AI</span>
          <span style="font-size: 14.5px; font-weight: 600;">Sky · Facebook Messenger</span>
        </div>
        <span class="pill pill-amber">${ICONS.clock} SLA 12m remaining</span>
      </div>
      <div class="card-sub" style="padding: 13px 15px; font-size: 13px; line-height: 1.5;">
        <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;">Customer on Stowline Page</div>
        My UPS label scanned as delivered in Austin, TX, but nothing arrived at our warehouse.
      </div>
      <div class="card" style="padding: 14px 16px; border-color: var(--accent-border); display: flex; flex-direction: column; gap: 9px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span class="pill pill-purple" style="font-size: 11.5px;">≡ Procedure: Missing delivered parcel</span>
          <span class="pill pill-accent">Step 2 of 3</span>
        </div>
        <div style="font-size: 13px; line-height: 1.5;">
          I checked UPS tracking <span class="mono" style="font-size: 12px;">1Z999AA1</span> — the driver left it at Loading Dock B with proof-of-delivery photo. If it isn't there, reply here and I'll open a UPS trace with our team immediately.
        </div>
      </div>
      <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
        <span>Workflow: <strong>Tag "missing-parcel" + start 15m SLA on escalation</strong></span>
      </div>
    </div>`,
  );
}

function apiHeroHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div class="app-shell" style="gap: 12px;">
      <!-- Left: Custom native app / headless client calling /widget/v1 -->
      <div class="pane" style="width: 600px; flex-shrink: 0;">
        <div class="pane-header" style="background: var(--bg-sub);">
          <div style="display: flex; align-items: center; gap: 9px;">
            <span class="pill pill-blue mono" style="font-size: 11px;">POST /widget/v1</span>
            <span style="font-size: 14px; font-weight: 600;">Custom Mobile / Web UI</span>
          </div>
          <span class="pill pill-accent"><span class="dot dot-green"></span>SSE stream connected</span>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px;">
          <!-- Phone / custom app frame -->
          <div class="card" style="padding: 16px 18px; display: flex; flex-direction: column; gap: 12px; flex: 1;">
            <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 10px; border-bottom: 1px solid var(--border);">
              <span style="font-size: 14px; font-weight: 600;">Your native Support screen</span>
              <span class="mono" style="font-size: 11px; color: var(--fg-3);">Zero iframe · Zero cookies</span>
            </div>
            <div style="align-self: flex-end; max-width: 82%; padding: 10px 14px; border-radius: 13px 13px 4px 13px; background: var(--bg-bubble-out); color: var(--fg-bubble-out); font-size: 13.5px; line-height: 1.45;">
              Can I increase our webhook concurrency limit to 50 req/s?
            </div>
            <div id="api-r1" style="align-self: flex-start; max-width: 86%; padding: 11px 14px; border-radius: 4px 13px 13px 13px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; line-height: 1.45; opacity: 0;">
              <div style="font-size: 11px; font-weight: 600; color: var(--accent-fg); margin-bottom: 3px; display: flex; align-items: center; gap: 5px;"><span class="chip-ai">AI</span> Sky · Streamed via /widget/v1/stream</div>
              Scale workspaces support up to 100 req/s. I've flagged your account for Priya to bump the limit now.
            </div>
            <div id="api-r2" style="align-self: flex-start; max-width: 86%; padding: 11px 14px; border-radius: 4px 13px 13px 13px; background: var(--bg-bubble-in); font-size: 13.5px; line-height: 1.45; opacity: 0;">
              <div style="font-size: 11px; font-weight: 600; color: var(--fg-2); margin-bottom: 3px;">Priya Moreau · Teammate</div>
              Limit raised to 50 req/s on <span class="mono" style="font-size: 12px;">org_northwind</span> — live right away!
            </div>
          </div>
          <!-- Live HTTP / Stream inspector -->
          <div class="card" style="padding: 12px 14px; background: var(--code-bg); color: var(--code-fg);">
            <div class="mono" style="font-size: 11.5px; line-height: 1.6;">
              <div><span style="color: #34d399;">201</span> POST /widget/v1/conversations/conv_84a/messages <span style="color: var(--fg-3);">JWT HS256</span></div>
              <div id="api-log2" style="opacity: 0;"><span style="color: #60a5fa;">SSE</span> event: message.created · author: Sky (ai) · seq: 14</div>
              <div id="api-log3" style="opacity: 0;"><span style="color: #60a5fa;">SSE</span> event: message.created · author: Priya · seq: 15</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Right: dimmi inbox -->
      ${renderRail("inbox")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="font-size: 15px; font-weight: 600;">Webhook concurrency limit</span>
            <span class="pill pill-blue">${ICONS.code} Headless /widget/v1</span>
          </div>
          <span class="pill pill-accent">${ICONS.lock} JWT verified</span>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px;">
          <div class="card-sub" style="padding: 13px 15px; font-size: 13.5px;">
            <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;"><strong style="color: var(--fg);">Sven Larsen</strong> · iOS App · <span class="mono">plan:scale</span></div>
            Can I increase our webhook concurrency limit to 50 req/s?
          </div>
          <div id="api-dm1" style="align-self: flex-end; max-width: 82%; padding: 12px 15px; border-radius: 12px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; opacity: 0;">
            <div style="font-size: 11px; font-weight: 600; color: var(--accent-fg); margin-bottom: 3px;"><span class="chip-ai">AI</span> Sky · Handed over to Priya</div>
            Scale workspaces support up to 100 req/s. I've flagged your account for Priya to bump the limit now.
          </div>
          <div id="api-dm2" style="align-self: flex-end; max-width: 82%; padding: 12px 15px; border-radius: 12px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 13.5px; opacity: 0;">
            <div style="font-size: 11px; font-weight: 600; color: var(--fg-2); margin-bottom: 3px;">Priya Moreau → Delivered over realtime stream</div>
            Limit raised to 50 req/s on <span class="mono" style="font-size: 12px;">org_northwind</span> — live right away!
          </div>
        </div>
        <div style="padding: 14px 18px; border-top: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center;">
          <span style="font-size: 13px; color: var(--fg-3);">Same SLAs, workflows, and AI agents on headless threads</span>
          <span class="btn btn-primary btn-sm">Send reply</span>
        </div>
      </main>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('api-dm1', t, 0.5, 0.35);
      showPop('api-r1', t, 0.65, 0.35);
      showFade('api-log2', t, 0.65, 0.25);
      showPop('api-dm2', t, 2.05, 0.38);
      showPop('api-r2', t, 2.2, 0.38);
      showFade('api-log3', t, 2.2, 0.25);
    };
    window.__setTime(3.6);`,
  );
}

function apiRestHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 20px 22px; display: flex; flex-direction: column; gap: 13px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 10px; border-bottom: 1px solid var(--border);">
        <span style="font-size: 14.5px; font-weight: 600;">Built-in API channel · /api/v1</span>
        <span class="pill pill-accent">Scoped token</span>
      </div>
      <pre class="mono" style="padding: 13px 15px; border-radius: 11px; background: var(--code-bg); color: var(--code-fg); border: 1px solid var(--border); font-size: 12px; line-height: 1.55; overflow: hidden;"><span style="color: var(--purple-fg);">await</span> dimmi.conversations.<span style="color: var(--blue-fg);">create</span>({
  inboxId: <span style="color: var(--accent-fg);">"inb_support"</span>,
  customer: { email: <span style="color: var(--accent-fg);">"ops@northwind.io"</span> },
  message: <span style="color: var(--accent-fg);">"Carrier webhook failed 3x on #STW-4829"</span>,
});</pre>
      <div id="ar-1" class="card" style="padding: 12px 14px; border-color: var(--accent-border); font-size: 12.5px; opacity: 0;">
        <div style="display: flex; justify-content: space-between; margin-bottom: 4px;">
          <strong style="color: var(--accent-fg);">Opened in Support Inbox · #conv_912</strong>
          <span class="pill pill-blue">API channel</span>
        </div>
        Priya Moreau replied: "Replayed failed deliveries and cleared the queue."
      </div>
      <div id="ar-2" class="card-sub" style="padding: 11px 14px; font-size: 12px; display: flex; align-items: center; justify-content: space-between; opacity: 0;">
        <span class="mono">webhook → message.created (evt_01J9...)</span>
        <span class="pill pill-accent">200 OK · 38ms</span>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('ar-1', t, 0.7, 0.38);
      showPop('ar-2', t, 2.0, 0.38);
    };
    window.__setTime(3.2);`,
  );
}

function apiMessengerHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Messenger API (/widget/v1)</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Headless web, iOS, Android, and React Native chat</div>
        </div>
        <span class="pill pill-accent">Active</span>
      </div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
        <div class="card-sub" style="padding: 12px 14px;">
          <div style="font-size: 11.5px; color: var(--fg-3);">App ID</div>
          <div class="mono" style="font-size: 13px; font-weight: 600; margin-top: 4px;">app_stw_live_9f2a</div>
        </div>
        <div class="card-sub" style="padding: 12px 14px;">
          <div style="font-size: 11.5px; color: var(--fg-3);">Auth modes</div>
          <div style="font-size: 13px; font-weight: 600; margin-top: 4px;">HS256 · Ed25519 · JWKS</div>
        </div>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 13px 15px; display: flex; align-items: center; justify-content: space-between;">
          <div>
            <div style="font-size: 13.5px; font-weight: 600;">Native mobile app access</div>
            <div style="font-size: 12px; color: var(--fg-3); margin-top: 2px;">Allow iOS &amp; Android clients to call /widget/v1 without browser cookies</div>
          </div>
          <span class="switch on"></span>
        </div>
        <div style="padding: 13px 15px; border-top: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between;">
          <div>
            <div style="font-size: 13.5px; font-weight: 600;">Require signed customer JWT</div>
            <div style="font-size: 12px; color: var(--fg-3); margin-top: 2px;">Merge anonymous sessions and trust verified attributes</div>
          </div>
          <span class="switch on"></span>
        </div>
      </div>
      <div style="font-size: 12px; color: var(--fg-3);">Allowed origins: <span class="mono" style="color: var(--fg-2);">https://app.stowline.co</span> · <span class="mono" style="color: var(--fg-2);">stowline://app</span></div>
    </div>`,
  );
}

function apiWebhooksHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Signed webhooks</div>
          <div class="mono" style="font-size: 12px; color: var(--fg-3); margin-top: 2px;">https://api.stowline.co/hooks/dimmi</div>
        </div>
        <span class="pill pill-accent">${ICONS.shield} Standard Webhooks</span>
      </div>
      <div class="card-sub" style="padding: 12px 14px; display: flex; align-items: center; justify-content: space-between;">
        <div>
          <div style="font-size: 12px; font-weight: 600;">Subscribed events (wildcards)</div>
          <div style="display: flex; gap: 6px; margin-top: 6px;">
            <span class="pill pill-blue mono" style="font-size: 11px;">conversation.*</span>
            <span class="pill pill-purple mono" style="font-size: 11px;">message.created</span>
            <span class="pill pill-muted mono" style="font-size: 11px;">ticket.updated</span>
          </div>
        </div>
        <span class="btn btn-sm">Rotate secret</span>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 9px 14px; background: var(--bg-sub); border-bottom: 1px solid var(--border); font-size: 11.5px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">Recent deliveries (12 retries over 24h)</div>
        ${[
          ["message.created", "evt_01J9K4", "200 OK · 41ms", "pill-accent"],
          ["conversation.assigned", "evt_01J9K2", "200 OK · 36ms", "pill-accent"],
          ["conversation.closed", "evt_01J9JX", "Retry 2 → 200 OK", "pill-amber"],
        ]
          .map(
            (
              [ev, id, st, cls],
              idx,
            ) => `<div style="padding: 11px 14px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""} display: flex; align-items: center; justify-content: space-between; font-size: 12.5px;">
            <div>
              <span class="mono" style="font-weight: 600;">${ev}</span>
              <span class="mono" style="font-size: 11px; color: var(--fg-3); margin-left: 8px;">${id}</span>
            </div>
            <span class="pill ${cls}">${st}</span>
          </div>`,
          )
          .join("")}
      </div>
    </div>`,
  );
}

function apiSdksHtml(scheme: Scheme): string {
  const sdks = [
    "TypeScript",
    "Go",
    "PHP",
    "Laravel",
    "Python",
    "Browser",
    "React",
    "Next.js",
    "Vue",
    "Angular",
    "Svelte",
  ];
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="font-size: 15px; font-weight: 600;">11 SDKs, OAuth apps &amp; MCP</div>
        <span class="pill pill-accent">Typed packages</span>
      </div>
      <div style="display: flex; flex-wrap: wrap; gap: 7px;">
        ${sdks
          .map(
            (sdk) =>
              `<span style="padding: 6px 11px; border-radius: 8px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12.5px; font-weight: 500;">${sdk}</span>`,
          )
          .join("")}
      </div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
        <div class="card" style="padding: 14px 15px; display: flex; flex-direction: column; gap: 6px;">
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <span style="font-size: 13px; font-weight: 600;">Remote MCP server</span>
            <span class="pill pill-accent">OAuth 2</span>
          </div>
          <div class="mono" style="font-size: 11.5px; color: var(--accent-fg);">https://app.trydimmi.com/mcp</div>
          <div style="font-size: 12px; color: var(--fg-2);">Claude, Cursor &amp; ChatGPT with write confirmation</div>
        </div>
        <div class="card" style="padding: 14px 15px; display: flex; flex-direction: column; gap: 6px;">
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <span style="font-size: 13px; font-weight: 600;">API &amp; Webhook docs</span>
            <span class="pill pill-blue">OpenAPI</span>
          </div>
          <div class="mono" style="font-size: 11.5px; color: var(--blue-fg);">https://trydimmi.com/docs</div>
          <div style="font-size: 12px; color: var(--fg-2);">Updates never break existing API calls</div>
        </div>
      </div>
      <div class="card-sub" style="padding: 11px 14px; font-size: 12px; color: var(--fg-2); display: flex; justify-content: space-between;">
        <span>OAuth 2 apps: <strong>Stowline Internal Bridge</strong> (PKCE · 4 scopes)</span>
        <span class="pill pill-muted">Active</span>
      </div>
    </div>`,
  );
}

export const META_API_SCENES: SceneSpec[] = [
  {
    name: "channel-instagram-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.2,
    posterTime: 3.7,
    html: instagramHeroHtml,
  },
  {
    name: "channel-instagram-thread",
    width: 720,
    height: 640,
    kind: "video",
    duration: 3.6,
    posterTime: 3.1,
    html: instagramThreadHtml,
  },
  { name: "channel-instagram-connect", width: 720, height: 640, kind: "still", html: instagramConnectHtml },
  { name: "channel-instagram-media", width: 720, height: 640, kind: "still", html: instagramMediaHtml },
  { name: "channel-instagram-agents", width: 720, height: 640, kind: "still", html: instagramAgentsHtml },
  {
    name: "channel-facebook-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.0,
    posterTime: 3.4,
    html: facebookHeroHtml,
  },
  {
    name: "channel-facebook-thread",
    width: 720,
    height: 640,
    kind: "video",
    duration: 3.6,
    posterTime: 3.1,
    html: facebookThreadHtml,
  },
  { name: "channel-facebook-connect", width: 720, height: 640, kind: "still", html: facebookConnectHtml },
  { name: "channel-facebook-routing", width: 720, height: 640, kind: "still", html: facebookRoutingHtml },
  { name: "channel-facebook-agents", width: 720, height: 640, kind: "still", html: facebookAgentsHtml },
  {
    name: "channel-api-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.0,
    posterTime: 3.5,
    html: apiHeroHtml,
  },
  {
    name: "channel-api-rest",
    width: 720,
    height: 640,
    kind: "video",
    duration: 3.6,
    posterTime: 3.1,
    html: apiRestHtml,
  },
  { name: "channel-api-messenger", width: 720, height: 640, kind: "still", html: apiMessengerHtml },
  { name: "channel-api-webhooks", width: 720, height: 640, kind: "still", html: apiWebhooksHtml },
  { name: "channel-api-sdks", width: 720, height: 640, kind: "still", html: apiSdksHtml },
];
