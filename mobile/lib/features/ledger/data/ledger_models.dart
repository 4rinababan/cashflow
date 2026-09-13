enum TransactionDirection {
  income,
  expense;

  static TransactionDirection fromJson(String value) => value == 'income'
      ? TransactionDirection.income
      : TransactionDirection.expense;

  String toJson() => name;
}

class Account {
  const Account({
    required this.id,
    required this.name,
    required this.currency,
    required this.archived,
    required this.balanceMinor,
  });

  final String id;
  final String name;
  final String currency;
  final bool archived;
  final int balanceMinor;

  factory Account.fromJson(Map<String, dynamic> json) => Account(
        id: json['id'] as String,
        name: json['name'] as String,
        currency: json['currency'] as String,
        archived: json['archived'] as bool,
        balanceMinor: json['balanceMinor'] as int,
      );
}

class Category {
  const Category({
    required this.id,
    required this.name,
    required this.kind,
    required this.archived,
  });

  final String id;
  final String name;
  final TransactionDirection kind;
  final bool archived;

  factory Category.fromJson(Map<String, dynamic> json) => Category(
        id: json['id'] as String,
        name: json['name'] as String,
        kind: TransactionDirection.fromJson(json['kind'] as String),
        archived: json['archived'] as bool,
      );
}

class Transaction {
  const Transaction({
    required this.id,
    required this.accountId,
    required this.categoryId,
    required this.direction,
    required this.amountMinor,
    required this.currency,
    required this.occurredAt,
    required this.note,
  });

  final String id;
  final String accountId;
  final String categoryId;
  final TransactionDirection direction;
  final int amountMinor;
  final String currency;
  final DateTime occurredAt;
  final String note;

  factory Transaction.fromJson(Map<String, dynamic> json) => Transaction(
        id: json['id'] as String,
        accountId: json['accountId'] as String,
        categoryId: json['categoryId'] as String,
        direction: TransactionDirection.fromJson(json['direction'] as String),
        amountMinor: json['amountMinor'] as int,
        currency: json['currency'] as String,
        occurredAt: DateTime.parse(json['occurredAt'] as String),
        note: json['note'] as String,
      );
}

class TransactionPage {
  const TransactionPage({
    required this.items,
    required this.total,
    required this.limit,
    required this.offset,
  });

  final List<Transaction> items;
  final int total;
  final int limit;
  final int offset;

  factory TransactionPage.fromJson(Map<String, dynamic> json) =>
      TransactionPage(
        items: (json['items'] as List<dynamic>? ?? [])
            .map((e) => Transaction.fromJson(e as Map<String, dynamic>))
            .toList(),
        total: json['total'] as int,
        limit: json['limit'] as int,
        offset: json['offset'] as int,
      );
}
