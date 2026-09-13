# ADR 0002 — AI receipt scanning: provider, flow, cost, and privacy

## Status

Implemented in Phase 4 (`backend/internal/receipts`). **Live-verified working
2026-09-13**, end to end, through the real app: register → `/receipts/scan`
with a real (synthetic but genuine) receipt photo → correct merchant, date,
currency, total, category, and every line item came back, confidence 0.98.
Getting there took three provider changes and two bugs found only by live
testing — recorded here in full because each step is a real lesson, not
just trivia:

1. **Claude API (original plan)** — never implemented; the user redirected
   to Biznet Gio before this phase's provider was built.
2. **Biznet Gio, `openai/gpt-oss-20b`** (`AI_ENDPOINT=https://api.biznetgio.ai/v1`)
   — implemented, then **live-tested and confirmed broken**: a legible
   synthetic receipt photo came back `HTTP 200` with every field empty and
   confidence 0. A raw follow-up request asking the model to simply describe
   that same image returned, in its own reasoning trace, *"we cannot
   actually see the image."* `prompt_tokens` for that request was 81 — far
   too low to have included an encoded image, confirming the `image_url`
   content part never reached the model. A plain text-only request to the
   same key/endpoint worked fine, isolating the problem to exactly one
   thing: `gpt-oss-20b` (OpenAI's open-weight model, documented text-only)
   does not process images, on this gateway or at all.
3. **Google Gemini, `gemini-flash-latest`, via its OpenAI-compatibility layer**
   (`AI_ENDPOINT=https://generativelanguage.googleapis.com/v1beta/openai`) —
   switched to after (2) was confirmed broken. A raw request with the same
   receipt photo correctly described every line of it. Wiring this into the
   real `/receipts/scan` endpoint then surfaced two more real bugs, both
   fixed:
   - **`max_tokens` too low.** Gemini is a reasoning model: it spends part of
     its token budget on hidden "thinking" before emitting visible content.
     At `max_tokens: 1500`, real requests came back with `finish_reason:
     length` and the JSON truncated after ~4 fields. Raising it to `6000`
     (a real request used ~3250 combined reasoning+answer tokens) fixed it.
     See `internal/receipts.maxCompletionTokens`.
   - **Server `WriteTimeout` too short.** At 15s, a real Gemini round trip
     sometimes took longer than that, and Go's `net/http` silently killed
     the connection mid-request — curl saw "empty reply from server," not a
     clean error/timeout response. Raised to 60s in
     `internal/server.New`, with the AI HTTP client's own timeout raised to
     45s to match.
   - A transient `503 "high demand"` from Gemini was also observed during
     testing — handled correctly already by the existing fallback path
     (`ErrScanFailed` → 422 "enter manually"), no fix needed; a retry
     succeeded immediately after.
   - **`total_minor`/`amount_minor` were not actually scaled ×100 for IDR.**
     Found while building the Flutter form that pre-fills from a draft (Phase
     6): the "working" test above returned `totalMinor: 35000` for a receipt
     whose printed total was literally "35000" — the model had echoed the
     face value instead of multiplying by 100 as the prompt asked, because
     IDR has no real-world cents for "multiply by 100" to intuitively anchor
     to. Every other field was correct, which is exactly why this is easy to
     miss without cross-checking against the ledger's own fixed-point
     convention (backend/docs/business-rules.md). Fixed by rewriting
     `receiptSystemPrompt` (`internal/receipts/scanner.go`) with a concrete
     worked example matching this exact case ("Rp 35.000" → `3500000`, not
     `35000`) instead of an abstract rule — re-verified live immediately
     after: the same receipt now correctly returns `totalMinor: 3500000` and
     every line item scaled the same way.

Model/provider choice is fully config-driven (`AI_ENDPOINT`/`AI_API_KEY`/
`AI_MODEL`) — none of this needed a code change beyond the two bug fixes
above, which apply regardless of provider. Model is pinned to the
`-latest` alias rather than a specific version: `gemini-2.5-flash` was
deprecated mid-testing-session (Google's own error pointed the way to the
replacement), so version pinning is a demonstrated, not hypothetical,
maintenance burden for this integration.

- **Photo retention**: the original photo is **never persisted server-side**.
  It is held in memory for exactly one API call and discarded once the draft
  is returned; only a metadata-only audit row (`user_id`, `status`,
  `error_code`, `created_at` — never the merchant/total/line items) is kept,
  solely to enforce the per-user daily limit and see failure patterns. The
  mobile app still holds its own local copy of the photo for the "compare
  against the original" review UX — the backend never needs to store or
  return it for that.

**Extraction convention**: rather than the API's tool/function-calling
feature, the model is asked (via a system prompt) to reply with a plain JSON
object, which the backend then extracts defensively (outermost `{...}`, so a
stray code fence or a sentence of preamble doesn't break parsing). This
mirrors `TaskManagement.Infrastructure.Ai.ChatAiAssistant` — another
OpenAI-compatible integration already in this workspace — which deliberately
avoids tool-calling for the same reason: support for it is inconsistent
across OpenAI-compatible providers, while prompted JSON always works. That
project's `AiOptions` shape (`Endpoint`, `ApiKey`, `Model`, generous
`MaxTokens`, low `Temperature`) is mirrored here too, and its warning about
reasoning models eating the token budget turned out to apply directly, as
described above.

## Context

The feature: a user photographs a paper receipt; the app sends it to an AI
model; the model extracts merchant, date, total, and (where legible) line
items; the app shows this as a **draft** transaction; the user edits anything
wrong and confirms; only then is it saved as a real transaction. This must
never auto-post — a misread total or currency must not silently become a
ledger entry.

## Decision

### Provider: an OpenAI-compatible chat-completions gateway, called from the Go backend

The photo is sent from the backend, not directly from the phone, so the API
key never ships inside the mobile app (an API key embedded in a public app
binary can be extracted). The backend:
1. Receives the receipt image over an authenticated endpoint
   (`POST /receipts/scan`, user-scoped).
2. Calls `AI_ENDPOINT` (currently Gemini's OpenAI-compatibility layer) with
   the image (as an `image_url` data-URL content part) and a system prompt
   asking for a structured `{merchant, occurred_at, currency, total_minor,
   category_guess, line_items[], confidence}` JSON result.
3. Returns that structure to the app as a **draft** — never writes a
   transaction row itself.
4. The app pre-fills the "new transaction" form from the draft; the user must
   press "Save" — the existing manual-entry path, not a new auto-post path.

This ADR originally specified the Claude API and left the exact model as a
Phase 4 implementation detail. See the Status section above for the full
provider history (Claude → Biznet Gio, confirmed broken → Gemini, confirmed
working) and the two real bugs live-testing surfaced along the way.

### Cost control

Every scan costs a real API call. Because the app is free and public:
- Rate-limit scans per user per day/month (exact numbers are a Phase 4 product
  decision — the user, not the AI, sets the free-tier limit, since it is
  budget/cost policy, not an engineering fact).
- Log scan usage per user so the limit is enforceable and abuse is visible.
- Never let a failed/low-confidence extraction retry silently in a loop.

### Privacy

Receipt photos and extracted text can contain personal data (what someone
bought, where, when) — treat them with the same care as the transaction data
itself: `user_id`-scoped storage, no cross-user access, included in whatever
data-export/data-deletion flow Phase 8 builds for Play Store compliance.
Whether to retain the original photo after extraction (vs. discarding it once
the user confirms the draft) is a Phase 4 decision to make explicitly, not a
default to fall into.

### Failure handling

Low-confidence or failed extraction must fall back to a normal blank manual-entry
form with a clear message — never block the user from recording the expense
just because the AI couldn't read the receipt.

## Consequences

- The backend needs an `AI_API_KEY` (for `AI_ENDPOINT`) held as a server-side
  secret — never in the mobile app, never committed, placeholder only in
  `env.example`. Provisioning that key is a human action.
- The mobile app's "new transaction" flow is designed once (Phase 3) and reused
  by both manual entry and AI-drafted entry (Phase 4) — the draft is just a
  pre-filled form, not a parallel code path.
- This ADR intentionally does not commit to per-scan pricing or a specific
  free-tier quota — that is a business decision for the user to set when
  Phase 4 starts.
