/*
 * Cloudflare Turnstile, loaded only on the signed-out pages that ask for
 * an answer and only when the server turned a captcha on, so the app
 * never carries the provider's script otherwise. The script is fetched
 * once and shared by every widget on the page.
 */

const source = "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

/*
 * TurnstileOptions are what a widget is shown with.
 */
export interface TurnstileOptions {
  sitekey: string;
  theme: "auto";
  callback: (token: string) => void;
  "expired-callback": () => void;
  "error-callback": () => void;
}

/*
 * Turnstile is the part of the provider's API the app uses.
 */
export interface Turnstile {
  render: (element: HTMLElement, options: TurnstileOptions) => string;
  remove: (id: string) => void;
}

declare global {
  interface Window {
    turnstile?: Turnstile;
  }
}

let loading: Promise<Turnstile> | null = null;

/*
 * loadTurnstile resolves with the provider's API, adding its script to
 * the page the first time it is asked for.
 */
export function loadTurnstile(): Promise<Turnstile> {
  if (window.turnstile) {
    return Promise.resolve(window.turnstile);
  }

  loading ??= new Promise<Turnstile>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = source;
    script.async = true;
    script.onload = () => (window.turnstile ? resolve(window.turnstile) : reject(new Error("Turnstile did not load.")));
    script.onerror = () => {
      loading = null;
      reject(new Error("Turnstile did not load."));
    };
    document.head.append(script);
  });

  return loading;
}
