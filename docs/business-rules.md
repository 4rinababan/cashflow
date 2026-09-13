# Business rules

Living document; extend as each phase adds behaviour. Do not contradict
`docs/roadmap.md` or the ADRs — if a rule here conflicts, the ADR wins and this
file should be corrected.

## Ownership

- A transaction, account, or category belongs to exactly one **workspace**,
  not directly to a user (Phase 9). A workspace has one or more members, each
  either `owner` or `member`: an owner can rename the workspace and
  invite/remove members; a member can read/write ledger data but cannot
  manage membership. The owner can never be removed (no ownership transfer,
  no "leave" flow in v1).
- Every new user gets exactly one personal workspace automatically at
  registration, so solo use needs no extra setup. Sharing beyond it (the
  household/family or UMKM case) is opt-in: create an additional workspace
  and invite others into it by email.
- A receipt scan still belongs to exactly one user directly (not a
  workspace) — its daily rate limit is per-user by design, and a scan is
  never itself saved as a transaction (see docs/adr/0002-ai-receipt-scanning.md).
- The API must reject (404, not 403 — never confirm another workspace's row
  exists) any request for a row the authenticated user's current workspace
  does not own, and any nested ledger/report route for a workspace the
  caller isn't a member of.
- Deleting your own account (Phase 8) deletes any workspace you are the
  *sole* member of, along with everything in it. A workspace you share with
  others survives your own deletion — only your membership in it goes away.

## Transactions

- Every transaction has: amount (positive decimal), a direction (`income` |
  `expense`), a currency, an `occurred_at` date, an account, a category, and an
  optional note.
- Amounts are stored as fixed-point (integer minor units or `numeric`), never
  floating point.
- Deleting a transaction is a soft delete (keep history for reports) unless the
  user explicitly asks for hard delete — decide the exact mechanism in Phase 3.

## Categories

- Ship a small set of sensible defaults (e.g. Food, Transport, Salary, Bills)
  per new workspace (a user's personal one at signup, or a shared one when
  created); any member can add/rename/archive on top.
- A category cannot be deleted while transactions reference it — archive
  instead.

## AI-drafted transactions

- A draft from receipt scanning is never saved as a real transaction until the
  user confirms it in the app (see ADR 0002). The draft is not a database row
  in the transactions table until confirmed.
- The app must show the AI's confidence / let the user see the original photo
  next to the extracted fields, so a bad read is easy to catch before saving.

## Currency

- v1 target currency: IDR (the initial user's own use case). The schema should
  not hard-code IDR-only, but multi-currency UX polish is not required for v1.

## Free tier / cost control

- Receipt-scan rate limits are configuration, not a hard-coded constant, so
  they can change without a code change — see ADR 0002. The actual limit
  numbers are a product decision for the user to set in Phase 4.
