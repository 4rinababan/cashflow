import 'dart:async';

import 'package:dio/dio.dart';

import '../env/env.dart';
import 'token_storage.dart';

/// Thin wrapper over [Dio] pointed at the Cashflow backend, with one
/// interceptor that attaches the signed-in user's bearer token and
/// transparently refreshes it on a 401 (the backend's refresh tokens rotate
/// on every use — backend/internal/auth — so at most one refresh call may be
/// in flight at a time, or the second caller's stale token gets rejected;
/// [_AuthInterceptor] queues requests to guarantee that, and
/// [refreshAccessToken] itself coalesces concurrent callers too).
class ApiClient {
  ApiClient({TokenStorage? tokenStorage})
      : tokenStorage = tokenStorage ?? TokenStorage(),
        dio = Dio(
          BaseOptions(
            baseUrl: Env.apiBaseUrl,
            connectTimeout: const Duration(seconds: 10),
            // Receipt scanning calls a real AI model and can genuinely take
            // a while (see docs/adr/0002-ai-receipt-scanning.md) — this must
            // stay comfortably above the backend's own AI-call timeout.
            receiveTimeout: const Duration(seconds: 45),
          ),
        ),
        _refreshDio = Dio(
          BaseOptions(
            baseUrl: Env.apiBaseUrl,
            connectTimeout: const Duration(seconds: 10),
            receiveTimeout: const Duration(seconds: 10),
          ),
        ) {
    dio.interceptors.add(_AuthInterceptor(this));
  }

  final Dio dio;
  final TokenStorage tokenStorage;

  /// A separate, interceptor-free client for the refresh call itself — using
  /// [dio] here would recurse back into this same interceptor on a 401.
  final Dio _refreshDio;

  /// Set by the auth feature; called when a refresh attempt fails so the app
  /// can drop back to the login screen. A plain callback, not an import of
  /// the auth feature, to keep this a one-way dependency.
  void Function()? onSessionExpired;

  Completer<String?>? _refreshCompleter;

  Future<bool> isBackendReady() async {
    try {
      final response = await dio.get<Map<String, dynamic>>('/readyz');
      return response.statusCode == 200;
    } on DioException {
      return false;
    }
  }

  /// Refreshes the access token using the stored refresh token. Concurrent
  /// callers all await the same in-flight attempt rather than each firing
  /// their own refresh request.
  Future<String?> refreshAccessToken() async {
    final inFlight = _refreshCompleter;
    if (inFlight != null) {
      return inFlight.future;
    }
    final completer = Completer<String?>();
    _refreshCompleter = completer;

    try {
      final refreshToken = await tokenStorage.readRefreshToken();
      if (refreshToken == null) {
        completer.complete(null);
        return null;
      }

      final response = await _refreshDio.post<Map<String, dynamic>>(
        '/auth/refresh',
        data: {'refreshToken': refreshToken},
      );
      final body = response.data!;
      final newAccessToken = body['accessToken'] as String;
      final newRefreshToken = body['refreshToken'] as String;
      await tokenStorage.save(
          accessToken: newAccessToken, refreshToken: newRefreshToken);
      completer.complete(newAccessToken);
      return newAccessToken;
    } on DioException {
      await tokenStorage.clear();
      onSessionExpired?.call();
      completer.complete(null);
      return null;
    } finally {
      _refreshCompleter = null;
    }
  }
}

class _AuthInterceptor extends QueuedInterceptor {
  _AuthInterceptor(this.client);

  final ApiClient client;

  @override
  Future<void> onRequest(
      RequestOptions options, RequestInterceptorHandler handler) async {
    final token = await client.tokenStorage.readAccessToken();
    if (token != null) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
      DioException err, ErrorInterceptorHandler handler) async {
    final isAuthEndpoint = err.requestOptions.path.startsWith('/auth/');
    if (err.response?.statusCode == 401 && !isAuthEndpoint) {
      final newToken = await client.refreshAccessToken();
      if (newToken != null) {
        final retryOptions = err.requestOptions;
        retryOptions.headers['Authorization'] = 'Bearer $newToken';
        try {
          final response = await client.dio.fetch<dynamic>(retryOptions);
          handler.resolve(response);
          return;
        } on DioException catch (retryError) {
          handler.next(retryError);
          return;
        }
      }
    }
    handler.next(err);
  }
}
