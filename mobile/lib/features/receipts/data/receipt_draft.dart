class ReceiptLineItemDraft {
  const ReceiptLineItemDraft(
      {required this.description, required this.amountMinor});

  final String description;
  final int amountMinor;

  factory ReceiptLineItemDraft.fromJson(Map<String, dynamic> json) =>
      ReceiptLineItemDraft(
        description: json['description'] as String,
        amountMinor: json['amountMinor'] as int,
      );
}

/// What POST /receipts/scan returns — never a saved transaction, always a
/// suggestion the user reviews (see docs/adr/0002-ai-receipt-scanning.md and
/// TransactionFormPage, which is the only thing that ever calls
/// POST /transactions).
class ReceiptDraft {
  const ReceiptDraft({
    required this.merchant,
    required this.occurredAt,
    required this.currency,
    required this.totalMinor,
    required this.categoryGuess,
    required this.lineItems,
    required this.confidence,
  });

  final String merchant;
  final DateTime? occurredAt;
  final String currency;
  final int totalMinor;
  final String categoryGuess;
  final List<ReceiptLineItemDraft> lineItems;
  final double confidence;

  factory ReceiptDraft.fromJson(Map<String, dynamic> json) => ReceiptDraft(
        merchant: json['merchant'] as String,
        occurredAt: json['occurredAt'] == null
            ? null
            : DateTime.parse(json['occurredAt'] as String),
        currency: json['currency'] as String,
        totalMinor: json['totalMinor'] as int,
        categoryGuess: json['categoryGuess'] as String,
        lineItems: (json['lineItems'] as List<dynamic>? ?? [])
            .map(
                (e) => ReceiptLineItemDraft.fromJson(e as Map<String, dynamic>))
            .toList(),
        confidence: (json['confidence'] as num).toDouble(),
      );
}
