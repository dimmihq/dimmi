/*
 * Connecting through Meta's sign-in, without Meta's JavaScript SDK: the
 * dashboard loads no third-party script, so the dialog opens in a popup
 * of its own and Meta returns its code to /d/whatsapp-return, which
 * hands it back here. Meta's pages cut the popup off from the window
 * that opened it, so the code comes back over a channel every dimmi
 * window on this browser hears, not through the popup itself; for the
 * same reason the popup cannot be watched for closing, and a sign-in
 * counts as abandoned when the person is back on this page without a
 * code. WhatsApp's popup also posts which account and number were
 * picked; when that message never arrives, the server reads them from
 * the token instead.
 */

const metaOrigins = ["https://www.facebook.com", "https://web.facebook.com", "https://business.facebook.com"];

export const signupReturnPath = "/d/whatsapp-return";
export const signupMessage = "dimmi:whatsapp-signup";
export const signupChannel = "dimmi:meta-signup";
export const signupTaken = "dimmi:meta-signup-taken";

const startedKey = "dimmi:meta-signup:started";
const finishedKey = "dimmi:meta-signup:finished";

/*
 * A sign-in that came back to a window with nothing left listening, as
 * when the browser opened Meta in place of the page: the page it started
 * on, and the code Meta handed back.
 */
interface Started {
  state: string;
  page: string;
  redirect_uri: string;
}

function readJSON<T>(key: string): T | null {
  try {
    const raw = window.localStorage.getItem(key);

    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

function writeJSON(key: string, value: unknown) {
  try {
    if (value === null) {
      window.localStorage.removeItem(key);
    } else {
      window.localStorage.setItem(key, JSON.stringify(value));
    }
  } catch {
    /* Storage is blocked; only the open page can finish the sign-in. */
  }
}

/*
 * leaveForPage keeps the code for the page the sign-in started on and
 * returns that page's address, or null when this browser started none
 * with the state.
 */
export function leaveForPage(state: string, code: string): string | null {
  const started = readJSON<Started>(startedKey);
  if (!started || started.state !== state) {
    return null;
  }

  writeJSON(finishedKey, { code, redirect_uri: started.redirect_uri, page: started.page });

  return started.page;
}

/*
 * takeFinishedSignup returns, once, the code of a sign-in that came back
 * for this page while it was not open.
 */
export function takeFinishedSignup(): SignupResult | null {
  const finished = readJSON<{ code: string; redirect_uri: string; page: string }>(finishedKey);
  if (!finished || finished.page !== window.location.pathname) {
    return null;
  }

  writeJSON(finishedKey, null);
  writeJSON(startedKey, null);

  return { code: finished.code, redirect_uri: finished.redirect_uri };
}

/*
 * How long after the person is back on this page a sign-in that handed
 * no code back counts as abandoned.
 */
const abandonedAfter = 4000;

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
export function openSignup(
  config: {
    app_id: string;
    config_id: string;
    graph_version: string;
  },
  /* plain opens Meta's ordinary business sign-in, as Messenger and Instagram use, without WhatsApp's signup steps. */
  plain = false,
): Promise<SignupResult> {
  const state = crypto.randomUUID();
  const redirect = `${window.location.origin}${signupReturnPath}`;
  const url = new URL(`https://www.facebook.com/${config.graph_version}/dialog/oauth`);
  const query = new URLSearchParams({
    client_id: config.app_id,
    config_id: config.config_id,
    response_type: "code",
    override_default_response_type: "true",
    redirect_uri: redirect,
    state,
    display: "popup",
  });
  if (!plain) {
    query.set("extras", JSON.stringify({ setup: {}, sessionInfoVersion: "3", version: "v4" }));
  }
  url.search = query.toString();

  writeJSON(startedKey, { state, page: window.location.pathname, redirect_uri: redirect } satisfies Started);

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
    const channel = new BroadcastChannel(signupChannel);
    let settled = false;
    let leaving: number | undefined;

    const finish = (fn: () => void) => {
      if (settled) {
        return;
      }
      settled = true;
      window.removeEventListener("message", listen);
      window.removeEventListener("focus", back);
      window.removeEventListener("blur", back);
      document.removeEventListener("visibilitychange", back);
      window.clearTimeout(leaving);
      channel.close();
      fn();
    };

    const returned = (data: { type?: string; state?: string; code?: string }) => {
      if (data?.type !== signupMessage || data.state !== state) {
        return;
      }

      channel.postMessage({ type: signupTaken, state });

      finish(() => {
        if (data.code) {
          resolve({ ...picked, code: data.code, redirect_uri: redirect });
        } else {
          reject(new SignupClosed(false));
        }
      });
    };

    const listen = (event: MessageEvent) => {
      if (metaOrigins.includes(event.origin)) {
        Object.assign(picked, readSession(event.data));
        return;
      }

      if (event.origin === window.location.origin) {
        returned(event.data);
      }
    };

    /* The person left for Meta's window, then came back to this page. */
    let away = false;
    const back = () => {
      if (document.visibilityState !== "visible" || !document.hasFocus()) {
        away = true;
        return;
      }
      if (!away) {
        return;
      }
      window.clearTimeout(leaving);
      leaving = window.setTimeout(() => finish(() => reject(new SignupClosed(false))), abandonedAfter);
    };

    channel.onmessage = (event) => returned(event.data);
    window.addEventListener("message", listen);
    window.addEventListener("focus", back);
    window.addEventListener("blur", back);
    document.addEventListener("visibilitychange", back);
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
