# Android signing & staged rollout (Phase 8)

**No signing key exists. This is documentation only — the AI does not
generate, hold, or have access to a real Android signing keystore, and must
not create one. Generating and safeguarding it is a human action**, per
`policies/APPROVALS.md` and the standing rule that production credentials
are human-held.

## Why a human has to do this part

A Play Store release is signed with a keystore only the developer holds.
Losing it means losing the ability to ever publish an update to the same
app listing again — Google cannot recover or reissue it. It must be
generated on a machine the human controls, backed up somewhere durable and
private (a password manager or encrypted storage — never committed to this
repo, never pasted into a chat with an AI assistant), and referenced from
the build **only** via local, git-ignored files.

## The steps (for the human to run)

1. Generate the keystore (only once, ever, for this app):
   ```bash
   keytool -genkey -v -keystore cashflow-release.keystore \
     -alias cashflow -keyalg RSA -keysize 2048 -validity 10000
   ```
   Store the resulting `.keystore` file and the passwords it asks for
   somewhere safe and backed up — **not** in this repo.

2. Create `android/key.properties` (after `flutter create` has generated the
   `android/` folder — see `mobile/README.md`) — this file must be
   git-ignored, never committed:
   ```properties
   storePassword=<the keystore password>
   keyPassword=<the key password>
   keyAlias=cashflow
   storeFile=<absolute path to cashflow-release.keystore>
   ```

3. Wire `android/app/build.gradle` to read `key.properties` and use it for
   the `release` signing config — this is standard Flutter/Android
   boilerplate, documented at
   <https://docs.flutter.dev/deployment/android#signing-the-app> (not
   reproduced here since the exact Gradle syntax depends on the Flutter/AGP
   version `flutter create` generates, which this environment cannot check).

4. Build the release artifact:
   ```bash
   flutter build appbundle --release
   ```

## Staged rollout plan

Once signing is set up and the app is otherwise ready (Definition of Done
in `docs/roadmap.md`):

1. **Internal testing** track — the developer and a few trusted people,
   real devices, for at least a few days of real usage (not just a smoke
   test) before moving on.
2. **Closed testing** track — a slightly wider group, opt-in via a Play
   Store testing link. Watch for crash reports and real-world receipt-scan
   results (Gemini's actual accuracy has never been checked against more
   than one synthetic photo — see `docs/adr/0002-ai-receipt-scanning.md`;
   this is the first point where that gets real signal).
3. **Production, staged rollout** — Play Console supports releasing to a
   percentage of users first (e.g. 10% → 50% → 100%) rather than
   everyone at once. Use it. Hold each stage for at least 24–48 hours and
   watch the Play Console crash/ANR dashboard before increasing it.
4. Only after a full-percentage production release with no red flags is
   this actually "launched."

None of these stages should be entered without the human explicitly
deciding to — this document describes the plan, it does not authorize
executing it.
