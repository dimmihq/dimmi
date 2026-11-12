/*
 * Which page each settings address shows. Every page the settings
 * sidebar lists has one here.
 */
import type { ComponentType } from "react";

import { ActionsSettings } from "@/features/ai/actions/actions-page";
import { AgentSettings } from "@/features/ai/agent/agent-settings-page";
import { ProvidersSettings } from "@/features/ai/providers/providers-page";
import type { SettingsSection } from "@/features/shell/navigation";

import { PreferencesSettings, ProfileSettings, SecuritySettings } from "./pages/account";
import { WorkspaceSecuritySettings } from "./pages/admin";
import { AttributesSettings } from "./pages/attributes";
import { AuditLogSettings } from "./pages/audit-log";
import { ConnectedAppsSettings } from "./pages/connected-apps";
import { ChangelogSettings, FeedbackSettings, HelpCentersSettings, KnowledgeSettings } from "./pages/content";
import { ConversationRatingsSettings } from "./pages/conversation-ratings";
import { ConversationSafetySettings } from "./pages/conversation-safety";
import { WebhooksSettings } from "./pages/developers";
import { EmailSettings } from "./pages/email";
import { ExportsSettings } from "./pages/exports";
import { IdentitySettings } from "./pages/identity";
import { InboxesSettings } from "./pages/inboxes";
import { IntegrationsSettings } from "./pages/integrations";
import { MessengersSettings } from "./pages/messengers";
import { NotificationsSettings } from "./pages/notifications";
import { OfficeHoursSettings } from "./pages/office-hours";
import { OperatorSettingsView } from "./pages/operator";
import { RolesSettings } from "./pages/roles";
import { SavedRepliesSettings } from "./pages/saved-replies";
import { SingleSignOnSettings } from "./pages/single-sign-on";
import { TagsSettings } from "./pages/tags";
import { TeamsSettings } from "./pages/teams";
import { TokensSettings } from "./pages/tokens";
import { MembersSettings, WorkspaceSettings } from "./pages/workspace";

export const settingsViews: Record<SettingsSection, ComponentType> = {
  profile: ProfileSettings,
  preferences: PreferencesSettings,
  notifications: NotificationsSettings,
  security: SecuritySettings,
  tokens: TokensSettings,
  "connected-apps": ConnectedAppsSettings,
  exports: ExportsSettings,
  inboxes: InboxesSettings,
  tags: TagsSettings,
  "saved-replies": SavedRepliesSettings,
  attributes: AttributesSettings,
  "office-hours": OfficeHoursSettings,
  "conversation-safety": ConversationSafetySettings,
  "conversation-ratings": ConversationRatingsSettings,
  messenger: MessengersSettings,
  email: EmailSettings,
  integrations: IntegrationsSettings,
  agent: AgentSettings,
  operator: OperatorSettingsView,
  actions: ActionsSettings,
  "ai-providers": ProvidersSettings,
  "help-centers": HelpCentersSettings,
  knowledge: KnowledgeSettings,
  changelog: ChangelogSettings,
  feedback: FeedbackSettings,
  webhooks: WebhooksSettings,
  identity: IdentitySettings,
  workspace: WorkspaceSettings,
  members: MembersSettings,
  teams: TeamsSettings,
  roles: RolesSettings,
  "workspace-security": WorkspaceSecuritySettings,
  "single-sign-on": SingleSignOnSettings,
  "audit-log": AuditLogSettings,
};
