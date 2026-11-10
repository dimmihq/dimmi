-- migrate: phase=contract
-- Broadcast emails are recorded like every other email the workspace
-- sends, so bounces and complaints find them by Message-ID and the
-- same suppressions protect the address. The check only widens: every
-- row the old release writes still passes it. Going down restores the
-- narrow check unvalidated, so broadcast rows are kept, not refused.

-- +goose Up
ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_kind_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_kind_check CHECK (kind IN (
    'reply', 'ai_reply', 'system', 'widget_offline', 'new_conversation', 'test', 'changelog', 'feedback_update',
    'portal_sign_in', 'broadcast'
)) NOT VALID;
ALTER TABLE outbound_emails VALIDATE CONSTRAINT outbound_emails_kind_check;

-- +goose Down
ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_kind_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_kind_check CHECK (kind IN (
    'reply', 'ai_reply', 'system', 'widget_offline', 'new_conversation', 'test', 'changelog', 'feedback_update',
    'portal_sign_in'
)) NOT VALID;
