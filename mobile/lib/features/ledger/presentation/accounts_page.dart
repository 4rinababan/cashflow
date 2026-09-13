import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/money/money.dart';
import '../../../core/network/error_message.dart';
import '../../../core/widgets/async_state_views.dart';
import '../../../core/widgets/offline_banner.dart';
import '../application/ledger_providers.dart';
import '../data/ledger_models.dart';

class AccountsPage extends ConsumerWidget {
  const AccountsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountsAsync = ref.watch(accountsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Accounts')),
      floatingActionButton: FloatingActionButton(
        onPressed: () => context.push('/accounts/new'),
        tooltip: 'Add account',
        child: const Icon(Icons.add),
      ),
      body: Column(
        children: [
          const OfflineBanner(),
          Expanded(
            child: RefreshIndicator(
              onRefresh: () => ref.refresh(accountsProvider.future),
              child: accountsAsync.when(
                data: (accounts) {
                  if (accounts.isEmpty) {
                    return ListView(
                      children: const [
                        SizedBox(height: 80),
                        EmptyStateView(
                          icon: Icons.account_balance_wallet_outlined,
                          title: 'No accounts yet',
                          message:
                              'Add one (e.g. "Cash" or "BCA") to start tracking your balance.',
                        ),
                      ],
                    );
                  }
                  return ListView.separated(
                    padding: const EdgeInsets.all(16),
                    itemCount: accounts.length,
                    separatorBuilder: (_, __) => const SizedBox(height: 8),
                    itemBuilder: (context, index) {
                      final account = accounts[index];
                      return Card(
                        child: ListTile(
                          title: Text(account.name),
                          subtitle:
                              account.archived ? const Text('Archived') : null,
                          trailing: Text(
                            Money.format(
                                account.balanceMinor, account.currency),
                            style: Theme.of(context).textTheme.titleMedium,
                          ),
                          onTap: () =>
                              _showAccountActions(context, ref, account),
                        ),
                      );
                    },
                  );
                },
                loading: () => const LoadingView(),
                error: (error, _) => ErrorStateView(
                  error: error,
                  onRetry: () => ref.invalidate(accountsProvider),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  void _showAccountActions(
      BuildContext context, WidgetRef ref, Account account) {
    showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: Icon(account.archived
                  ? Icons.unarchive_outlined
                  : Icons.archive_outlined),
              title: Text(account.archived ? 'Unarchive' : 'Archive'),
              onTap: () async {
                Navigator.of(sheetContext).pop();
                try {
                  await ref
                      .read(ledgerApiProvider)
                      .updateAccount(account.id, archived: !account.archived);
                  ref.invalidate(accountsProvider);
                } on DioException catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(apiErrorMessage(e))));
                  }
                }
              },
            ),
            ListTile(
              leading: const Icon(Icons.delete_outline),
              title: const Text('Delete'),
              onTap: () async {
                Navigator.of(sheetContext).pop();
                try {
                  await ref.read(ledgerApiProvider).deleteAccount(account.id);
                  ref.invalidate(accountsProvider);
                } on DioException catch (e) {
                  if (context.mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text(apiErrorMessage(e))));
                  }
                }
              },
            ),
          ],
        ),
      ),
    );
  }
}
