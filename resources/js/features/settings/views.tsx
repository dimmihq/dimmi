/*
 * Which page each settings address shows. Every page the settings
 * sidebar lists has one here.
 */
import type { ComponentType } from "react";

import type { SettingsSection } from "@/features/shell/navigation";

import {
  NotificationsSettings,
  PreferencesSettings,
  ProfileSettings,
  SecuritySettings,
  TokensSettings,
} from "./pages/account";
import {
  AuditLogSettings,
  BillingSettings,
  ImportSettings,
  TeamsSettings,
  WorkspaceSecuritySettings,
} from "./pages/admin";
import { ActionsSettings, AgentSettings, ProvidersSettings } from "./pages/ai";
import { ConnectionsSettings, MessengerSettings } from "./pages/channels";
import {
  ChangelogSettings,
  EngageSettings,
  FeedbackSettings,
  HelpCentersSettings,
  KnowledgeSettings,
} from "./pages/content";
import { AppsSettings, GithubSettings, IdentitySettings, WebhooksSettings } from "./pages/developers";
import { EmailSettings } from "./pages/email";
import { InboxesSettings, SavedRepliesSettings, TagsSettings } from "./pages/inbox";
import { MembersSettings, WorkspaceSettings } from "./pages/workspace";

export const settingsViews: Record<SettingsSection, ComponentType> = {
  profile: ProfileSettings,
  preferences: PreferencesSettings,
  notifications: NotificationsSettings,
  security: SecuritySettings,
  tokens: TokensSettings,
  inboxes: InboxesSettings,
  tags: TagsSettings,
  "saved-replies": SavedRepliesSettings,
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
  "workspace-security": WorkspaceSecuritySettings,
  "audit-log": AuditLogSettings,
  import: ImportSettings,
  billing: BillingSettings,
};
