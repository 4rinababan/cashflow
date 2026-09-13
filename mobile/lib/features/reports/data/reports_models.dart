class ReportSummary {
  const ReportSummary({
    required this.from,
    required this.to,
    required this.incomeMinor,
    required this.expenseMinor,
    required this.netMinor,
    required this.transactionCount,
  });

  final String from;
  final String to;
  final int incomeMinor;
  final int expenseMinor;
  final int netMinor;
  final int transactionCount;

  factory ReportSummary.fromJson(Map<String, dynamic> json) => ReportSummary(
        from: json['from'] as String,
        to: json['to'] as String,
        incomeMinor: json['incomeMinor'] as int,
        expenseMinor: json['expenseMinor'] as int,
        netMinor: json['netMinor'] as int,
        transactionCount: json['transactionCount'] as int,
      );
}

class CategoryBreakdown {
  const CategoryBreakdown({
    required this.categoryId,
    required this.categoryName,
    required this.kind,
    required this.totalMinor,
    required this.transactionCount,
  });

  final String categoryId;
  final String categoryName;
  final String kind;
  final int totalMinor;
  final int transactionCount;

  factory CategoryBreakdown.fromJson(Map<String, dynamic> json) =>
      CategoryBreakdown(
        categoryId: json['categoryId'] as String,
        categoryName: json['categoryName'] as String,
        kind: json['kind'] as String,
        totalMinor: json['totalMinor'] as int,
        transactionCount: json['transactionCount'] as int,
      );
}

class MonthlyTrendPoint {
  const MonthlyTrendPoint({
    required this.month,
    required this.incomeMinor,
    required this.expenseMinor,
    required this.netMinor,
  });

  final String month;
  final int incomeMinor;
  final int expenseMinor;
  final int netMinor;

  factory MonthlyTrendPoint.fromJson(Map<String, dynamic> json) =>
      MonthlyTrendPoint(
        month: json['month'] as String,
        incomeMinor: json['incomeMinor'] as int,
        expenseMinor: json['expenseMinor'] as int,
        netMinor: json['netMinor'] as int,
      );
}
