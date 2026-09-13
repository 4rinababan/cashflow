import '../../../core/network/api_client.dart';
import '../../../core/network/offline_cache.dart';
import 'ledger_models.dart';

/// Calls backend/internal/ledger's accounts/categories/transactions
/// endpoints (Phase 3). Every call is scoped server-side to the signed-in
/// user; this class does no filtering of its own.
///
/// The three `list*` reads go through [withOfflineFallback] (Phase 6): on a
/// network failure they fall back to the last successful response instead
/// of throwing. Writes (create/update/delete) deliberately do not — a
/// mutation must fail loudly offline, never silently pretend to succeed.
class LedgerApi {
  LedgerApi(this._client, {this.onOfflineChanged});

  final ApiClient _client;
  final void Function(bool offline)? onOfflineChanged;
  final _cache = const OfflineCache();

  Future<List<Account>> listAccounts() async {
    final json = await withOfflineFallback(
      cache: _cache,
      key: 'accounts',
      call: () async {
        final response = await _client.dio.get<List<dynamic>>('/accounts');
        return response.data ?? [];
      },
      onOfflineChanged: onOfflineChanged,
    );
    return (json as List)
        .map((e) => Account.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Account> createAccount(
      {required String name, required String currency}) async {
    final response =
        await _client.dio.post<Map<String, dynamic>>('/accounts', data: {
      'name': name,
      'currency': currency,
    });
    return Account.fromJson(response.data!);
  }

  Future<Account> updateAccount(String id,
      {String? name, bool? archived}) async {
    final response =
        await _client.dio.patch<Map<String, dynamic>>('/accounts/$id', data: {
      if (name != null) 'name': name,
      if (archived != null) 'archived': archived,
    });
    return Account.fromJson(response.data!);
  }

  Future<void> deleteAccount(String id) =>
      _client.dio.delete<void>('/accounts/$id');

  Future<List<Category>> listCategories() async {
    final json = await withOfflineFallback(
      cache: _cache,
      key: 'categories',
      call: () async {
        final response = await _client.dio.get<List<dynamic>>('/categories');
        return response.data ?? [];
      },
      onOfflineChanged: onOfflineChanged,
    );
    return (json as List)
        .map((e) => Category.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Category> createCategory(
      {required String name, required TransactionDirection kind}) async {
    final response =
        await _client.dio.post<Map<String, dynamic>>('/categories', data: {
      'name': name,
      'kind': kind.toJson(),
    });
    return Category.fromJson(response.data!);
  }

  Future<Category> updateCategory(String id,
      {String? name, bool? archived}) async {
    final response =
        await _client.dio.patch<Map<String, dynamic>>('/categories/$id', data: {
      if (name != null) 'name': name,
      if (archived != null) 'archived': archived,
    });
    return Category.fromJson(response.data!);
  }

  Future<void> deleteCategory(String id) =>
      _client.dio.delete<void>('/categories/$id');

  Future<TransactionPage> listTransactions({
    String? accountId,
    String? categoryId,
    DateTime? from,
    DateTime? to,
    int limit = 50,
    int offset = 0,
  }) async {
    final cacheKey =
        'transactions:$accountId:$categoryId:${from == null ? '' : _dateOnly(from)}:'
        '${to == null ? '' : _dateOnly(to)}:$limit:$offset';
    final json = await withOfflineFallback(
      cache: _cache,
      key: cacheKey,
      call: () async {
        final response = await _client.dio
            .get<Map<String, dynamic>>('/transactions', queryParameters: {
          if (accountId != null) 'accountId': accountId,
          if (categoryId != null) 'categoryId': categoryId,
          if (from != null) 'from': _dateOnly(from),
          if (to != null) 'to': _dateOnly(to),
          'limit': limit,
          'offset': offset,
        });
        return response.data!;
      },
      onOfflineChanged: onOfflineChanged,
    );
    return TransactionPage.fromJson(json as Map<String, dynamic>);
  }

  Future<Transaction> createTransaction({
    required String accountId,
    required String categoryId,
    required TransactionDirection direction,
    required int amountMinor,
    required String currency,
    required DateTime occurredAt,
    String note = '',
  }) async {
    final response =
        await _client.dio.post<Map<String, dynamic>>('/transactions', data: {
      'accountId': accountId,
      'categoryId': categoryId,
      'direction': direction.toJson(),
      'amountMinor': amountMinor,
      'currency': currency,
      'occurredAt': _dateOnly(occurredAt),
      'note': note,
    });
    return Transaction.fromJson(response.data!);
  }

  Future<void> deleteTransaction(String id) =>
      _client.dio.delete<void>('/transactions/$id');

  String _dateOnly(DateTime date) =>
      '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
}
