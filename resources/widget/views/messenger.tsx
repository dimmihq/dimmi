/*
 * The whole messenger: the panel with one view per tab the customer kept,
 * in their order, the conversation and the pages that open over them, the
 * tab bar, and the launcher. With a single tab the bar is left out. The
 * first tab is current until the messenger controller moves on.
 */
import { type Html, raw } from "widget-jsx/jsx-runtime";

import { ChangelogIcon, CloseIcon, ExternalIcon, HelpIcon, HomeIcon, MessagesIcon } from "../components/icons";
import { Launcher } from "../components/launcher";
import type { Content } from "../content";
import type { Options, Tab } from "../options";
import { Changelog } from "./changelog";
import { Conversation } from "./conversation";
import { Help } from "./help";
import { Home } from "./home";
import { Messages } from "./messages";
import { Pages } from "./pages";

type View = (props: { options: Options; content: Content }) => Html;

const views: Record<Tab, View> = { home: Home, messages: Messages, changelog: Changelog, help: Help };

const icons: Record<Tab, () => Html> = {
  home: HomeIcon,
  messages: MessagesIcon,
  changelog: ChangelogIcon,
  help: HelpIcon,
};

function TabBar({ options, content }: { options: Options; content: Content }) {
  if (options.tabs.length < 2) {
    return raw("");
  }

  return (
    <div className="tabs" role="tablist">
      {options.tabs.map((tab, index) => {
        const Icon = icons[tab];

        return (
          <button
            type="button"
            role="tab"
            data-action="click->select"
            data-tab={tab}
            aria-selected={String(index === 0)}
            key={tab}
          >
            <Icon />
            {tab === "changelog" && content.unread_changelog > 0 && <b className="dot" />}
            {tab === "messages" && <b className="count" data-target="tab-count" hidden />}
            <span>{options.texts[tab]}</span>
          </button>
        );
      })}
    </div>
  );
}

/*
 * Viewer shows one picture across the whole panel, over everything else,
 * with a way to open the original and a way back.
 */
function Viewer() {
  return (
    <div className="viewer" data-target="viewer" role="dialog" aria-label="Picture" hidden>
      <div className="viewer-bar">
        <button className="icon-button" type="button" data-action="click->original" aria-label="Open the original">
          <ExternalIcon />
        </button>
        <button className="icon-button" type="button" data-action="click->unview" aria-label="Close picture">
          <CloseIcon />
        </button>
      </div>
      <div className="viewer-stage" data-action="click->unview">
        <img data-target="viewer-image" alt="" />
      </div>
    </div>
  );
}

export function Messenger({ options, content }: { options: Options; content: Content }) {
  const { appearance } = options;
  const first = options.tabs[0] ?? "home";

  return (
    <div
      className="messenger"
      data-controller="messenger"
      data-action="keydown->dismiss"
      data-tab={first}
      data-side={appearance.position}
      data-font={appearance.font}
      data-sound={String(appearance.sound)}
    >
      <section
        className="panel"
        data-target="panel"
        role="dialog"
        aria-label={options.texts.messages}
        lang={options.locale || undefined}
        dir={options.dir}
        inert
      >
        <div className="views">
          {options.tabs.map((tab) => {
            const View = views[tab];

            return (
              <div
                className="view"
                data-view={tab}
                role="tabpanel"
                data-current={tab === first}
                data-load
                data-controller="scroll"
                key={tab}
              >
                <View options={options} content={content} />
              </div>
            );
          })}
          <div className="view" data-view="conversation" data-load>
            <Conversation options={options} content={content} />
          </div>
          <Pages options={options} content={content} />
        </div>
        <TabBar options={options} content={content} />
        <Viewer />
      </section>
      <div className="notice" data-target="notice" role="status" aria-live="polite" dir={options.dir} hidden />
      <Launcher launcher={appearance.launcher} ruledOut={options.launcher_ruled_out} />
    </div>
  );
}
