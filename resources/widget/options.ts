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
  popular: string;
  helpful: string;
  update_feedback: string;
  still_need_help: string;
  home: string;
  messages: string;
  changelog: string;
  help: string;
  seen: string;
  status_with_team: string;
  status_waiting_on_you: string;
  status_closed: string;
  email_prompt: string;
  delay_notice: string;
  ai_answering: string;
  ai_helped: string;
  ai_thanks: string;
  talk_to_human: string;
}

export interface Options {
  appearance: Appearance;
  texts: Texts;
  tabs: Tab[];
  show_branding: boolean;
  branding_url: string;
}

export type OptionsUpdate = {
  appearance?: Partial<Omit<Appearance, "launcher">> & { launcher?: Partial<Launcher> };
  texts?: Partial<Texts>;
  tabs?: Tab[];
  show_branding?: boolean;
  branding_url?: string;
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
    popular: "Popular articles",
    helpful: "Did this answer your question?",
    update_feedback: "What do you think of this update?",
    still_need_help: "Still need help? Chat with us",
    home: "Home",
    messages: "Messages",
    changelog: "Changelog",
    help: "Help",
    seen: "Seen",
    status_with_team: "With the team",
    status_waiting_on_you: "Waiting on you",
    status_closed: "Closed",
    email_prompt: "Where should we reply?",
    delay_notice: "We're away right now. We'll reply as soon as we're back.",
    ai_answering: "AI answering",
    ai_helped: "That helped",
    ai_thanks: "Glad it helped.",
    talk_to_human: "Talk to a person",
  },
  tabs: ["home", "messages", "changelog", "help"],
  show_branding: true,
  branding_url: "https://trydimmi.com",
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
