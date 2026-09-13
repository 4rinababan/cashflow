import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/error_message.dart';
import '../../../core/widgets/async_state_views.dart';
import '../../../core/widgets/offline_banner.dart';
import '../application/ledger_providers.dart';
import '../data/ledger_models.dart';

class CategoriesPage extends ConsumerWidget {
  const CategoriesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final categoriesAsync = ref.watch(categoriesProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Categories')),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showCreateDialog(context, ref),
        tooltip: 'Add category',
        child: const Icon(Icons.add),
      ),
      body: Column(
        children: [
          const OfflineBanner(),
          Expanded(
            child: categoriesAsync.when(
              data: (categories) {
                final income = categories
                    .where((c) => c.kind == TransactionDirection.income)
                    .toList();
                final expense = categories
                    .where((c) => c.kind == TransactionDirection.expense)
                    .toList();
                return ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    Text('Income',
                        style: Theme.of(context).textTheme.titleMedium),
                    if (income.isEmpty)
                      const Padding(
                          padding: EdgeInsets.all(8),
                          child: Text('No income categories yet.')),
                    for (final category in income)
                      _CategoryTile(category: category),
                    const SizedBox(height: 24),
                    Text('Expense',
                        style: Theme.of(context).textTheme.titleMedium),
                    if (expense.isEmpty)
                      const Padding(
                          padding: EdgeInsets.all(8),
                          child: Text('No expense categories yet.')),
                    for (final category in expense)
                      _CategoryTile(category: category),
                  ],
                );
              },
              loading: () => const LoadingView(),
              error: (error, _) => ErrorStateView(
                  error: error,
                  onRetry: () => ref.invalidate(categoriesProvider)),
            ),
          ),
        ],
      ),
    );
  }

  void _showCreateDialog(BuildContext context, WidgetRef ref) {
    final nameController = TextEditingController();
    var kind = TransactionDirection.expense;

    showDialog<void>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          title: const Text('New Category'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                  controller: nameController,
                  decoration: const InputDecoration(labelText: 'Name')),
              const SizedBox(height: 16),
              SegmentedButton<TransactionDirection>(
                segments: const [
                  ButtonSegment(
                      value: TransactionDirection.expense,
                      label: Text('Expense')),
                  ButtonSegment(
                      value: TransactionDirection.income,
                      label: Text('Income')),
                ],
                selected: {kind},
                onSelectionChanged: (selection) =>
                    setDialogState(() => kind = selection.first),
              ),
            ],
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(),
                child: const Text('Cancel')),
            FilledButton(
              onPressed: () async {
                final name = nameController.text.trim();
                if (name.isEmpty) return;
                Navigator.of(dialogContext).pop();
                try {
                  await ref
                      .read(ledgerApiProvider)
                      .createCategory(name: name, kind: kind);
                  ref.invalidate(categoriesProvider);
                } on DioException catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(apiErrorMessage(e))));
                  }
                }
              },
              child: const Text('Create'),
            ),
          ],
        ),
      ),
    );
  }
}

class _CategoryTile extends ConsumerWidget {
  const _CategoryTile({required this.category});

  final Category category;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ListTile(
      title: Text(category.name),
      subtitle: category.archived ? const Text('Archived') : null,
      trailing: IconButton(
        icon: Icon(category.archived
            ? Icons.unarchive_outlined
            : Icons.archive_outlined),
        tooltip: category.archived ? 'Unarchive' : 'Archive',
        onPressed: () async {
          try {
            await ref
                .read(ledgerApiProvider)
                .updateCategory(category.id, archived: !category.archived);
            ref.invalidate(categoriesProvider);
          } on DioException catch (e) {
            if (context.mounted) {
              ScaffoldMessenger.of(context)
                  .showSnackBar(SnackBar(content: Text(apiErrorMessage(e))));
            }
          }
        },
      ),
    );
  }
}
