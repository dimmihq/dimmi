import { doc, ICONS, renderRail, type SceneSpec, type Scheme, TIMELINE_SCRIPT_HELPER } from "./theme.ts";

function messengerHeroHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg-shell); display: flex; flex-direction: column; overflow: hidden; position: relative;">
      <!-- Host customer app chrome (Stowline Dashboard) -->
      <div style="height: 46px; background: var(--bg); border-bottom: 1px solid var(--border); padding: 0 22px; display: flex; align-items: center; justify-content: space-between;">
        <div style="display: flex; align-items: center; gap: 18px;">
          <div style="display: flex; gap: 7px;">
            <span style="width: 11px; height: 11px; border-radius: 999px; background: var(--border-strong);"></span>
            <span style="width: 11px; height: 11px; border-radius: 999px; background: var(--border-strong);"></span>
            <span style="width: 11px; height: 11px; border-radius: 999px; background: var(--border-strong);"></span>
          </div>
          <div class="mono" style="padding: 4px 12px; border-radius: 7px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12px; color: var(--fg-2);">
            https://app.stowline.co/shipments
          </div>
        </div>
        <div style="display: flex; align-items: center; gap: 10px; font-size: 12.5px; color: var(--fg-2);">
          <span class="pill pill-accent">${ICONS.lock} Signed in as Hana Kowalski (Growth plan)</span>
        </div>
      </div>

      <!-- Host app workspace behind the floating messenger -->
      <div style="flex: 1; display: flex; padding: 26px 32px; gap: 24px; position: relative;">
        <div style="width: 210px; display: flex; flex-direction: column; gap: 6px; padding-top: 6px;">
          <div style="display: flex; align-items: center; gap: 9px; padding: 6px 10px; margin-bottom: 12px;">
            <div style="width: 26px; height: 26px; border-radius: 7px; background: var(--fg); color: var(--bg); font-weight: 700; font-size: 13px; display: flex; align-items: center; justify-content: center;">S</div>
            <span style="font-weight: 600; font-size: 15px;">Stowline</span>
          </div>
          ${["Overview", "Shipments", "Batch labels", "Carriers", "Returns", "Billing"]
            .map(
              (item, i) =>
                `<div style="padding: 9px 12px; border-radius: 9px; font-size: 13.5px; font-weight: ${i === 1 ? "600" : "500"}; background: ${i === 1 ? "var(--bg)" : "transparent"}; border: 1px solid ${i === 1 ? "var(--border)" : "transparent"}; color: ${i === 1 ? "var(--fg)" : "var(--fg-2)"};">${item}</div>`,
            )
            .join("")}
        </div>

        <div style="flex: 1; display: flex; flex-direction: column; gap: 18px; min-width: 0;">
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <div>
              <h2 style="font-size: 22px; font-weight: 600; letter-spacing: -0.02em;">Shipments</h2>
              <p style="font-size: 13.5px; color: var(--fg-3); margin-top: 2px;">142 labels created today · USPS &amp; UPS connected</p>
            </div>
            <span class="btn btn-primary">+ Create label</span>
          </div>

          <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px;">
            ${[
              ["In transit", "86", "+12 today"],
              ["Delivered", "1,294", "99.1% on time"],
              ["Carrier adjustments", "2", "Action needed"],
            ]
              .map(
                ([label, value, sub]) => `<div class="card" style="padding: 16px 18px;">
                <div style="font-size: 12px; color: var(--fg-3);">${label}</div>
                <div style="font-size: 26px; font-weight: 600; letter-spacing: -0.02em; margin-top: 4px;">${value}</div>
                <div style="font-size: 12px; color: var(--fg-2); margin-top: 3px;">${sub}</div>
              </div>`,
              )
              .join("")}
          </div>

          <div class="card" style="padding: 0; overflow: hidden; flex: 1;">
            <div style="display: grid; grid-template-columns: 1.1fr 1.5fr 1fr 1fr 0.8fr; padding: 12px 18px; background: var(--bg-sub); border-bottom: 1px solid var(--border); font-size: 11.5px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em;">
              <span>Tracking</span><span>Recipient</span><span>Carrier</span><span>Status</span><span>Label</span>
            </div>
            ${[
              ["#STW-4829", "Kieran Patel · Austin, TX", "USPS Priority", "Adjustment +$4.20", "pill-amber", "$11.40"],
              ["#STW-4828", "Nadia Okafor · Seattle, WA", "UPS Ground", "In transit", "pill-blue", "$14.80"],
              ["#STW-4827", "Lucas Berg · Denver, CO", "USPS Ground", "Delivered", "pill-accent", "$8.15"],
              ["#STW-4826", "Sofia Lind · Chicago, IL", "UPS 2nd Day", "In transit", "pill-blue", "$22.50"],
              ["#STW-4825", "Devon Brooks · Boston, MA", "USPS Priority", "Delivered", "pill-accent", "$10.90"],
              ["#STW-4824", "Mina Park · Portland, OR", "FedEx Home", "Label created", "pill-muted", "$16.20"],
            ]
              .map(
                (
                  [id, rec, car, st, cls, price],
                  idx,
                ) => `<div style="display: grid; grid-template-columns: 1.1fr 1.5fr 1fr 1fr 0.8fr; align-items: center; padding: 14px 18px; font-size: 13px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""}">
                <span class="mono" style="font-weight: 600;">${id}</span>
                <span style="color: var(--fg-2);">${rec}</span>
                <span style="color: var(--fg-2);">${car}</span>
                <span><span class="pill ${cls}">${st}</span></span>
                <span class="mono" style="color: var(--fg-2);">${price}</span>
              </div>`,
              )
              .join("")}
          </div>
        </div>

        <!-- Floating dimmi Web Messenger widget -->
        <div id="widget" style="position: absolute; right: 34px; bottom: 28px; width: 410px; height: 670px; border-radius: 20px; background: var(--bg); border: 1px solid var(--border-strong); box-shadow: var(--shadow-float); display: flex; flex-direction: column; overflow: hidden; z-index: 10;">
          <div style="padding: 18px 20px; background: linear-gradient(135deg, #059669 0%, #047857 100%); color: #ffffff; display: flex; align-items: center; justify-content: space-between;">
            <div style="display: flex; align-items: center; gap: 11px;">
              <div style="width: 34px; height: 34px; border-radius: 10px; background: rgba(255,255,255,0.18); display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 15px;">S</div>
              <div>
                <div style="font-size: 15px; font-weight: 600; display: flex; align-items: center; gap: 6px;">Stowline Support <span style="padding: 1px 6px; border-radius: 5px; background: rgba(255,255,255,0.2); font-size: 10.5px; font-weight: 600;">Verified</span></div>
                <div style="font-size: 12px; opacity: 0.88;">Sky (AI) &amp; team · replies in under 1m</div>
              </div>
            </div>
            <span style="width: 28px; height: 28px; border-radius: 8px; background: rgba(255,255,255,0.15); display: flex; align-items: center; justify-content: center; font-size: 14px;">✕</span>
          </div>

          <!-- Context banner -->
          <div style="padding: 9px 16px; background: var(--bg-sub); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; font-size: 11.5px; color: var(--fg-2);">
            <span>JWT verified · <strong>Hana Kowalski</strong></span>
            <span class="mono" style="color: var(--accent-fg);">plan:growth · #STW-4829</span>
          </div>

          <!-- Messages -->
          <div style="flex: 1; padding: 16px 18px; display: flex; flex-direction: column; gap: 12px; overflow: hidden;">
            <div id="m1" style="align-self: flex-end; max-width: 82%; padding: 11px 14px; border-radius: 14px 14px 4px 14px; background: var(--bg-bubble-out); color: var(--fg-bubble-out); font-size: 13.5px; line-height: 1.45;">
              Why did USPS add a $4.20 adjustment to shipment #STW-4829?
            </div>

            <div id="m-typing" style="align-self: flex-start; padding: 9px 13px; border-radius: 14px; background: var(--bg-bubble-in); font-size: 12px; color: var(--fg-2); display: flex; align-items: center; gap: 8px;">
              <span class="chip-ai">AI</span>
              <span>Sky is checking parcel #STW-4829 &amp; docs...</span>
            </div>

            <div id="m2" style="align-self: flex-start; max-width: 88%; display: flex; flex-direction: column; gap: 6px; opacity: 0;">
              <div style="display: flex; align-items: center; gap: 6px; font-size: 11.5px; color: var(--fg-3);">
                <span class="avatar avatar-sm av-emerald">${ICONS.sparkle}</span>
                <strong style="color: var(--fg);">Sky</strong>
                <span class="chip-ai">AI</span>
              </div>
              <div style="padding: 12px 14px; border-radius: 4px 14px 14px 14px; background: var(--bg-bubble-in); font-size: 13.5px; line-height: 1.5; color: var(--fg);">
                USPS re-weighed <strong>#STW-4829</strong> at 2 lb 4 oz (label was 1 lb 12 oz). You can dispute this within 30 days by replying with a photo of the box on a scale beside a tape measure.
                <div style="margin-top: 9px; padding: 8px 10px; border-radius: 9px; background: var(--bg); border: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; font-size: 12px;">
                  <span style="display: flex; align-items: center; gap: 6px; font-weight: 500;">${ICONS.book} Carrier weight disputes</span>
                  <span style="color: var(--accent-fg); font-weight: 500;">Read →</span>
                </div>
              </div>
            </div>

            <div id="m3" style="align-self: flex-end; max-width: 82%; padding: 10px 14px; border-radius: 14px 14px 4px 14px; background: var(--bg-bubble-out); color: var(--fg-bubble-out); font-size: 13.5px; line-height: 1.45; opacity: 0;">
              Got it, I have the scale photo ready. Can a teammate file it?
            </div>

            <div id="m4" style="align-self: flex-start; max-width: 88%; display: flex; flex-direction: column; gap: 5px; opacity: 0;">
              <div style="display: flex; align-items: center; gap: 6px; font-size: 11.5px; color: var(--fg-3);">
                <span class="avatar avatar-sm av-rose">P</span>
                <strong style="color: var(--fg);">Priya Moreau</strong>
                <span>joined with summary</span>
              </div>
              <div style="padding: 11px 14px; border-radius: 4px 14px 14px 14px; background: var(--accent-soft); border: 1px solid var(--accent-border); font-size: 13.5px; line-height: 1.45; color: var(--fg);">
                Hi Hana! Drop the photo right here and I'll submit the USPS dispute for #STW-4829 now.
              </div>
            </div>
          </div>

          <!-- Composer + tab bar -->
          <div style="padding: 12px 14px; border-top: 1px solid var(--border); background: var(--bg);">
            <div style="padding: 9px 12px; border-radius: 10px; border: 1px solid var(--border); background: var(--bg-sub); display: flex; align-items: center; justify-content: space-between; font-size: 13px; color: var(--fg-3);">
              <span id="comp-text">Write a reply or drop a photo...</span>
              <span style="color: var(--accent);">${ICONS.send}</span>
            </div>
            <div style="display: grid; grid-template-columns: repeat(4, 1fr); margin-top: 10px; text-align: center; font-size: 11px; color: var(--fg-3);">
              <span>Home</span>
              <span style="color: var(--accent-fg); font-weight: 600;">Messages</span>
              <span>Help</span>
              <span>Changelog <span class="dot dot-green" style="width:6px;height:6px;margin-left:2px;"></span></span>
            </div>
          </div>
        </div>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('m1', t, 0.1, 0.3);
      setDisplay('m-typing', t >= 0.45 && t < 1.55);
      showPop('m2', t, 1.55, 0.38);
      showPop('m3', t, 2.7, 0.35);
      showPop('m4', t, 3.65, 0.38);
    };
    window.__setTime(4.2);`,
  );
}

function messengerHomeHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg-shell); display: flex; align-items: flex-start; justify-content: center; padding: 20px 26px;">
      <div style="width: 440px; height: 590px; border-radius: 18px; background: var(--bg); border: 1px solid var(--border-strong); box-shadow: var(--shadow-float); display: flex; flex-direction: column; overflow: hidden;">
        <div style="padding: 20px 22px 18px; background: linear-gradient(135deg, #059669 0%, #047857 100%); color: #fff;">
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 14px;">
            <div style="width: 30px; height: 30px; border-radius: 8px; background: rgba(255,255,255,0.2); display: flex; align-items: center; justify-content: center; font-weight: 700;">S</div>
            <div style="display: flex; align-items: center; gap: -6px;">
              <span class="avatar avatar-sm av-rose" style="border: 2px solid #059669;">P</span>
              <span class="avatar avatar-sm av-sky" style="border: 2px solid #059669; margin-left: -6px;">M</span>
              <span class="avatar avatar-sm av-amber" style="border: 2px solid #059669; margin-left: -6px;">K</span>
            </div>
          </div>
          <div style="font-size: 20px; font-weight: 600; letter-spacing: -0.02em;">Hi Hana, how can we help?</div>
          <div style="font-size: 13px; opacity: 0.88; margin-top: 3px;">Stowline Support · Online now</div>
        </div>

        <div style="flex: 1; padding: 14px 16px; display: flex; flex-direction: column; gap: 11px; overflow: hidden; background: var(--bg-sub);">
          <!-- Start conversation CTA -->
          <div id="home-cta" class="card" style="padding: 13px 15px; display: flex; align-items: center; justify-content: space-between;">
            <div>
              <div style="font-size: 13.5px; font-weight: 600;">Start a conversation</div>
              <div style="font-size: 12px; color: var(--fg-3); margin-top: 2px;">Sky (AI) replies instantly · Teammates in ~2m</div>
            </div>
            <span style="width: 32px; height: 32px; border-radius: 9px; background: var(--accent); color: #fff; display: flex; align-items: center; justify-content: center;">${ICONS.send}</span>
          </div>

          <!-- Search docs card -->
          <div id="home-search" class="card" style="padding: 13px 15px; display: flex; flex-direction: column; gap: 9px;">
            <div style="display: flex; align-items: center; gap: 8px; padding: 8px 11px; border-radius: 9px; background: var(--bg-sub); border: 1px solid var(--border); font-size: 12.5px; color: var(--fg-2);">
              ${ICONS.search}
              <span id="search-input">bulk void</span>
            </div>
            <div id="search-res-1" style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; padding: 4px 2px;">
              <span style="font-weight: 500;">Voiding USPS labels in bulk</span>
              <span style="color: var(--fg-3);">→</span>
            </div>
            <div id="search-res-2" style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; padding: 4px 2px; border-top: 1px solid var(--border); padding-top: 8px;">
              <span style="font-weight: 500;">Carrier weight dispute checklist</span>
              <span style="color: var(--fg-3);">→</span>
            </div>
          </div>

          <!-- Changelog card with unread dot -->
          <div id="home-changelog" class="card" style="padding: 13px 15px; display: flex; flex-direction: column; gap: 6px; border-color: var(--accent-border);">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span class="pill pill-accent"><span class="dot dot-green"></span>New release · Today</span>
              <span style="font-size: 11.5px; color: var(--fg-3);">Changelog</span>
            </div>
            <div style="font-size: 13.5px; font-weight: 600;">14-day USPS bulk label voiding</div>
            <div style="font-size: 12px; color: var(--fg-2); line-height: 1.4;">Void up to 50 unused USPS labels at once right from Shipments → Labels.</div>
          </div>

          <!-- Feature requests -->
          <div id="home-roadmap" class="card" style="padding: 11px 15px; display: flex; align-items: center; justify-content: space-between;">
            <span style="font-size: 13px; font-weight: 500;">Feature requests &amp; roadmap</span>
            <span class="pill pill-muted">18 open</span>
          </div>
        </div>

        <!-- Bottom tab bar -->
        <div style="height: 54px; border-top: 1px solid var(--border); background: var(--bg); display: grid; grid-template-columns: repeat(4, 1fr); align-items: center; text-align: center; font-size: 11.5px;">
          <span id="tab-home" style="color: var(--accent-fg); font-weight: 600;">Home</span>
          <span id="tab-msg" style="color: var(--fg-3);">Messages</span>
          <span id="tab-help" style="color: var(--fg-3);">Help</span>
          <span id="tab-log" style="color: var(--fg-3); display: inline-flex; align-items: center; justify-content: center; gap: 4px;">Changelog <span id="unread-dot" class="dot dot-green"></span></span>
        </div>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('home-cta', t, 0.05, 0.3);
      showPop('home-search', t, 0.35, 0.35);
      typeText('search-input', 'bulk void labels', t, 0.9, 1.9);
      showPop('search-res-1', t, 1.75, 0.3);
      showPop('search-res-2', t, 2.05, 0.3);
      showPop('home-changelog', t, 2.55, 0.38);
      showPop('home-roadmap', t, 3.15, 0.35);
    };
    window.__setTime(4.0);`,
  );
}

function messengerInstallHtml(scheme: Scheme): string {
  const tabs = ["HTML", "Browser", "React", "Next.js", "Vue", "Angular", "Svelte"];
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 16px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Install web messenger</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Stowline App · <span class="mono">app_stw_live_9f2a</span></div>
        </div>
        <span class="pill pill-accent"><span class="dot dot-green"></span>Active</span>
      </div>
      <div style="display: flex; flex-wrap: wrap; gap: 6px;">
        ${tabs
          .map(
            (tab, i) =>
              `<span style="padding: 6px 11px; border-radius: 8px; font-size: 12.5px; font-weight: ${i === 3 ? "600" : "500"}; background: ${i === 3 ? "var(--accent-soft)" : "var(--bg-sub)"}; color: ${i === 3 ? "var(--accent-fg)" : "var(--fg-2)"}; border: 1px solid ${i === 3 ? "var(--accent-border)" : "var(--border)"};">${tab}</span>`,
          )
          .join("")}
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 10px 14px; background: var(--bg-sub); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; font-size: 12px;">
          <span class="mono" style="color: var(--fg-2);">app/layout.tsx</span>
          <span class="btn btn-sm">${ICONS.copy} Copy snippet</span>
        </div>
        <pre class="mono" style="padding: 16px; background: var(--code-bg); color: var(--code-fg); font-size: 12.5px; line-height: 1.65; overflow: hidden;"><span style="color: var(--fg-3);">// pnpm add @dimmihq/next</span>
<span style="color: var(--purple-fg);">import</span> { DimmiMessenger } <span style="color: var(--purple-fg);">from</span> <span style="color: var(--accent-fg);">"@dimmihq/next"</span>;

<span style="color: var(--purple-fg);">export default function</span> RootLayout({ children }) {
  <span style="color: var(--purple-fg);">return</span> (
    &lt;html&gt;&lt;body&gt;
      {children}
      &lt;<span style="color: var(--blue-fg);">DimmiMessenger</span>
        appId=<span style="color: var(--accent-fg);">"app_stw_live_9f2a"</span>
        tokenEndpoint=<span style="color: var(--accent-fg);">"/api/dimmi-token"</span>
      /&gt;
    &lt;/body&gt;&lt;/html&gt;
  );
}</pre>
      </div>
      <div class="card-sub" style="padding: 12px 14px; display: flex; align-items: center; justify-content: space-between;">
        <div>
          <div style="font-size: 12.5px; font-weight: 600;">Or paste one HTML script tag</div>
          <div class="mono" style="font-size: 11.5px; color: var(--fg-2); margin-top: 3px;">&lt;script async src="https://app.trydimmi.com/w/app_stw_live_9f2a.js"&gt;&lt;/script&gt;</div>
        </div>
        <span class="pill pill-muted">14 KB gz</span>
      </div>
      <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
        <span>2 messengers on this inbox: <strong>Stowline App</strong> · <strong>Marketing site</strong></span>
      </div>
    </div>`,
  );
}

function messengerIdentityHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Identity verification</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Require signed JWTs before loading account attributes</div>
        </div>
        <span class="pill pill-accent">${ICONS.shield} Enforced</span>
      </div>
      <div class="card-sub" style="padding: 13px 15px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 12.5px; font-weight: 600;">HS256 signing secret</span>
          <span class="btn btn-sm">Rotate (7d overlap)</span>
        </div>
        <div class="mono" style="padding: 8px 11px; border-radius: 8px; background: var(--bg); border: 1px solid var(--border); font-size: 12px; color: var(--fg-2); display: flex; justify-content: space-between;">
          <span>whsec_stw_live_••••••••••••••••8f3c</span>
          <span style="color: var(--accent-fg);">Primary</span>
        </div>
        <div style="font-size: 12px; color: var(--fg-3);">Or verify automatically via JWKS URL (Clerk, Auth.js, Supabase, Better Auth)</div>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 10px 14px; background: var(--bg-sub); border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; font-size: 12px;">
          <span style="font-weight: 600;">Token debugger</span>
          <span class="pill pill-accent">${ICONS.check} Signature valid · HS256</span>
        </div>
        <div style="padding: 14px 16px; display: grid; grid-template-columns: 1fr 1fr; gap: 10px; font-size: 12.5px;">
          ${[
            ["sub", "usr_92817"],
            ["email", "hana@northwind.io"],
            ["plan", "growth"],
            ["mrr_cents", "29900"],
            ["shipments_30d", "1,420"],
            ["exp", "in 54 minutes"],
          ]
            .map(
              ([
                k,
                v,
              ]) => `<div style="padding: 7px 10px; border-radius: 8px; background: var(--bg-sub); border: 1px solid var(--border); display: flex; justify-content: space-between;">
              <span class="mono" style="color: var(--fg-3);">${k}</span>
              <span class="mono" style="font-weight: 500;">${v}</span>
            </div>`,
            )
            .join("")}
        </div>
      </div>
      <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
        <span>Unsigned browser calls rejected on <span class="mono">app.stowline.co</span></span>
      </div>
    </div>`,
  );
}

function messengerBrandHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="font-size: 15px; font-weight: 600;">Appearance &amp; security</div>
        <span class="pill pill-muted">Light &amp; dark auto</span>
      </div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
        <div class="card-sub" style="padding: 12px 14px;">
          <div style="font-size: 12px; color: var(--fg-3); margin-bottom: 8px;">Brand accent</div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <span style="width: 24px; height: 24px; border-radius: 7px; background: #059669; box-shadow: 0 0 0 2px var(--bg), 0 0 0 4px #059669;"></span>
            <span style="width: 24px; height: 24px; border-radius: 7px; background: #2563eb;"></span>
            <span style="width: 24px; height: 24px; border-radius: 7px; background: #7c3aed;"></span>
            <span style="width: 24px; height: 24px; border-radius: 7px; background: #18181b;"></span>
            <span class="mono" style="font-size: 12px; color: var(--fg-2); margin-left: 4px;">#059669</span>
          </div>
        </div>
        <div class="card-sub" style="padding: 12px 14px;">
          <div style="font-size: 12px; color: var(--fg-3); margin-bottom: 8px;">Theme mode</div>
          <div style="display: flex; gap: 6px;">
            <span class="pill pill-muted">Light</span>
            <span class="pill pill-muted">Dark</span>
            <span class="pill pill-accent">Match app</span>
          </div>
        </div>
      </div>
      <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 13px; font-weight: 600;">Allowed sites &amp; strict CSP</span>
          <span class="pill pill-accent">CSP nonce ready</span>
        </div>
        <div style="display: flex; flex-wrap: wrap; gap: 6px;">
          <span class="pill pill-muted mono" style="font-size: 11.5px;">https://app.stowline.co</span>
          <span class="pill pill-muted mono" style="font-size: 11.5px;">https://help.stowline.co</span>
          <span class="pill pill-muted mono" style="font-size: 11.5px;">strict-dynamic ✓</span>
        </div>
      </div>
      <div class="card-sub" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <div style="display: flex; align-items: center; gap: 7px; font-size: 13px; font-weight: 600;">
            <span>AI disclosure badge</span>
            <span class="chip-ai">AI</span>
          </div>
          <span class="pill pill-muted">${ICONS.lock} Always on</span>
        </div>
        <div style="font-size: 12.5px; color: var(--fg-2); line-height: 1.45;">
          Every AI agent reply displays a localized <strong>AI</strong> badge and hands over to teammates with a private summary.
        </div>
      </div>
    </div>`,
  );
}

function emailHeroHtml(scheme: Scheme): string {
  const isDark = scheme === "dark";
  const gmBg = isDark ? "#18191c" : "#ffffff";
  const gmTop = isDark ? "#202124" : "#f6f8fc";
  const gmBorder = isDark ? "#2d2f34" : "#e2e7ef";
  const gmPill = isDark ? "#2a2b30" : "#eaf1fb";
  return doc(
    scheme,
    `<div class="app-shell" style="gap: 12px;">
      <!-- Left: Customer Gmail client -->
      <div class="pane" style="width: 610px; flex-shrink: 0; background: ${gmBg}; border-color: ${gmBorder};">
        <div style="height: 54px; padding: 0 18px; background: ${gmTop}; border-bottom: 1px solid ${gmBorder}; display: flex; align-items: center; justify-content: space-between; gap: 12px;">
          <div style="display: flex; align-items: center; gap: 10px;">
            ${ICONS.gmail}
            <span style="font-size: 15px; font-weight: 600; color: var(--fg);">Gmail</span>
          </div>
          <div style="flex: 1; max-width: 300px; height: 32px; border-radius: 999px; background: ${gmPill}; padding: 0 14px; display: flex; align-items: center; gap: 8px; font-size: 12.5px; color: var(--fg-2);">
            ${ICONS.search}
            <span>support@stowline.co</span>
          </div>
          <span class="avatar avatar-sm av-lime">O</span>
        </div>

        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 16px; overflow: hidden;">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="font-size: 18px; font-weight: 500; letter-spacing: -0.01em;">Invoice #4821 VAT number</span>
            <span style="padding: 2px 8px; border-radius: 5px; background: ${gmPill}; font-size: 11px; font-weight: 500; color: var(--fg-2);">Inbox</span>
          </div>

          <div style="padding-bottom: 16px; border-bottom: 1px solid ${gmBorder}; display: flex; flex-direction: column; gap: 10px;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <div style="display: flex; align-items: center; gap: 10px;">
                <span class="avatar av-lime">O</span>
                <div>
                  <div style="font-size: 13.5px; font-weight: 600;">Omar Haddad <span style="font-weight: 400; font-size: 12px; color: var(--fg-3);">&lt;omar@northwind.io&gt;</span></div>
                  <div style="font-size: 11.5px; color: var(--fg-3);">to support@stowline.co</div>
                </div>
              </div>
              <span style="font-size: 11.5px; color: var(--fg-3);">10:14 AM</span>
            </div>
            <p style="font-size: 13.5px; line-height: 1.55; padding-left: 38px; color: var(--fg);">
              Hi team, could you add our EU VAT ID (<span class="mono" style="font-size: 12.5px;">DE298410221</span>) to Invoice #4821 and resend the PDF? Finance closes books at 3 PM CET.
            </p>
          </div>

          <div id="gm-reply" style="display: flex; flex-direction: column; gap: 10px; opacity: 0;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <div style="display: flex; align-items: center; gap: 10px;">
                <span class="avatar av-emerald">S</span>
                <div>
                  <div style="font-size: 13.5px; font-weight: 600;">Priya from Stowline <span style="font-weight: 400; font-size: 12px; color: var(--fg-3);">&lt;support@stowline.co&gt;</span></div>
                  <div style="font-size: 11.5px; color: var(--fg-3);">to me · signed-by: stowline.co</div>
                </div>
              </div>
              <span style="font-size: 11.5px; color: var(--fg-3);">10:16 AM</span>
            </div>
            <div style="padding-left: 38px; display: flex; flex-direction: column; gap: 10px;">
              <p style="font-size: 13.5px; line-height: 1.55; color: var(--fg);">
                Hi Omar, I added <strong>DE298410221</strong> to your billing profile and attached the updated Invoice #4821 PDF. Future monthly invoices will include it automatically.
              </p>
              <div style="width: 225px; padding: 9px 11px; border-radius: 10px; border: 1px solid ${gmBorder}; background: ${gmTop}; display: flex; align-items: center; gap: 10px;">
                <span style="width: 28px; height: 28px; border-radius: 6px; background: #ea4335; color: #fff; font-size: 10px; font-weight: 700; display: flex; align-items: center; justify-content: center;">PDF</span>
                <div style="min-width: 0;">
                  <div style="font-size: 12px; font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">Invoice-4821-VAT.pdf</div>
                  <div style="font-size: 11px; color: var(--fg-3);">84 KB</div>
                </div>
              </div>
              <div style="display: flex; gap: 8px; padding-top: 4px;">
                <span style="padding: 6px 16px; border-radius: 999px; border: 1px solid ${gmBorder}; font-size: 12.5px; color: var(--fg-2);">↩ Reply</span>
                <span style="padding: 6px 16px; border-radius: 999px; border: 1px solid ${gmBorder}; font-size: 12.5px; color: var(--fg-2);">↪ Forward</span>
              </div>
            </div>
          </div>
        </div>

        <div style="padding: 12px 18px; border-top: 1px solid ${gmBorder}; background: ${gmTop}; font-size: 12px; color: var(--fg-3); display: flex; justify-content: space-between;">
          <span>Customer's inbox · Sent from support@stowline.co</span>
          <span class="pill pill-accent">${ICONS.shield} SPF · DKIM passed</span>
        </div>
      </div>

      <!-- Right: dimmi shared inbox -->
      ${renderRail("inbox")}
      <main class="pane" style="flex: 1;">
        <div class="pane-header">
          <div style="display: flex; align-items: center; gap: 10px; min-width: 0;">
            <span style="font-size: 15px; font-weight: 600;">Re: Invoice #4821 VAT number</span>
            <span class="pill pill-muted mono" style="font-size: 11px;">support@stowline.co</span>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="pill pill-amber">${ICONS.clock} 18m SLA left</span>
            <span class="btn btn-sm">Close</span>
          </div>
        </div>
        <div style="flex: 1; padding: 20px 22px; display: flex; flex-direction: column; gap: 14px; overflow: hidden;">
          <div class="card-sub" style="padding: 13px 15px;">
            <div style="display: flex; justify-content: space-between; font-size: 11.5px; color: var(--fg-3); margin-bottom: 5px;">
              <span><strong style="color: var(--fg);">Omar Haddad</strong> · Email · support@stowline.co</span>
              <span>10:14 AM</span>
            </div>
            <p style="font-size: 13.5px; line-height: 1.5;">Hi team, could you add our EU VAT ID (<span class="mono" style="font-size: 12px;">DE298410221</span>) to Invoice #4821 and resend the PDF? Finance closes books at 3 PM CET.</p>
          </div>

          <div id="email-note" style="align-self: flex-end; max-width: 82%; padding: 12px 15px; border-radius: 12px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 13px; line-height: 1.45; opacity: 0;">
            <div style="font-size: 11px; font-weight: 600; margin-bottom: 3px; display: flex; align-items: center; gap: 5px;">${ICONS.lock} Private note · Priya Moreau · Not visible to Omar</div>
            Updated VAT ID on Northwind's Stripe profile and regenerated PDF #4821. Attaching it now.
          </div>

          <div id="email-reply" class="card" style="padding: 14px 16px; border-color: var(--accent-border); background: var(--accent-soft); opacity: 0;">
            <div style="display: flex; justify-content: space-between; font-size: 11.5px; color: var(--accent-fg); font-weight: 600; margin-bottom: 5px;">
              <span>Priya Moreau → Sent from support@stowline.co</span>
              <span>Delivered</span>
            </div>
            <p style="font-size: 13.5px; line-height: 1.5;">Hi Omar, I added <strong>DE298410221</strong> to your billing profile and attached the updated Invoice #4821 PDF. Future monthly invoices will include it automatically.</p>
            <div style="margin-top: 8px;">
              <span style="display: inline-flex; align-items: center; gap: 6px; padding: 5px 9px; border-radius: 8px; background: var(--bg); border: 1px solid var(--border); font-size: 12px; font-weight: 500;">${ICONS.paperclip} Invoice-4821-VAT.pdf</span>
            </div>
          </div>

          <div id="undo-toast" style="align-self: center; padding: 7px 14px; border-radius: 999px; background: var(--fg); color: var(--bg); font-size: 12px; font-weight: 500; display: flex; align-items: center; gap: 10px; opacity: 0;">
            <span>Sending email from support@stowline.co...</span>
            <span style="color: #34d399; font-weight: 600;">Undo (5s)</span>
          </div>
        </div>

        <div style="padding: 14px 18px; border-top: 1px solid var(--border); background: var(--bg);">
          <div style="padding: 11px 14px; border-radius: 11px; border: 1px solid var(--border); background: var(--bg-sub); display: flex; justify-content: space-between; align-items: center; font-size: 13px; color: var(--fg-2);">
            <span id="email-composer">Reply from support@stowline.co...</span>
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="pill pill-muted">Note (N)</span>
              <span class="btn btn-primary btn-sm">Send reply</span>
            </div>
          </div>
        </div>
      </main>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('email-note', t, 0.45, 0.35);
      typeText('email-composer', 'Hi Omar, I added DE298410221 and attached Invoice-4821-VAT.pdf...', t, 1.1, 2.2);
      showPop('undo-toast', t, 2.35, 0.28);
      showPop('email-reply', t, 2.95, 0.38);
      showPop('gm-reply', t, 3.1, 0.38);
      if (t >= 2.95) {
        const el = document.getElementById('email-composer');
        if (el) el.textContent = 'Reply from support@stowline.co...';
      }
    };
    window.__setTime(3.85);`,
  );
}

function emailThreadHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 20px 22px; display: flex; flex-direction: column; gap: 13px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 14.5px; font-weight: 600;">Re: Bulk carrier invoice mismatch</div>
          <div style="font-size: 12px; color: var(--fg-3);">Claire Dupont &lt;claire@atelier.eu&gt; · billing@stowline.co</div>
        </div>
        <div id="collision-pill" class="pill pill-amber" style="opacity: 0;">
          <span class="avatar avatar-sm av-sky" style="width:16px;height:16px;font-size:9px;">M</span>
          Maya is viewing &amp; typing...
        </div>
      </div>

      <div class="card-sub" style="padding: 12px 14px; font-size: 13px; line-height: 1.5;">
        <div style="font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;">Claire Dupont · 9:48 AM</div>
        Our October UPS summary shows 412 labels, but the invoice billed 426. Can someone check?
      </div>

      <div id="et-note" style="padding: 12px 14px; border-radius: 11px; background: var(--bg-note); border: 1px solid var(--border-note); color: var(--fg-note); font-size: 13px; line-height: 1.45; opacity: 0;">
        <div style="font-size: 11px; font-weight: 600; margin-bottom: 3px;">${ICONS.lock} Internal note · Maya Lin</div>
        I've got this one. 14 labels were voided on Nov 1 so they credit on the November invoice. Replying now!
      </div>

      <div id="et-reply" class="card" style="padding: 12px 14px; border-color: var(--accent-border); font-size: 13px; line-height: 1.5; opacity: 0;">
        <div style="display: flex; justify-content: space-between; font-size: 11.5px; color: var(--fg-3); margin-bottom: 4px;">
          <span><strong style="color: var(--fg);">Maya Lin</strong> · billing@stowline.co</span>
          <span style="color: var(--accent-fg);">Delivered</span>
        </div>
        Hi Claire, 14 labels were voided after Oct 31, so the $118.40 credit is already queued on your November invoice.
      </div>

      <div style="padding: 11px 14px; border-radius: 11px; border: 1px solid var(--border); background: var(--bg-sub); display: flex; justify-content: space-between; align-items: center; font-size: 12.5px; color: var(--fg-3);">
        <span>Collision guard active · 1 teammate online</span>
        <span class="pill pill-muted">Customer: Atelier · Growth</span>
      </div>
    </div>`,
    "",
    `${TIMELINE_SCRIPT_HELPER}
    window.__setTime = function(t) {
      showPop('collision-pill', t, 0.3, 0.35);
      showPop('et-note', t, 1.15, 0.38);
      showPop('et-reply', t, 2.55, 0.4);
    };
    window.__setTime(3.6);`,
  );
}

function emailDomainHtml(scheme: Scheme): string {
  const records = [
    ["TXT (SPF)", "stowline.co", "v=spf1 include:mail.trydimmi.com ~all"],
    ["CNAME (DKIM)", "dimmi._domainkey.stowline.co", "dkim1.mail.trydimmi.com"],
    ["TXT (DMARC)", "_dmarc.stowline.co", "v=DMARC1; p=quarantine; rua=mailto:dmarc@stowline.co"],
  ];
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Sending domain · stowline.co</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Keep Google Workspace: forward in, send from your domain</div>
        </div>
        <span class="pill pill-accent">${ICONS.check} Verified</span>
      </div>
      <div class="card-sub" style="padding: 12px 14px; display: flex; align-items: center; justify-content: space-between;">
        <div>
          <div style="font-size: 12.5px; font-weight: 600;">Inbound forwarding test</div>
          <div class="mono" style="font-size: 11.5px; color: var(--fg-2); margin-top: 2px;">support@stowline.co → in+9f2a@mail.trydimmi.com</div>
        </div>
        <span class="pill pill-accent">${ICONS.check} Passed</span>
      </div>
      <div class="card" style="padding: 0; overflow: hidden;">
        <div style="padding: 10px 14px; background: var(--bg-sub); border-bottom: 1px solid var(--border); font-size: 11.5px; font-weight: 600; color: var(--fg-3); text-transform: uppercase; letter-spacing: 0.04em; display: flex; justify-content: space-between;">
          <span>DNS records (rechecked daily)</span>
          <span>Status</span>
        </div>
        ${records
          .map(
            (
              [type, host, val],
              idx,
            ) => `<div style="padding: 12px 14px; ${idx > 0 ? "border-top: 1px solid var(--border);" : ""} display: flex; align-items: center; justify-content: space-between; gap: 12px;">
            <div style="min-width: 0; flex: 1;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <span class="pill pill-muted mono" style="font-size: 10.5px;">${type}</span>
                <span class="mono" style="font-size: 12px; font-weight: 500;">${host}</span>
              </div>
              <div class="mono" style="font-size: 11px; color: var(--fg-3); margin-top: 5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${val}</div>
            </div>
            <div style="display: flex; align-items: center; gap: 6px;">
              <span class="pill pill-accent">${ICONS.check} Valid</span>
            </div>
          </div>`,
          )
          .join("")}
      </div>
    </div>`,
  );
}

function emailRoutingHtml(scheme: Scheme): string {
  const routes = [
    ["support@stowline.co", "Customer Support", "15m SLA", "Sky (AI) · Auto-reply", "pill-accent"],
    ["billing@stowline.co", "Finance & Billing", "1h SLA", "Billing workflow · Priya", "pill-blue"],
    ["security@stowline.co", "Security (Private)", "30m SLA", "Private queue · 3 leads", "pill-purple"],
  ];
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div>
          <div style="font-size: 15px; font-weight: 600;">Email addresses &amp; routing</div>
          <div style="font-size: 12.5px; color: var(--fg-3);">Connect unlimited addresses with dedicated inboxes &amp; SLAs</div>
        </div>
        <span class="btn btn-primary btn-sm">+ Add address</span>
      </div>
      <div style="display: flex; flex-direction: column; gap: 11px;">
        ${routes
          .map(
            ([
              addr,
              inbox,
              sla,
              agent,
              cls,
            ]) => `<div class="card" style="padding: 15px 16px; display: flex; flex-direction: column; gap: 10px;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span class="mono" style="font-size: 13.5px; font-weight: 600;">${addr}</span>
              <span class="pill ${cls}">${sla}</span>
            </div>
            <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; color: var(--fg-2);">
              <span>Inbox: <strong style="color: var(--fg);">${inbox}</strong></span>
              <span>${agent}</span>
            </div>
          </div>`,
          )
          .join("")}
      </div>
      <div class="card-sub" style="padding: 11px 14px; font-size: 12px; color: var(--fg-2); display: flex; justify-content: space-between;">
        <span>Bulk mail filter: <strong>Auto-close newsletters &amp; automated alerts</strong></span>
        <span class="switch on"></span>
      </div>
    </div>`,
  );
}

function emailSignatureHtml(scheme: Scheme): string {
  return doc(
    scheme,
    `<div style="width: 100%; height: 100%; background: var(--bg); padding: 24px 26px; display: flex; flex-direction: column; gap: 15px;">
      <div style="display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid var(--border);">
        <div style="font-size: 15px; font-weight: 600;">Reply style &amp; deliverability</div>
        <div style="display: flex; gap: 6px;">
          <span class="pill pill-accent">Plain personal</span>
          <span class="pill pill-muted">Branded card</span>
        </div>
      </div>
      <div class="card-sub" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
          <span>Signature template</span>
          <span>Variables supported</span>
        </div>
        <div class="mono" style="padding: 12px 14px; border-radius: 9px; background: var(--bg); border: 1px solid var(--border); font-size: 12.5px; line-height: 1.6;">
          Best,<br />
          <span class="pill pill-blue" style="padding: 0 6px; font-size: 11px;">{{teammate.name}}</span><br />
          <span class="pill pill-purple" style="padding: 0 6px; font-size: 11px;">{{workspace.name}}</span> · stowline.co
        </div>
      </div>
      <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-size: 13px; font-weight: 600;">Deliverability guard</span>
          <span class="pill pill-accent">${ICONS.shield} Healthy · 0.1% bounce</span>
        </div>
        <div style="font-size: 12.5px; color: var(--fg-2); line-height: 1.45;">
          Sending pauses automatically if hard bounces or spam complaints spike. Replies to broadcasts open right inside your shared inbox.
        </div>
      </div>
      <div style="display: flex; align-items: center; justify-content: space-between; font-size: 12px; color: var(--fg-3);">
        <span>Attachments up to 25 MB · Undo send 5s window</span>
      </div>
    </div>`,
  );
}

export const MESSENGER_EMAIL_SCENES: SceneSpec[] = [
  {
    name: "channel-messenger-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.8,
    posterTime: 4.3,
    html: messengerHeroHtml,
  },
  {
    name: "channel-messenger-home",
    width: 720,
    height: 640,
    kind: "video",
    duration: 4.2,
    posterTime: 3.8,
    html: messengerHomeHtml,
  },
  { name: "channel-messenger-install", width: 720, height: 640, kind: "still", html: messengerInstallHtml },
  { name: "channel-messenger-identity", width: 720, height: 640, kind: "still", html: messengerIdentityHtml },
  { name: "channel-messenger-brand", width: 720, height: 640, kind: "still", html: messengerBrandHtml },
  {
    name: "channel-email-hero",
    width: 1680,
    height: 1000,
    kind: "video",
    duration: 4.4,
    posterTime: 3.8,
    html: emailHeroHtml,
  },
  {
    name: "channel-email-thread",
    width: 720,
    height: 640,
    kind: "video",
    duration: 4.0,
    posterTime: 3.5,
    html: emailThreadHtml,
  },
  { name: "channel-email-domain", width: 720, height: 640, kind: "still", html: emailDomainHtml },
  { name: "channel-email-routing", width: 720, height: 640, kind: "still", html: emailRoutingHtml },
  { name: "channel-email-signature", width: 720, height: 640, kind: "still", html: emailSignatureHtml },
];
