import '../../../core/network/api_client.dart';
import 'auth_models.dart';

/// Calls backend/internal/auth's five endpoints. Register/login/refresh
/// return the same {user, accessToken, refreshToken} shape; only the tokens
/// need attention here — [ApiClient]'s interceptor handles attaching and
/// refreshing them for every other call.
class AuthApi {
  AuthApi(this._client);

  final ApiClient _client;

  Future<AuthResult> register({
    required String email,
    required String password,
    required String displayName,
  }) async {
    final response =
        await _client.dio.post<Map<String, dynamic>>('/auth/register', data: {
      'email': email,
      'password': password,
      'displayName': displayName,
    });
    return AuthResult.fromJson(response.data!);
  }

  Future<AuthResult> login(
      {required String email, required String password}) async {
    final response =
        await _client.dio.post<Map<String, dynamic>>('/auth/login', data: {
      'email': email,
      'password': password,
    });
    return AuthResult.fromJson(response.data!);
  }

  Future<AuthUser> me() async {
    final response = await _client.dio.get<Map<String, dynamic>>('/auth/me');
    return AuthUser.fromJson(response.data!);
  }

  Future<void> logout(String refreshToken) {
    return _client.dio
        .post<void>('/auth/logout', data: {'refreshToken': refreshToken});
  }

  /// Permanently deletes the signed-in user's account and everything they
  /// own (Phase 8 self-service account deletion). Irreversible — the caller
  /// is responsible for confirming with the user first (see MorePage).
  Future<void> deleteAccount() {
    return _client.dio.delete<void>('/auth/me');
  }
}
