import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/money/money.dart';
import '../../../core/theme/app_theme.dart';
import '../../../core/widgets/async_state_views.dart';
import '../../../core/widgets/offline_banner.dart';
import '../application/reports_providers.dart';
import '../data/reports_models.dart';

class ReportsPage extends ConsumerWidget {
  const ReportsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final summaryAsync = ref.watch(reportSummaryProvider);
    final categoryAsync = ref.watch(categoryBreakdownProvider);
    final trendAsync = ref.watch(monthlyTrendProvider);
    final moneyColors = Theme.of(context).extension<MoneyColors>()!;

    return Scaffold(
      appBar: AppBar(title: const Text('Reports')),
      body: Column(
        children: [
          const OfflineBanner(),
          Expanded(
            child: RefreshIndicator(
              onRefresh: () async {
                ref.invalidate(reportSummaryProvider);
                ref.invalidate(categoryBreakdownProvider);
                ref.invalidate(monthlyTrendProvider);
              },
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  Text('This Month',
                      style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 8),
                  summaryAsync.when(
                    data: (summary) => _SummaryCard(
                        summary: summary, moneyColors: moneyColors),
                    loading: () => const Padding(
                        padding: EdgeInsets.all(16), child: LoadingView()),
                    error: (error, _) => ErrorStateView(
                        error: error,
                        onRetry: () => ref.invalidate(reportSummaryProvider)),
                  ),
                  const SizedBox(height: 24),
                  Text('By Category',
                      style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 8),
                  categoryAsync.when(
                    data: (categories) => _CategoryBreakdownList(
                        categories: categories, moneyColors: moneyColors),
                    loading: () => const Padding(
                        padding: EdgeInsets.all(16), child: LoadingView()),
                    error: (error, _) => ErrorStateView(
                        error: error,
                        onRetry: () =>
                            ref.invalidate(categoryBreakdownProvider)),
                  ),
                  const SizedBox(height: 24),
                  Text('Last 6 Months',
                      style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 8),
                  trendAsync.when(
                    data: (points) => _MonthlyTrendChart(
                        points: points, moneyColors: moneyColors),
                    loading: () => const Padding(
                        padding: EdgeInsets.all(16), child: LoadingView()),
                    error: (error, _) => ErrorStateView(
                        error: error,
                        onRetry: () => ref.invalidate(monthlyTrendProvider)),
                  ),
                  const SizedBox(height: 24),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.summary, required this.moneyColors});

  final ReportSummary summary;
  final MoneyColors moneyColors;

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            _StatColumn(
                label: 'Income',
                value: summary.incomeMinor,
                color: moneyColors.income),
            _StatColumn(
                label: 'Expense',
                value: summary.expenseMinor,
                color: moneyColors.expense),
            _StatColumn(
                label: 'Net',
                value: summary.netMinor,
                color: Theme.of(context).colorScheme.primary),
          ],
        ),
      ),
    );
  }
}

class _StatColumn extends StatelessWidget {
  const _StatColumn(
      {required this.label, required this.value, required this.color});

  final String label;
  final int value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: '$label: ${Money.format(value, 'IDR')}',
      excludeSemantics: true,
      child: Column(
        children: [
          Text(label, style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 4),
          Text(Money.format(value, 'IDR'),
              style: TextStyle(color: color, fontWeight: FontWeight.bold)),
        ],
      ),
    );
  }
}

class _CategoryBreakdownList extends StatelessWidget {
  const _CategoryBreakdownList(
      {required this.categories, required this.moneyColors});

  final List<CategoryBreakdown> categories;
  final MoneyColors moneyColors;

  @override
  Widget build(BuildContext context) {
    if (categories.isEmpty) {
      return const EmptyStateView(
        icon: Icons.pie_chart_outline,
        title: 'No activity this month yet',
      );
    }
    final maxTotal =
        categories.map((c) => c.totalMinor).reduce((a, b) => a > b ? a : b);
    return Column(
      children: [
        for (final category in categories)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 6),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(category.categoryName),
                    Text(Money.format(category.totalMinor, 'IDR')),
                  ],
                ),
                const SizedBox(height: 4),
                ClipRRect(
                  borderRadius: BorderRadius.circular(4),
                  child: LinearProgressIndicator(
                    value: maxTotal == 0 ? 0 : category.totalMinor / maxTotal,
                    minHeight: 6,
                    color: category.kind == 'income'
                        ? moneyColors.income
                        : moneyColors.expense,
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

class _MonthlyTrendChart extends StatelessWidget {
  const _MonthlyTrendChart({required this.points, required this.moneyColors});

  final List<MonthlyTrendPoint> points;
  final MoneyColors moneyColors;

  @override
  Widget build(BuildContext context) {
    if (points.isEmpty) {
      return const EmptyStateView(
          icon: Icons.show_chart, title: 'Not enough data yet');
    }
    final maxValue = points
        .expand((p) => [p.incomeMinor, p.expenseMinor])
        .fold<int>(0, (max, v) => v > max ? v : max);
    final maxY = maxValue == 0 ? 100.0 : (maxValue / 100) * 1.2;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          height: 220,
          child: BarChart(
            BarChartData(
              maxY: maxY,
              alignment: BarChartAlignment.spaceAround,
              titlesData: FlTitlesData(
                leftTitles:
                    const AxisTitles(sideTitles: SideTitles(showTitles: false)),
                rightTitles:
                    const AxisTitles(sideTitles: SideTitles(showTitles: false)),
                topTitles:
                    const AxisTitles(sideTitles: SideTitles(showTitles: false)),
                bottomTitles: AxisTitles(
                  sideTitles: SideTitles(
                    showTitles: true,
                    getTitlesWidget: (value, meta) {
                      final index = value.toInt();
                      if (index < 0 || index >= points.length) {
                        return const SizedBox.shrink();
                      }
                      final month = points[index].month;
                      return Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Text(
                          month.length >= 7 ? month.substring(5) : month,
                          style: const TextStyle(fontSize: 10),
                        ),
                      );
                    },
                  ),
                ),
              ),
              borderData: FlBorderData(show: false),
              gridData: const FlGridData(show: false),
              barGroups: [
                for (var i = 0; i < points.length; i++)
                  BarChartGroupData(
                    x: i,
                    barRods: [
                      BarChartRodData(
                          toY: points[i].incomeMinor / 100,
                          color: moneyColors.income,
                          width: 8),
                      BarChartRodData(
                          toY: points[i].expenseMinor / 100,
                          color: moneyColors.expense,
                          width: 8),
                    ],
                  ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            _LegendDot(color: moneyColors.income),
            const SizedBox(width: 4),
            const Text('Income'),
            const SizedBox(width: 16),
            _LegendDot(color: moneyColors.expense),
            const SizedBox(width: 4),
            const Text('Expense'),
          ],
        ),
      ],
    );
  }
}

class _LegendDot extends StatelessWidget {
  const _LegendDot({required this.color});

  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 10,
      height: 10,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
    );
  }
}
