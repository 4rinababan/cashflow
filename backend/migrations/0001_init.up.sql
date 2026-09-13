-- Phase 1: no domain tables yet (users/accounts/categories/transactions land
-- in Phases 2–3). This migration only proves the migration runner works end
-- to end and establishes the extensions later migrations can rely on.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
