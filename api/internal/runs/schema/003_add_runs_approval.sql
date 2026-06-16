-- Adds an approval JSONB column to capture the audit record for plan
-- approvals and rejections. Nullable: existing rows have no decision.
-- The JSON shape is {decision, reason, decided_at}; see the spec for
-- details.
ALTER TABLE runs
    ADD COLUMN IF NOT EXISTS approval JSONB;
