import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/offline_cache.dart';
import '../../../core/network/providers.dart';
import '../data/ledger_api.dart';
import '../data/ledger_models.dart';

final ledgerApiProvider = Provider<LedgerApi>((ref) {
  return LedgerApi(
    ref.watch(apiClientProvider),
    onOfflineChanged: (offline) =>
        ref.read(isOfflineProvider.notifier).state = offline,
  );
});

final accountsProvider = FutureProvider<List<Account>>(
    (ref) => ref.watch(ledgerApiProvider).listAccounts());

final categoriesProvider = FutureProvider<List<Category>>(
    (ref) => ref.watch(ledgerApiProvider).listCategories());

/// The most recent transactions (default page size, no filters) — good
/// enough for a single "Transactions" list in v1; see mobile/README.md for
/// the filtering UI this doesn't have yet.
final transactionsProvider = FutureProvider<TransactionPage>(
    (ref) => ref.watch(ledgerApiProvider).listTransactions());
