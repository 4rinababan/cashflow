ALTER TABLE transactions ADD COLUMN user_id UUID REFERENCES users(id);
UPDATE transactions SET user_id = created_by_user_id;
ALTER TABLE transactions ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE transactions DROP COLUMN workspace_id;
ALTER TABLE transactions DROP COLUMN created_by_user_id;

ALTER TABLE categories ADD COLUMN user_id UUID REFERENCES users(id);
UPDATE categories SET user_id = created_by_user_id;
ALTER TABLE categories ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE categories DROP COLUMN workspace_id;
ALTER TABLE categories DROP COLUMN created_by_user_id;

ALTER TABLE accounts ADD COLUMN user_id UUID REFERENCES users(id);
UPDATE accounts SET user_id = created_by_user_id;
ALTER TABLE accounts ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE accounts DROP COLUMN workspace_id;
ALTER TABLE accounts DROP COLUMN created_by_user_id;

DROP TABLE IF EXISTS workspace_invites;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;
