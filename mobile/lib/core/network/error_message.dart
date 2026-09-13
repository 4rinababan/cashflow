import 'package:dio/dio.dart';

/// Extracts the backend's `{"error": "..."}` message from a failed call, or
/// a plain, non-technical fallback — used everywhere an error reaches the
/// UI so a raw exception/stack trace never does (part of the Phase 6
/// "professional UI" bar). Previously this exact logic was duplicated ad hoc
/// in several presentation files; consolidated here during Phase 6.
String apiErrorMessage(Object error) {
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map && data['error'] is String) {
      return data['error'] as String;
    }
    switch (error.type) {
      case DioExceptionType.connectionError:
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.receiveTimeout:
      case DioExceptionType.sendTimeout:
        return "Can't reach the server. Check your connection and try again.";
      default:
        break;
    }
  }
  return 'Something went wrong. Please try again.';
}
