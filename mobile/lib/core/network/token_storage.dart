import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Persists the current session's tokens in the platform keystore/keychain —
/// never SharedPreferences or a plain file. Mirrors the backend's own
/// discipline around tokens (backend/internal/auth): this app never has the
/// user's password, only these two short-lived/revocable tokens.
class TokenStorage {
  TokenStorage([FlutterSecureStorage? storage])
      : _storage = storage ?? const FlutterSecureStorage();

  static const _accessKey = 'cashflow.accessToken';
  static const _refreshKey = 'cashflow.refreshToken';

  final FlutterSecureStorage _storage;

  Future<void> save(
      {required String accessToken, required String refreshToken}) async {
    await _storage.write(key: _accessKey, value: accessToken);
    await _storage.write(key: _refreshKey, value: refreshToken);
  }

  Future<String?> readAccessToken() => _storage.read(key: _accessKey);

  Future<String?> readRefreshToken() => _storage.read(key: _refreshKey);

  Future<void> clear() async {
    await _storage.delete(key: _accessKey);
    await _storage.delete(key: _refreshKey);
  }
}
