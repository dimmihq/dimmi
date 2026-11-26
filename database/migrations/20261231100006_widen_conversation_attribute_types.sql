-- migrate: phase=contract
-- Conversation attributes take more types: multi-line text, decimals,
-- several choices, a date and time, files, and a link to a contact or a
-- company. A list may hold up to 200 options. Only widens what was
-- allowed, so the release before reads every row it wrote.

-- +goose Up
ALTER TABLE ai_conversation_attributes DROP CONSTRAINT ai_conversation_attributes_data_type_check;
ALTER TABLE ai_conversation_attributes ADD CONSTRAINT ai_conversation_attributes_data_type_check
    CHECK (data_type IN (
        'text', 'long_text', 'number', 'decimal', 'boolean', 'date', 'datetime', 'choice', 'multiple_choice',
        'file', 'contact', 'company'
    )) NOT VALID;
ALTER TABLE ai_conversation_attributes VALIDATE CONSTRAINT ai_conversation_attributes_data_type_check;
ALTER TABLE ai_conversation_attributes DROP CONSTRAINT ai_conversation_attributes_options_check;
ALTER TABLE ai_conversation_attributes ADD CONSTRAINT ai_conversation_attributes_options_check
    CHECK (cardinality(options) <= 200) NOT VALID;
ALTER TABLE ai_conversation_attributes VALIDATE CONSTRAINT ai_conversation_attributes_options_check;
ALTER TABLE ai_conversation_attributes DROP CONSTRAINT ai_conversation_attributes_check;
ALTER TABLE ai_conversation_attributes ADD CONSTRAINT ai_conversation_attributes_check
    CHECK ((data_type IN ('choice', 'multiple_choice')) = (cardinality(options) > 0)) NOT VALID;
ALTER TABLE ai_conversation_attributes VALIDATE CONSTRAINT ai_conversation_attributes_check;

-- +goose Down
DELETE FROM ai_conversation_attributes
WHERE data_type NOT IN ('text', 'number', 'date', 'boolean', 'choice') OR cardinality(options) > 30;
ALTER TABLE ai_conversation_attributes DROP CONSTRAINT ai_conversation_attributes_check;
ALTER TABLE ai_conversation_attributes ADD CONSTRAINT ai_conversation_attributes_check
    CHECK ((data_type = 'choice') = (cardinality(options) > 0));
ALTER TABLE ai_conversation_attributes DROP CONSTRAINT ai_conversation_attributes_options_check;
ALTER TABLE ai_conversation_attributes ADD CONSTRAINT ai_conversation_attributes_options_check
    CHECK (cardinality(options) <= 30);
ALTER TABLE ai_conversation_attributes DROP CONSTRAINT ai_conversation_attributes_data_type_check;
ALTER TABLE ai_conversation_attributes ADD CONSTRAINT ai_conversation_attributes_data_type_check
    CHECK (data_type IN ('text', 'number', 'date', 'boolean', 'choice'));
