import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/error_message.dart';
import '../../../core/network/providers.dart';
import '../../../core/network/token_storage.dart';
import '../data/auth_api.dart';
import 'auth_state.dart';

final authApiProvider =
    Provider<AuthApi>((ref) => AuthApi(ref.watch(apiClientProvider)));

final authControllerProvider =
    StateNotifierProvider<AuthController, AuthState>((ref) {
  final client = ref.watch(apiClientProvider);
  final controller =
      AuthController(ref.watch(authApiProvider), client.tokenStorage);
  client.onSessionExpired = controller.handleSessionExpired;
  return controller;
});

/// Owns the current session: restoring it at startup, login/register,
/// logout, and reacting when [ApiClient] gives up refreshing a dead
/// refresh token. The router (lib/core/router/app_router.dart) watches
/// [AuthState] to decide what a user is allowed to see.
class AuthController extends StateNotifier<AuthState> {
  AuthController(this._authApi, this._tokenStorage)
      : super(const AuthInitial()) {
    _restoreSession();
  }

  final AuthApi _authApi;
  final TokenStorage _tokenStorage;

  Future<void> _restoreSession() async {
    final accessToken = await _tokenStorage.readAccessToken();
    if (accessToken == null) {
      state = const AuthUnauthenticated();
      return;
    }
    try {
      final user = await _authApi.me();
      state = AuthAuthenticated(user);
    } on DioException {
      // ApiClient's interceptor already tried refreshing once inside me();
      // still failing here means the session is genuinely gone.
      await _tokenStorage.clear();
      state = const AuthUnauthenticated();
    }
  }

  /// Returns null on success, or a message to show the user on failure.
  Future<String?> login(
      {required String email, required String password}) async {
    try {
      final result = await _authApi.login(email: email, password: password);
      await _tokenStorage.save(
          accessToken: result.accessToken, refreshToken: result.refreshToken);
      state = AuthAuthenticated(result.user);
      return null;
    } on DioException catch (e) {
      return apiErrorMessage(e);
    }
  }

  Future<String?> register({
    required String email,
    required String password,
    required String displayName,
  }) async {
    try {
      final result = await _authApi.register(
          email: email, password: password, displayName: displayName);
      await _tokenStorage.save(
          accessToken: result.accessToken, refreshToken: result.refreshToken);
      state = AuthAuthenticated(result.user);
      return null;
    } on DioException catch (e) {
      return apiErrorMessage(e);
    }
  }

  Future<void> logout() async {
    final refreshToken = await _tokenStorage.readRefreshToken();
    if (refreshToken != null) {
      try {
        await _authApi.logout(refreshToken);
      } on DioException {
        // Best-effort: the local session is dropped regardless of whether
        // the server-side revoke call itself succeeded.
      }
    }
    await _tokenStorage.clear();
    state = const AuthUnauthenticated();
  }

  void handleSessionExpired() {
    state = const AuthUnauthenticated();
  }

  /// Permanently deletes the account (Phase 8). Returns null on success (the
  /// caller navigates away — there is no session left to be in), or a
  /// message to show the user on failure. Unlike [logout], there is no
  /// server-side token to revoke afterwards: the user row itself is gone.
  Future<String?> deleteAccount() async {
    try {
      await _authApi.deleteAccount();
      await _tokenStorage.clear();
      state = const AuthUnauthenticated();
      return null;
    } on DioException catch (e) {
      return apiErrorMessage(e);
    }
  }
}
