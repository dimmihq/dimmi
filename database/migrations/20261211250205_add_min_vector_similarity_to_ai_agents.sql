-- migrate: phase=expand
-- A passage found by meaning carries the cosine similarity of its
-- vector to the question, a different scale from the keyword coverage
-- min_similarity gates, so the agent keeps its own minimum for it.

-- +goose Up
ALTER TABLE ai_agents ADD COLUMN min_vector_similarity numeric(3,2) NOT NULL DEFAULT 0.50;

-- +goose Down
ALTER TABLE ai_agents DROP COLUMN min_vector_similarity;
