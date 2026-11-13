/*
 * Everything a customer can change about the messenger, with the values a
 * new install starts from. The shape follows the widget settings the server
 * will send in its config response, keys and all, so wiring the backend in
 * means merging that response over these defaults and nothing else. Until
 * then a page can preview changes with Dimmi("update", { appearance: ... }).
 *
 * The brand color drives the palette: the logo tile, primary buttons, the
 * header's tint, avatars and accents all derive from it, and text on it
 * turns ink or white by its lightness.
 */

export type Tab = "home" | "messages" | "changelog" | "help";

export interface Launcher {
  style: "bubble" | "tab" | "hidden";
  shape: "circle" | "squircle";
  label: string;
}

export interface Appearance {
  primary_color: string;
  header_color?: string;
  button_color?: string;
  theme: "light" | "dark" | "auto";
  position: "right" | "left";
  padding_x: number;
  padding_y: number;
  launcher: Launcher;
  font: "system" | "inherit";
  sound: boolean;
}

export interface Texts {
  greeting_intro: string;
  greeting: string;
  chat_title: string;
  chat_body: string;
  recent_messages: string;
  links: string;
  show_all: string;
  send_message: string;
  composer_placeholder: string;
  messages_empty_title: string;
  messages_empty_body: string;
  search_placeholder: string;
  search_none: string;
  popular: string;
  helpful: string;
  update_feedback: string;
  still_need_help: string;
  open_in_help_center: string;
  rating_thanks: string;
  home: string;
  messages: string;
  changelog: string;
  help: string;
  seen: string;
  today: string;
  status_with_team: string;
  status_waiting_on_you: string;
  status_closed: string;
  email_prompt: string;
  delay_notice: string;
  ai_answering: string;
  ai_helped: string;
  ai_thanks: string;
  rating_ask: string;
  rating_comment: string;
  rating_send: string;
  rated_thanks: string;
  talk_to_human: string;
  action_confirm: string;
  action_cancel: string;
  action_waiting: string;
  action_done: string;
  action_declined: string;
  action_canceled: string;
  action_expired: string;
  action_failed: string;
  verify_intro: string;
  verify_email: string;
  verify_send: string;
  verify_sent: string;
  verify_code: string;
  verify_submit: string;
  verify_resend: string;
  verify_other: string;
}

export interface Options {
  appearance: Appearance;
  texts: Texts;
  tabs: Tab[];
  show_branding: boolean;
  branding_url: string;
  launcher_ruled_out: boolean;
  locale: string;
  dir: "ltr" | "rtl";
}

/*
 * language is the visitor's language a page chose with Dimmi("boot") or
 * Dimmi("update"), over the browser's, for the server to answer in.
 */
export type OptionsUpdate = {
  language?: string;
  locale?: string;
  dir?: "ltr" | "rtl";
  appearance?: Partial<Omit<Appearance, "launcher">> & { launcher?: Partial<Launcher> };
  texts?: Partial<Texts>;
  tabs?: Tab[];
  show_branding?: boolean;
  branding_url?: string;
  launcher_ruled_out?: boolean;
};

export const defaults: Options = {
  appearance: {
    primary_color: "#1f2328",
    theme: "auto",
    position: "right",
    padding_x: 20,
    padding_y: 20,
    launcher: { style: "bubble", shape: "circle", label: "" },
    font: "system",
    sound: true,
  },
  texts: {
    greeting_intro: "Hey there.",
    greeting: "How can we help?",
    chat_title: "Chat with us",
    chat_body: "We usually reply in a few minutes",
    recent_messages: "Recent messages",
    links: "Get in touch",
    show_all: "Show all",
    send_message: "Send us a message",
    composer_placeholder: "Write a message…",
    messages_empty_title: "No messages yet",
    messages_empty_body: "Questions you ask will show up here.",
    search_placeholder: "Search for help",
    search_none: "No results",
    popular: "Popular articles",
    helpful: "Did this answer your question?",
    update_feedback: "What do you think of this update?",
    still_need_help: "Still need help? Chat with us",
    open_in_help_center: "Open in help center",
    rating_thanks: "Thanks for letting us know.",
    home: "Home",
    messages: "Messages",
    changelog: "Changelog",
    help: "Help",
    seen: "Seen",
    today: "Today",
    status_with_team: "With the team",
    status_waiting_on_you: "Waiting on you",
    status_closed: "Closed",
    email_prompt: "Where should we reply?",
    delay_notice: "We're away right now. We'll reply as soon as we're back.",
    ai_answering: "AI answering",
    ai_helped: "That helped",
    ai_thanks: "Glad it helped.",
    rating_ask: "How did we do?",
    rating_comment: "Anything to add?",
    rating_send: "Send",
    rated_thanks: "Thanks for your feedback!",
    talk_to_human: "Talk to a person",
    action_confirm: "Confirm",
    action_cancel: "Cancel",
    action_waiting: "Waiting for the team to review this",
    action_done: "Done",
    action_declined: "Not approved",
    action_canceled: "Canceled",
    action_expired: "This request expired",
    action_failed: "This didn't go through",
    verify_intro: "Confirm your email to continue",
    verify_email: "Email",
    verify_send: "Send code",
    verify_sent: "Enter the 6-digit code we sent to {email}",
    verify_code: "Code",
    verify_submit: "Verify",
    verify_resend: "Send a new code",
    verify_other: "Use another email",
  },
  tabs: ["home", "messages", "changelog", "help"],
  show_branding: true,
  branding_url: "https://trydimmi.com",
  launcher_ruled_out: false,
  locale: "",
  dir: "ltr",
};

export function merge(options: Options, update: OptionsUpdate = {}): Options {
  return {
    appearance: {
      ...options.appearance,
      ...update.appearance,
      launcher: { ...options.appearance.launcher, ...update.appearance?.launcher },
    },
    texts: { ...options.texts, ...update.texts },
    tabs: update.tabs?.length ? update.tabs : options.tabs,
    show_branding: update.show_branding ?? options.show_branding,
    branding_url: update.branding_url ?? options.branding_url,
    launcher_ruled_out: update.launcher_ruled_out ?? options.launcher_ruled_out,
    locale: update.locale ?? options.locale,
    dir: update.dir ?? options.dir,
  };
}

/*
 * Whether text on the brand color should be dark ink rather than white,
 * by the color's relative luminance.
 */
export function isLight(hex: string): boolean {
  const [r = 0, g = 0, b = 0] = [1, 3, 5].map((at) => (Number.parseInt(hex.slice(at, at + 2), 16) || 0) / 255);

  return 0.2126 * r + 0.7152 * g + 0.0722 * b > 0.62;
}
