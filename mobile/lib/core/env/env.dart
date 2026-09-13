/// Runtime configuration read from `--dart-define` at build/run time, e.g.
/// `flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080`.
///
/// 10.0.2.2 is the Android emulator's alias for the host machine's localhost;
/// a physical device needs the host's real LAN IP instead, and a release
/// build needs the deployed backend's public URL (Phase 8).
class Env {
  const Env._();

  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://10.0.2.2:8080',
  );
}
