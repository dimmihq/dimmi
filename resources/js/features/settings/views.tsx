/*
 * Which page each settings address shows. Every page the settings
 * sidebar lists has one here.
 */
import type { ComponentType } from "react";

import type { SettingsSection } from "@/features/shell/navigation";

import { PreferencesSettings, ProfileSettings, SecuritySettings } from "./pages/account";
import { BillingSettings, ImportSettings, WorkspaceSecuritySettings } from "./pages/admin";
import { ActionsSettings, AgentSettings, ProvidersSettings } from "./pages/ai";
import { AuditLogSettings } from "./pages/audit-log";
import { ConnectionsSettings, MessengerSettings } from "./pages/channels";
import {
  ChangelogSettings,
  EngageSettings,
  FeedbackSettings,
  HelpCentersSettings,
  KnowledgeSettings,
} from "./pages/content";
import { ConversationSafetySettings } from "./pages/conversation-safety";
import { AppsSettings, GithubSettings, IdentitySettings, WebhooksSettings } from "./pages/developers";
import { EmailSettings } from "./pages/email";
import { SavedRepliesSettings } from "./pages/inbox";
import { InboxesSettings } from "./pages/inboxes";
import { RolesSettings } from "./pages/roles";
import { SingleSignOnSettings } from "./pages/single-sign-on";
import { TagsSettings } from "./pages/tags";
import { TeamsSettings } from "./pages/teams";
import { TokensSettings } from "./pages/tokens";
import { MembersSettings, WorkspaceSettings } from "./pages/workspace";

export const settingsViews: Record<SettingsSection, ComponentType> = {
  profile: ProfileSettings,
  preferences: PreferencesSettings,
  security: SecuritySettings,
  tokens: TokensSettings,
  inboxes: InboxesSettings,
  tags: TagsSettings,
  "saved-replies": SavedRepliesSettings,
  "conversation-safety": ConversationSafetySettings,
  messenger: MessengerSettings,
  email: EmailSettings,
  connections: ConnectionsSettings,
  agent: AgentSettings,
  actions: ActionsSettings,
  "ai-providers": ProvidersSettings,
  "help-centers": HelpCentersSettings,
  knowledge: KnowledgeSettings,
  changelog: ChangelogSettings,
  feedback: FeedbackSettings,
  engage: EngageSettings,
  webhooks: WebhooksSettings,
  identity: IdentitySettings,
  apps: AppsSettings,
  github: GithubSettings,
  workspace: WorkspaceSettings,
  members: MembersSettings,
  teams: TeamsSettings,
  roles: RolesSettings,
  "workspace-security": WorkspaceSecuritySettings,
  "single-sign-on": SingleSignOnSettings,
  "audit-log": AuditLogSettings,
  import: ImportSettings,
  billing: BillingSettings,
};
