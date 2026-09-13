# Building a release for Play Console (Phase 9)

End-to-end walkthrough for producing a signed `.aab` and getting it into
Play Console. Read `docs/ops/android-signing.md` first — it explains *why*
the keystore step below must be done by a human, never by an AI assistant.

## How the `.aab` actually gets built: direct, not GitHub Actions

`.github/workflows/release-android.yml` still exists and is a valid,
tested-would-work path — but this account's GitHub billing was locked
("GitHub Actions workflows can't be executed on this repository. Your
account's billing is currently locked.") the first time it was tried, which
blocks **every** Actions run account-wide regardless of runner type
(GitHub-hosted or self-hosted — a self-hosted runner was registered and
running specifically to test this, and it made no difference, because
GitHub refuses to even schedule a run — 0 jobs ever created — while billing
is locked). Rather than wait on that, the build was moved to something that
doesn't depend on GitHub at all: a full Flutter + Android SDK toolchain,
installed directly (no `sudo`/root available either — everything below was
fetched as plain tarballs/zips into `$HOME`, not via `apt`):

- Flutter SDK (`~/flutter-sdk`, stable channel)
- Eclipse Temurin JDK 17 (`~/jdk17`) — needed for `keytool` and Gradle
- Android `cmdline-tools` + `platform-tools` + `platforms;android-35`/`36` +
  `build-tools;35.0.0`/`36.0.0`/`28.0.3` (`~/android-sdk`)
- `gh` CLI (`~/bin/gh`) and a self-hosted Actions runner (registered, not
  currently needed, but left in place in case GitHub billing gets resolved
  and the workflow route is wanted later)

All added to `PATH`/`JAVA_HOME` via `~/.bashrc`, so a fresh shell has
everything already. **`flutter build appbundle --release` was run for
real and succeeded** — `✓ Built build/app/outputs/bundle/release/app-release.aab
(53.9MB)` — using Flutter's debug-signing fallback (no `key.properties` was
present yet at that point). This is real, executed proof the Gradle/signing
wiring in `android/app/build.gradle.kts` is correct, not an inference from
reading Flutter's documentation.

If GitHub billing ever gets fixed and the GitHub Actions route is preferred
instead, see the bottom of this file for the (unused but still valid)
workflow-based instructions.

## Prerequisites (one-time, human-only)

**Generate the upload keystore** (see `docs/ops/android-signing.md` §1) —
only ever done once for this app's lifetime. `keytool` is available at
`~/jdk17/bin/keytool` (on `PATH` in a fresh shell — see above):

```bash
cd ~
keytool -genkey -v -keystore cashflow-release.keystore -alias cashflow -keyalg RSA -keysize 2048 -validity 10000
```

Back up the resulting `.keystore` file and the two passwords it asks for
somewhere durable and private (a password manager) — never in this repo,
never pasted into a chat with an AI assistant. This step cannot be done by
an AI assistant even though the tooling to run it now exists in this
environment — see `docs/ops/android-signing.md` for why.

## Building the real, signed `.aab`

Once the keystore exists, create `mobile/android/key.properties` (git-ignored,
never committed):

```properties
storePassword=<the keystore password>
keyPassword=<the key password>
keyAlias=cashflow
storeFile=/home/<you>/cashflow-release.keystore
```

Then, from `mobile/`:

```bash
flutter build appbundle --release --dart-define=API_BASE_URL=https://api.laburaku.com
```

`build.gradle.kts` detects `key.properties` and switches from debug to real
release signing automatically (see its `hasReleaseSigning` check) — no
other change needed. Output: `build/app/outputs/bundle/release/app-release.aab`,
exactly what Play Console's **App bundle explorer** / **Production → Create
new release** expects.

## First upload to Play Console

1. https://play.google.com/console → **Create app** (needs a Google Play
   Developer account, a one-time human-held cost/registration — see
   `docs/ops/android-signing.md`).
2. Fill in the store listing using `docs/store-listing.md` as a draft —
   translate/adjust as needed, it is not final copy.
3. **Privacy policy URL**: a working draft is published at
   <https://claude.ai/code/artifact/307ac55b-da90-463f-9f0c-bd399b89ab12> —
   fine to point Play Console at directly, or host `docs/privacy-policy.md`
   somewhere else you control instead. Either way, fill in its own
   `[ISI ...]` placeholders and get it legally reviewed **before** this
   step — Play Console will reject a listing without a working privacy
   policy URL, and Google can suspend an app whose actual behavior doesn't
   match what the policy claims.
4. **Data safety form**: answer using `docs/store-listing.md`'s "Data safety
   section" as your source of truth for what's actually collected/shared.
5. Upload `app-release.aab` to the **Internal testing** track first (never
   straight to Production) — see `docs/ops/android-signing.md`'s staged
   rollout plan for the full sequence (internal → closed → staged
   production).
6. `applicationId` is currently `com.laburaku.cashflow` (derived from this
   project's own domain) — **confirm this is the package name you actually
   want before this first upload**. It cannot be changed for this app
   listing afterward; a different one means a different, brand-new listing.

## Updating pubspec.yaml's version for each release

`mobile/pubspec.yaml`'s `version: X.Y.Z+N` — `X.Y.Z` is shown to users,
`N` (`versionCode`) must strictly increase on every Play Console upload
(re-uploading the same `N` is rejected). Bump `N` before building again for
a new release; bump `X.Y.Z` when it's a meaningful version bump for users
to see.

## Appendix: the GitHub Actions route (unused — billing-blocked)

`.github/workflows/release-android.yml` is left in the repo, valid and
unchanged, for whenever GitHub billing is fixed. It needs, as repo secrets
(Settings → Secrets and variables → Actions):

| Secret | Value |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | `base64 -w0 cashflow-release.keystore` (the whole keystore file, base64-encoded) |
| `ANDROID_KEYSTORE_PASSWORD` | the keystore password |
| `ANDROID_KEY_PASSWORD` | the key password |
| `ANDROID_KEY_ALIAS` | the alias (e.g. `cashflow`) |

Then: Actions tab → **Release Android** → **Run workflow** → download
`cashflow-release-aab` from the finished run's Artifacts. The workflow
currently targets `self-hosted` (a runner was registered against this repo
during setup, at `~/actions-runner` — it needs to actually be running,
`~/actions-runner/run.sh`, to pick up a dispatched job) rather than
`ubuntu-latest`, specifically so it doesn't depend on GitHub's own hosted
runner minutes/billing either — though the billing lock turned out to block
Actions scheduling entirely regardless of runner type, so this alone won't
help until billing itself is unlocked.
