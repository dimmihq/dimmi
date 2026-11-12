-- migrate: phase=contract
-- The email asking a customer to rate a closed conversation is recorded
-- like the team's replies, under a kind of its own, so bounces and
-- complaints find it by Message-ID and the same suppressions protect the
-- address. The check only widens: every row the old release writes
-- still passes it. Going down restores the narrow check unvalidated, so
-- rating rows are kept, not refused.

-- +goose Up
ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_kind_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_kind_check CHECK (kind IN (
    'reply', 'ai_reply', 'system', 'widget_offline', 'new_conversation', 'test', 'changelog', 'feedback_update',
    'portal_sign_in', 'broadcast', 'conversation_rating'
)) NOT VALID;
ALTER TABLE outbound_emails VALIDATE CONSTRAINT outbound_emails_kind_check;

-- +goose Down
ALTER TABLE outbound_emails DROP CONSTRAINT outbound_emails_kind_check;
ALTER TABLE outbound_emails ADD CONSTRAINT outbound_emails_kind_check CHECK (kind IN (
    'reply', 'ai_reply', 'system', 'widget_offline', 'new_conversation', 'test', 'changelog', 'feedback_update',
    'portal_sign_in', 'broadcast'
)) NOT VALID;
