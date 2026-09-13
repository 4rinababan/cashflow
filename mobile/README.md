# Cashflow — mobile (Flutter)

> **Update, Phase 9 (2026-09-13): this app has now actually been verified.**
> A Flutter SDK (3.47.4 stable) was temporarily installed to check this —
> `flutter analyze`, `dart format`, and `flutter test` all now pass clean
> against the real code (a handful of real issues surfaced by a newer SDK
> than this code was originally written against were found and fixed: a
> `CardTheme`/`CardThemeData` API change, a non-const `Semantics` constructor,
> and Dart formatting across nearly every file, which had never been run
> through `dart format` before). `android/` and `ios/` are now committed,
> real, `flutter create`-generated source — not regenerated on every clone —
> with the release signing config, application ID, and camera/photo
> permissions already wired in (see below and
> `docs/ops/play-store-release.md`). What's still unverified: an actual
> `flutter build appbundle` against a real Android SDK/toolchain, which
> genuinely needs Android build tools this environment doesn't have — that
> gets its first real test the first time `.github/workflows/release-android.yml`
> runs on GitHub's own Android-equipped runners.

## First-time setup

`android/` and `ios/` are committed — no `flutter create` step needed. Just:

```bash
cd mobile
flutter pub get
flutter analyze
flutter test
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080
```

Camera/photo-library permissions (`android/app/src/main/AndroidManifest.xml`'s
`CAMERA` permission, `ios/Runner/Info.plist`'s `NSCameraUsageDescription`/
`NSPhotoLibraryUsageDescription`) are already in place — this used to be a
required manual step before `flutter create` existed as committed source;
it no longer is.

`10.0.2.2` is the Android emulator's alias for your host machine — that's where
the Go backend is listening if you started it with
`docker compose up` / `go run ./cmd/api` from the repo root. A physical device
needs your machine's real LAN IP instead.

**Production backend is live** (Phase 9, `docs/ops/production-deployment.md`):
`https://api.laburaku.com`. Point a build at it with:

```bash
flutter run --dart-define=API_BASE_URL=https://api.laburaku.com
# or for a release build:
flutter build apk --dart-define=API_BASE_URL=https://api.laburaku.com
```

Still unverified like the rest of this Flutter app — no SDK is available in
the environment that wrote it, so this has only been checked by reading
`Env.apiBaseUrl`'s own code, not by actually running a build against the
live URL above.

## What's here

- **Phase 1** (foundation): `lib/main.dart`, `lib/app.dart`, `lib/core/theme/`,
  `lib/core/env/` — unchanged since first written.
- **Phase 2** (auth): `lib/core/network/` (`ApiClient` with a queued
  interceptor that attaches the bearer token and transparently refreshes it
  on a 401 — see the class doc comment in `api_client.dart` for why refresh
  calls are coalesced, matching the backend's rotate-on-use refresh tokens),
  `lib/core/network/token_storage.dart` (`flutter_secure_storage`, never
  SharedPreferences), `lib/features/auth/` (register/login/logout,
  `AuthController` restoring a session at startup and driving the router's
  auth redirect).
- **Phase 3** (ledger): `lib/features/ledger/` — accounts (list, create,
  archive/unarchive, delete), categories (list by income/expense, create,
  archive), transactions (list, create manually, swipe-to-delete). **Editing
  an existing transaction is not built** — only create and delete; a known,
  deliberate gap to keep v1 scope manageable, not an oversight.
- **Phase 4** (AI receipt scanning): `lib/features/receipts/` — camera/gallery
  picker → `POST /receipts/scan` → hands the returned draft straight to the
  same transaction form Phase 3 built (pre-filled, still requires the user to
  review and press Save — there is no auto-post path, matching
  `docs/adr/0002-ai-receipt-scanning.md`).
- **Phase 5** (reports): `lib/features/reports/` — this-month summary card,
  a category breakdown (proportional bars), and a 6-month income/expense bar
  chart (`fl_chart`).
- **Navigation**: `lib/core/router/` — a deliberately simple flat route list
  (no `StatefulShellRoute`) with a shared `AppShell` bottom-nav widget each
  tab page wraps itself in; see the doc comment in `app_shell.dart` for the
  trade-off this makes (no per-tab back-stack memory) and why it was chosen
  (much less router plumbing to get right without a compiler to check it
  against).
- **Phase 6** (polish):
  - `lib/core/widgets/async_state_views.dart` — shared `EmptyStateView`,
    `ErrorStateView`, `LoadingView` used by every list screen instead of an
    ad hoc `Center(Text(...))`.
  - `lib/core/network/error_message.dart` — one `apiErrorMessage()` helper
    used everywhere an error reaches the UI, so the backend's own
    `{"error": "..."}` message is shown consistently and a raw
    exception/stack trace never is.
  - `lib/core/theme/app_theme.dart` — a `MoneyColors` theme extension for
    income/expense colours (tuned separately per brightness, since a shade
    that reads fine on a light surface can fail contrast on a dark one) plus
    cohesive `AppBarTheme`/`CardTheme`/button/input theming for both light
    and dark, instead of raw Material 3 defaults with hardcoded
    `Colors.green`/`Colors.red` scattered across screens.
  - `lib/core/network/offline_cache.dart` — every list screen falls back to
    the last successful response (via `shared_preferences`) when a network
    call fails, showing an `OfflineBanner` rather than a blank error screen.
    **This is a read-through cache, not a sync engine**: nothing written
    while offline is queued or retried, and a cached read is always
    overwritten by the next successful call — "don't lose sight of your own
    money because the wifi hiccuped," not full offline editing. A
    deliberate v1 scope cut, not an oversight.
  - Every interactive mutation (archive/delete/create tapped from a button,
    not just form submission) now catches and shows `apiErrorMessage` —
    a few of these (category archive/create, account archive) had no error
    handling at all before this pass and would have failed silently.
  - A few `Semantics` labels/live regions added where an icon-only control or
    a fast-changing loading state needed one for a screen reader.
- **Phase 8** (store readiness, the mobile side of it): the More screen's
  "Danger zone" now has a real "Delete my account" flow —
  `AuthApi.deleteAccount()` → `DELETE /auth/me`, gated behind a dialog that
  requires typing "DELETE" to confirm before it fires, since it's
  irreversible (`AuthController.deleteAccount`, `features/more/presentation/more_page.dart`).
  `assets/icon/icon.png` (a generated 1024×1024 app icon) plus
  `flutter_launcher_icons` config in `pubspec.yaml` — run
  `dart run flutter_launcher_icons` after `flutter pub get` to generate the
  actual Android/iOS launcher icon files from it.

## What's not here yet

- Editing an existing transaction (see above).
- Any UI to filter/paginate transactions or reports by a custom date range —
  the backend supports both (`docs/roadmap.md` Phase 3/5), the app always
  uses the default window.
- Offline **write** support (queuing a transaction created/deleted while
  offline) — see the `offline_cache.dart` note above; only reads fall back
  to cache.
- A full accessibility audit (screen reader pass on a real device, dynamic
  type/large-text layout testing) — some `Semantics` additions were made
  during Phase 6, but this hasn't been verified with an actual screen reader,
  which nothing in this environment can do either.
- A designer's pass on the app icon — the current one is a programmatically
  generated placeholder (see `docs/store-listing.md`), functional but not
  necessarily the final one.
- Real device screenshots for the Play Store listing (needs a working build
  first).

## A note on how this was verified

Every request/response shape in `lib/features/*/data/*.dart` was written by
reading the actual Go handler/DTO code in `../backend/internal/*` line by
line — not guessed — so the JSON field names and types should be exactly
right. What could **not** be checked here is whether the Dart itself compiles
and whether the widgets behave as intended on a real device/emulator. Treat
`flutter analyze` as the first real signal, not a formality.
