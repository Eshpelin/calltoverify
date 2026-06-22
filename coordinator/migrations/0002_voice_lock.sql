-- 0002_voice_lock.sql — enforce one pending voice (call/DTMF) session per number.
-- A SIM handles a single voice call at a time, so this index makes the per-SIM
-- voice-concurrency guarantee atomic: a concurrent second voice start on the same
-- number fails with a unique violation (surfaced as a "busy" response) instead of
-- racing past the application-level check.

CREATE UNIQUE INDEX IF NOT EXISTS sessions_one_voice_per_number
    ON sessions (number_id)
    WHERE status = 'pending' AND channel IN ('call', 'dtmf');
