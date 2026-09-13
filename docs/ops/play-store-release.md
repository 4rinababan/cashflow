# Building a release for Play Console (Phase 9)

End-to-end walkthrough for producing a signed `.aab` and getting it into
Play Console. Read `docs/ops/android-signing.md` first — it explains *why*
the keystore step below must be done by a human, never by an AI assistant.

## Prerequisites (one-time, human-only)

1. **A GitHub repository for this code.** As of this writing the local repo
   has never been pushed anywhere. Create a repo (`gh repo create` or via
   github.com), then:
   ```bash
   git add -A
   git commit -m "Initial commit"
   git remote add origin <your-repo-url>
   git push -u origin master
   ```
2. **Generate the upload keystore** (see `docs/ops/android-signing.md` §1) —
   only ever done once for this app's lifetime; back up the `.keystore` file
   and its passwords somewhere durable and private (a password manager),
   never in this repo.
3. **Add these as GitHub repo secrets** (Settings → Secrets and variables →
   Actions → New repository secret):
   | Secret | Value |
   | --- | --- |
   | `ANDROID_KEYSTORE_BASE64` | `base64 -w0 cashflow-release.keystore` (the whole keystore file, base64-encoded) |
   | `ANDROID_KEYSTORE_PASSWORD` | the keystore password from step 2 |
   | `ANDROID_KEY_PASSWORD` | the key password from step 2 |
   | `ANDROID_KEY_ALIAS` | the alias you chose in step 2 (e.g. `cashflow`) |
4. Optionally add a repo **variable** (not secret — it's not sensitive)
   `API_BASE_URL` if you ever need to point a release build somewhere other
   than `https://api.laburaku.com` (the workflow defaults to that URL).

None of the above can be done by an AI assistant — the keystore and its
passwords must exist only where you control them.

## Building the `.aab`

Once the prerequisites are done:

1. GitHub → your repo → **Actions** tab → **Release Android** workflow →
   **Run workflow**.
2. Wait for the `build-aab` job to go green (a few minutes).
3. Open the finished run → **Artifacts** → download `cashflow-release-aab`
   → unzip it → you have `app-release.aab`.

This is the exact file Play Console expects for **App bundle explorer** /
**Production → Create new release**.

### Why this is reliable (not a guess)

`mobile/android/` is real, committed source — generated once via
`flutter create`, then hand-edited for release signing (see
`android/app/build.gradle.kts`'s `signingConfigs` block) and verified via a
real Flutter SDK (installed temporarily for this purpose) actually running
`flutter analyze` / `flutter test` / `dart format` against this exact code,
not inferred from documentation. The workflow above only needs to supply
`key.properties` from secrets at build time — no runtime patching of a
freshly-generated Gradle file, which is a meaningfully more fragile
approach this project deliberately moved away from.

What was **not** verified locally: an actual `flutter build appbundle`
against a real Android SDK (none is installed in the environment that wrote
this — see `mobile/README.md`). The *first* real run of the `Release
Android` workflow above is therefore also the first real test of the
Gradle/signing wiring end to end. Watch it closely; if it fails, the error
will point at exactly which step, and the Gradle config that's failing is
`mobile/android/app/build.gradle.kts` — small and readable enough to fix
directly, not a black box.

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
(re-uploading the same `N` is rejected). Bump `N` before triggering the
workflow again for a new release; bump `X.Y.Z` when it's a meaningful
version bump for users to see.
