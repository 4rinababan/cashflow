-- Phase 3: core ledger. Every row carries user_id directly (not just via a
-- join) so every query can be scoped to the authenticated user on its own,
-- per the ownership rule in docs/business-rules.md.
CREATE TABLE accounts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    currency    TEXT NOT NULL,
    archived_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ
);
CREATE INDEX idx_accounts_user_id ON accounts(user_id);

CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('income', 'expense')),
    archived_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ
);
CREATE INDEX idx_categories_user_id ON categories(user_id);

-- amount_minor is a fixed-point integer (minor currency units, e.g. cents) —
-- never a float, so rounding errors can't creep into anyone's money.
-- account_id/category_id are deliberately NOT ON DELETE CASCADE: the service
-- layer refuses to hard-delete an account/category that still has
-- transactions (archive instead), and this FK is the backstop if it ever
-- didn't.
CREATE TABLE transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id   UUID NOT NULL REFERENCES accounts(id),
    category_id  UUID NOT NULL REFERENCES categories(id),
    direction    TEXT NOT NULL CHECK (direction IN ('income', 'expense')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency     TEXT NOT NULL,
    occurred_at  DATE NOT NULL,
    note         TEXT NOT NULL DEFAULT '',
    deleted_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ
);
CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_account_id ON transactions(account_id);
CREATE INDEX idx_transactions_category_id ON transactions(category_id);
CREATE INDEX idx_transactions_occurred_at ON transactions(occurred_at);
