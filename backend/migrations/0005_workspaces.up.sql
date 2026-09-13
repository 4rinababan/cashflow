-- Phase 9: shared workspaces (family/UMKM use case). Every ledger row moves
-- from being owned directly by a user to being owned by a workspace, which
-- one or more users can be members of. A brand-new user still gets exactly
-- one workspace (their personal one, created at registration — see
-- internal/workspaces.Service.CreatePersonalWorkspace) so solo use is
-- unchanged in spirit; sharing is opt-in via inviting someone else into it,
-- or creating an additional shared workspace.
CREATE TABLE workspaces (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Two roles only (see docs/business-rules.md): 'owner' is whoever created
-- the workspace — permanent for v1, no ownership transfer, no removing an
-- owner — and 'member' can read/write ledger data but cannot manage
-- membership or invites.
CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         TEXT NOT NULL CHECK (role IN ('owner', 'member')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX idx_workspace_members_user_id ON workspace_members(user_id);

-- An invite is a single-use, expiring, emailed token — never the workspace
-- ID itself — so accepting one requires actually receiving the email.
CREATE TABLE workspace_invites (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email        TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    -- Deliberately no REFERENCES users(id): this is audit-only (who sent the
    -- invite), and accepting an invite never looks the inviter row back up —
    -- it must not block the inviter from later deleting their own account.
    invited_by   UUID NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    accepted_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_workspace_invites_workspace_id ON workspace_invites(workspace_id);

-- Add workspace scoping (replaces user_id as the ownership boundary) and a
-- separate "who actually did this" audit column (useful once more than one
-- person can post into the same workspace) to every ledger table.
-- workspace_id cascades: deleting a workspace (internal/workspaces.Service.
-- RemoveUserFromAllWorkspaces, run just before a user's own account is
-- deleted, deletes any workspace where they were the sole member) takes its
-- ledger data with it — nobody could ever reach it otherwise.
-- created_by_user_id deliberately has no REFERENCES users(id): it is
-- audit/display-only (docs/business-rules.md), and a shared workspace's
-- ledger data must survive one contributing member later deleting their own
-- account — a real FK here would instead block that deletion outright.
ALTER TABLE accounts ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE accounts ADD COLUMN created_by_user_id UUID;
ALTER TABLE categories ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE categories ADD COLUMN created_by_user_id UUID;
ALTER TABLE transactions ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
ALTER TABLE transactions ADD COLUMN created_by_user_id UUID;

-- Backfill: give every existing user their own personal workspace and move
-- their existing rows into it, so nothing already recorded is orphaned.
DO $$
DECLARE
    u RECORD;
    new_workspace_id UUID;
BEGIN
    FOR u IN SELECT id, display_name FROM users LOOP
        INSERT INTO workspaces (name) VALUES (u.display_name || '''s Workspace')
        RETURNING id INTO new_workspace_id;

        INSERT INTO workspace_members (workspace_id, user_id, role)
        VALUES (new_workspace_id, u.id, 'owner');

        UPDATE accounts SET workspace_id = new_workspace_id, created_by_user_id = u.id WHERE user_id = u.id;
        UPDATE categories SET workspace_id = new_workspace_id, created_by_user_id = u.id WHERE user_id = u.id;
        UPDATE transactions SET workspace_id = new_workspace_id, created_by_user_id = u.id WHERE user_id = u.id;
    END LOOP;
END $$;

-- Every row now has a workspace; make that required and drop the old
-- direct-to-user ownership column it replaces.
ALTER TABLE accounts ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE accounts ALTER COLUMN created_by_user_id SET NOT NULL;
ALTER TABLE accounts DROP COLUMN user_id;

ALTER TABLE categories ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE categories ALTER COLUMN created_by_user_id SET NOT NULL;
ALTER TABLE categories DROP COLUMN user_id;

ALTER TABLE transactions ALTER COLUMN workspace_id SET NOT NULL;
ALTER TABLE transactions ALTER COLUMN created_by_user_id SET NOT NULL;
ALTER TABLE transactions DROP COLUMN user_id;

CREATE INDEX idx_accounts_workspace_id ON accounts(workspace_id);
CREATE INDEX idx_categories_workspace_id ON categories(workspace_id);
CREATE INDEX idx_transactions_workspace_id ON transactions(workspace_id);
