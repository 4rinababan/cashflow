-- Phase 4: audit log for AI receipt scans. This exists to enforce the
-- per-user daily rate limit and make abuse/failure patterns visible — it
-- deliberately does NOT store any extracted receipt content (merchant,
-- total, line items, etc. never land here; the /receipts/scan response goes
-- straight back to the app as a draft, never through a table). The receipt
-- photo itself is never persisted server-side either: it is held in memory
-- for one AI API call and discarded once the draft is returned. See
-- docs/adr/0002-ai-receipt-scanning.md for the reasoning.
CREATE TABLE receipt_scans (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status     TEXT NOT NULL CHECK (status IN ('ok', 'failed')),
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_receipt_scans_user_id_created_at ON receipt_scans(user_id, created_at);
