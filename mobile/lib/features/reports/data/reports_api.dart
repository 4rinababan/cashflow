import '../../../core/network/api_client.dart';
import '../../../core/network/offline_cache.dart';
import 'reports_models.dart';

/// Calls backend/internal/reports (Phase 5) — all three endpoints are
/// read-only aggregates over the ledger, and all three go through
/// [withOfflineFallback] (Phase 6): on a network failure they fall back to
/// the last successful response instead of throwing.
class ReportsApi {
  ReportsApi(this._client, {this.onOfflineChanged});

  final ApiClient _client;
  final void Function(bool offline)? onOfflineChanged;
  final _cache = const OfflineCache();

  Future<ReportSummary> summary({DateTime? from, DateTime? to}) async {
    final key =
        'reports:summary:${from == null ? '' : _dateOnly(from)}:${to == null ? '' : _dateOnly(to)}';
    final json = await withOfflineFallback(
      cache: _cache,
      key: key,
      call: () async {
        final response = await _client.dio
            .get<Map<String, dynamic>>('/reports/summary', queryParameters: {
          if (from != null) 'from': _dateOnly(from),
          if (to != null) 'to': _dateOnly(to),
        });
        return response.data!;
      },
      onOfflineChanged: onOfflineChanged,
    );
    return ReportSummary.fromJson(json as Map<String, dynamic>);
  }

  Future<List<CategoryBreakdown>> byCategory(
      {DateTime? from, DateTime? to}) async {
    final key =
        'reports:byCategory:${from == null ? '' : _dateOnly(from)}:${to == null ? '' : _dateOnly(to)}';
    final json = await withOfflineFallback(
      cache: _cache,
      key: key,
      call: () async {
        final response = await _client.dio.get<Map<String, dynamic>>(
            '/reports/by-category',
            queryParameters: {
              if (from != null) 'from': _dateOnly(from),
              if (to != null) 'to': _dateOnly(to),
            });
        return response.data!;
      },
      onOfflineChanged: onOfflineChanged,
    );
    final categories =
        (json as Map<String, dynamic>)['categories'] as List<dynamic>? ?? [];
    return categories
        .map((e) => CategoryBreakdown.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<List<MonthlyTrendPoint>> monthlyTrend({int months = 6}) async {
    final json = await withOfflineFallback(
      cache: _cache,
      key: 'reports:monthlyTrend:$months',
      call: () async {
        final response = await _client.dio.get<List<dynamic>>(
          '/reports/monthly-trend',
          queryParameters: {'months': months},
        );
        return response.data ?? [];
      },
      onOfflineChanged: onOfflineChanged,
    );
    return (json as List)
        .map((e) => MonthlyTrendPoint.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  String _dateOnly(DateTime date) =>
      '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
}
