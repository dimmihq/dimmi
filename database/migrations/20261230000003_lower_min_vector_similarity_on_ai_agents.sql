-- migrate: phase=expand
-- The minimum cosine similarity was set for the first built-in
-- embedder. The one that replaced it scores the passages that answer a
-- question from about 0.45 up and unrelated ones under 0.35, so 0.50
-- hid passages that answer. No one could set the value yet, so every
-- agent still at the old default moves to the new one.

-- +goose Up
ALTER TABLE ai_agents ALTER COLUMN min_vector_similarity SET DEFAULT 0.40;
UPDATE ai_agents SET min_vector_similarity = 0.40 WHERE min_vector_similarity = 0.50;

-- +goose Down
UPDATE ai_agents SET min_vector_similarity = 0.50 WHERE min_vector_similarity = 0.40;
ALTER TABLE ai_agents ALTER COLUMN min_vector_similarity SET DEFAULT 0.50;
