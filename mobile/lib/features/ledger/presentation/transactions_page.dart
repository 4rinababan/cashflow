import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../../core/money/money.dart';
import '../../../core/network/error_message.dart';
import '../../../core/theme/app_theme.dart';
import '../../../core/widgets/async_state_views.dart';
import '../../../core/widgets/offline_banner.dart';
import '../../reports/application/reports_providers.dart';
import '../application/ledger_providers.dart';
import '../data/ledger_models.dart';

class TransactionsPage extends ConsumerWidget {
  const TransactionsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final transactionsAsync = ref.watch(transactionsProvider);
    final categoriesAsync = ref.watch(categoriesProvider);
    final moneyColors = Theme.of(context).extension<MoneyColors>()!;

    return Scaffold(
      appBar: AppBar(title: const Text('Transactions')),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showAddOptions(context),
        tooltip: 'Add transaction',
        child: const Icon(Icons.add),
      ),
      body: Column(
        children: [
          const OfflineBanner(),
          Expanded(
            child: RefreshIndicator(
              onRefresh: () => ref.refresh(transactionsProvider.future),
              child: transactionsAsync.when(
                data: (page) {
                  if (page.items.isEmpty) {
                    return ListView(
                      children: const [
                        SizedBox(height: 80),
                        EmptyStateView(
                          icon: Icons.receipt_long_outlined,
                          title: 'No transactions yet',
                          message:
                              'Tap + to log one manually or scan a receipt.',
                        ),
                      ],
                    );
                  }
                  final categoryNames = <String, String>{
                    for (final category
                        in categoriesAsync.valueOrNull ?? const <Category>[])
                      category.id: category.name,
                  };
                  return ListView.separated(
                    itemCount: page.items.length,
                    separatorBuilder: (_, __) => const Divider(height: 1),
                    itemBuilder: (context, index) {
                      final transaction = page.items[index];
                      final isIncome =
                          transaction.direction == TransactionDirection.income;
                      final categoryName =
                          categoryNames[transaction.categoryId] ??
                              'Unknown category';
                      final amountText =
                          '${isIncome ? '+' : '-'}${Money.format(transaction.amountMinor, transaction.currency)}';
                      return Dismissible(
                        key: ValueKey(transaction.id),
                        direction: DismissDirection.endToStart,
                        background: Container(
                          color: Theme.of(context).colorScheme.errorContainer,
                          alignment: Alignment.centerRight,
                          padding: const EdgeInsets.symmetric(horizontal: 20),
                          child: Icon(Icons.delete_outline,
                              color: Theme.of(context)
                                  .colorScheme
                                  .onErrorContainer),
                        ),
                        confirmDismiss: (_) => showDialog<bool>(
                          context: context,
                          builder: (dialogContext) => AlertDialog(
                            title: const Text('Delete transaction?'),
                            content: Text('$categoryName · $amountText'),
                            actions: [
                              TextButton(
                                onPressed: () =>
                                    Navigator.of(dialogContext).pop(false),
                                child: const Text('Cancel'),
                              ),
                              TextButton(
                                onPressed: () =>
                                    Navigator.of(dialogContext).pop(true),
                                child: const Text('Delete'),
                              ),
                            ],
                          ),
                        ),
                        onDismissed: (_) async {
                          try {
                            await ref
                                .read(ledgerApiProvider)
                                .deleteTransaction(transaction.id);
                            ref.invalidate(accountsProvider);
                            ref.invalidate(reportSummaryProvider);
                            ref.invalidate(categoryBreakdownProvider);
                            ref.invalidate(monthlyTrendProvider);
                          } on DioException catch (e) {
                            if (context.mounted) {
                              ScaffoldMessenger.of(context).showSnackBar(
                                  SnackBar(content: Text(apiErrorMessage(e))));
                            }
                          } finally {
                            // Re-fetch regardless of outcome: on success this just
                            // confirms the removal; on failure it brings the row
                            // back, since Dismissible already removed it from view
                            // but the delete itself did not actually happen.
                            ref.invalidate(transactionsProvider);
                          }
                        },
                        child: Semantics(
                          label:
                              '$categoryName, ${isIncome ? 'income' : 'expense'} $amountText, '
                              '${DateFormat('d MMMM yyyy').format(transaction.occurredAt)}. Swipe to delete.',
                          excludeSemantics: true,
                          child: ListTile(
                            title: Text(categoryName),
                            subtitle: Text(
                              '${DateFormat('d MMM yyyy').format(transaction.occurredAt)}'
                              '${transaction.note.isNotEmpty ? ' · ${transaction.note}' : ''}',
                            ),
                            trailing: Text(
                              amountText,
                              style: TextStyle(
                                color: isIncome
                                    ? moneyColors.income
                                    : moneyColors.expense,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ),
                        ),
                      );
                    },
                  );
                },
                loading: () => const LoadingView(),
                error: (error, _) => ErrorStateView(
                  error: error,
                  onRetry: () => ref.invalidate(transactionsProvider),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showAddOptions(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.edit_outlined),
              title: const Text('Enter manually'),
              onTap: () {
                Navigator.of(sheetContext).pop();
                context.push('/transactions/new');
              },
            ),
            ListTile(
              leading: const Icon(Icons.camera_alt_outlined),
              title: const Text('Scan a receipt'),
              onTap: () {
                Navigator.of(sheetContext).pop();
                context.push('/transactions/scan');
              },
            ),
          ],
        ),
      ),
    );
  }
}
