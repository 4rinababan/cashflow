import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/offline_cache.dart';
import '../../../core/network/providers.dart';
import '../data/reports_api.dart';
import '../data/reports_models.dart';

final reportsApiProvider = Provider<ReportsApi>((ref) {
  return ReportsApi(
    ref.watch(apiClientProvider),
    onOfflineChanged: (offline) =>
        ref.read(isOfflineProvider.notifier).state = offline,
  );
});

/// Defaults to "this month so far", matching the backend's own default
/// (backend/internal/reports.Service.resolveRange).
final reportSummaryProvider = FutureProvider<ReportSummary>(
    (ref) => ref.watch(reportsApiProvider).summary());

final categoryBreakdownProvider = FutureProvider<List<CategoryBreakdown>>(
    (ref) => ref.watch(reportsApiProvider).byCategory());

/// Defaults to the last 6 months, always zero-filled for months with no
/// activity (backend/internal/reports.Service.MonthlyTrend).
final monthlyTrendProvider = FutureProvider<List<MonthlyTrendPoint>>(
    (ref) => ref.watch(reportsApiProvider).monthlyTrend());
