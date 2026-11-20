/*
 * Connecting a WhatsApp number through Meta's Embedded Signup, without
 * Meta's JavaScript SDK: the dashboard loads no third-party script, so
 * the signup dialog opens in a popup of its own and Meta returns its code
 * to /d/whatsapp-return, which hands it back here. Meta's popup also
 * posts which account and number were picked; when that message never
 * arrives, the server reads them from the token instead.
 */

const metaOrigins = ["https://www.facebook.com", "https://web.facebook.com", "https://business.facebook.com"];

export const signupReturnPath = "/d/whatsapp-return";
export const signupMessage = "dimmi:whatsapp-signup";

/*
 * SignupResult is what the popup handed back: the code, the address it
 * returned to, and the account and number Meta said were picked.
 */
export interface SignupResult {
  code: string;
  redirect_uri: string;
  waba_id?: string;
  phone_number_id?: string;
  business_id?: string;
  coexistence?: boolean;
}

/*
 * SignupClosed is a popup the person closed, or one the browser blocked.
 */
export class SignupClosed extends Error {
  constructor(public blocked: boolean) {
    super(blocked ? "The browser blocked Meta's window. Allow pop-ups and try again." : "Signup was closed.");
  }
}

/*
 * openSignup opens Meta's signup dialog and resolves with what it handed
 * back, or rejects when the person closes it.
 */
export function openSignup(config: {
  app_id: string;
  config_id: string;
  graph_version: string;
}): Promise<SignupResult> {
  const state = crypto.randomUUID();
  const redirect = `${window.location.origin}${signupReturnPath}`;
  const url = new URL(`https://www.facebook.com/${config.graph_version}/dialog/oauth`);
  url.search = new URLSearchParams({
    client_id: config.app_id,
    config_id: config.config_id,
    response_type: "code",
    override_default_response_type: "true",
    redirect_uri: redirect,
    state,
    display: "popup",
    extras: JSON.stringify({ setup: {}, sessionInfoVersion: "3", version: "v4" }),
  }).toString();

  const width = 680;
  const height = 760;
  const left = Math.max(0, window.screenX + (window.outerWidth - width) / 2);
  const top = Math.max(0, window.screenY + (window.outerHeight - height) / 2);
  const popup = window.open(
    url,
    "dimmi-whatsapp-signup",
    `popup,width=${width},height=${height},left=${left},top=${top}`,
  );

  return new Promise((resolve, reject) => {
    if (!popup) {
      reject(new SignupClosed(true));
      return;
    }

    const picked: Partial<SignupResult> = {};
    let settled = false;

    const finish = (fn: () => void) => {
      if (settled) {
        return;
      }
      settled = true;
      window.removeEventListener("message", listen);
      window.clearInterval(watch);
      fn();
    };

    const listen = (event: MessageEvent) => {
      if (metaOrigins.includes(event.origin)) {
        Object.assign(picked, readSession(event.data));
        return;
      }

      if (event.origin !== window.location.origin || event.data?.type !== signupMessage) {
        return;
      }

      const data = event.data as { state?: string; code?: string; error?: string };
      if (data.state !== state) {
        return;
      }

      finish(() => {
        if (data.code) {
          resolve({ ...picked, code: data.code, redirect_uri: redirect });
        } else {
          reject(new SignupClosed(false));
        }
      });
    };

    window.addEventListener("message", listen);
    const watch = window.setInterval(() => {
      if (popup.closed) {
        window.setTimeout(() => finish(() => reject(new SignupClosed(false))), 800);
      }
    }, 500);
  });
}

/*
 * readSession reads Meta's message about what was picked, ignoring any
 * other message from Meta's windows.
 */
function readSession(raw: unknown): Partial<SignupResult> {
  let data = raw;
  if (typeof raw === "string") {
    try {
      data = JSON.parse(raw);
    } catch {
      return {};
    }
  }

  const message = data as { type?: string; event?: string; data?: Record<string, string> };
  if (message?.type !== "WA_EMBEDDED_SIGNUP" || !message.data) {
    return {};
  }

  return {
    waba_id: message.data.waba_id,
    phone_number_id: message.data.phone_number_id,
    business_id: message.data.business_id,
    coexistence: message.event === "FINISH_WHATSAPP_BUSINESS_APP_ONBOARDING",
  };
}
