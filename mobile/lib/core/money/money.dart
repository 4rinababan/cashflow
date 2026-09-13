import 'package:intl/intl.dart';

/// Amounts everywhere in this app are fixed-point integers in the currency's
/// minor unit — exactly what the backend stores and returns (see
/// backend/docs/business-rules.md: "never a float, so rounding errors can't
/// creep in"). These helpers are the only place that boundary is crossed, for
/// display and for parsing what a user types.
class Money {
  const Money._();

  static String format(int minor, String currency) {
    final normalized = currency.toUpperCase();
    final decimalDigits = normalized == 'IDR' ? 0 : 2;
    final major = minor / 100;
    final formatter = NumberFormat.currency(
      locale: 'id_ID',
      symbol: _symbolFor(normalized),
      decimalDigits: decimalDigits,
    );
    return formatter.format(major);
  }

  static String _symbolFor(String currency) {
    if (currency == 'IDR') return 'Rp';
    return '$currency ';
  }

  /// Parses a user-typed amount into minor units, or null if unparseable.
  /// Accepts a plain whole amount ("35000", "35.000" as a thousands
  /// separator) or a decimal amount with exactly two trailing digits after a
  /// separator ("12.50", "12,50"), which is treated as cents.
  static int? parseToMinor(String input) {
    final trimmed = input.trim();
    if (trimmed.isEmpty) return null;

    final decimalMatch =
        RegExp(r'^([0-9.,]*[0-9])[.,](\d{2})$').firstMatch(trimmed);
    if (decimalMatch != null) {
      final wholePart = decimalMatch.group(1)!.replaceAll(RegExp(r'[.,]'), '');
      final centsPart = decimalMatch.group(2)!;
      final whole = int.tryParse(wholePart);
      final cents = int.tryParse(centsPart);
      if (whole == null || cents == null) return null;
      return whole * 100 + cents;
    }

    final digitsOnly = trimmed.replaceAll(RegExp(r'[^0-9]'), '');
    if (digitsOnly.isEmpty) return null;
    final whole = int.tryParse(digitsOnly);
    if (whole == null) return null;
    return whole * 100;
  }
}
