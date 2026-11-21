-- migrate: phase=expand
-- What a run did with the procedure it followed: the path it took, the
-- branches code decided, and every time a guardrail stepped in, so the
-- team sees why a step ran or did not.

-- +goose Up
ALTER TABLE ai_runs ADD COLUMN procedure_trace jsonb;

-- +goose Down
ALTER TABLE ai_runs DROP COLUMN procedure_trace;
