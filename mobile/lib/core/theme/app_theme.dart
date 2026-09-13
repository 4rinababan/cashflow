import 'package:flutter/material.dart';

/// A single Material 3 seed colour drives light and dark palettes, plus a
/// handful of component themes and a semantic-colour extension — the
/// "professional UI" bar (docs/roadmap.md Phase 6).
class AppTheme {
  const AppTheme._();

  static const Color _seed = Color(0xFF1B6B4F); // calm, "money app" green

  static ThemeData light() => _themeFor(Brightness.light);

  static ThemeData dark() => _themeFor(Brightness.dark);

  static ThemeData _themeFor(Brightness brightness) {
    final colorScheme =
        ColorScheme.fromSeed(seedColor: _seed, brightness: brightness);

    return ThemeData(
      useMaterial3: true,
      colorScheme: colorScheme,
      appBarTheme: AppBarTheme(
        backgroundColor: colorScheme.surface,
        foregroundColor: colorScheme.onSurface,
        elevation: 0,
        scrolledUnderElevation: 2,
      ),
      cardTheme: CardThemeData(
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: BorderSide(color: colorScheme.outlineVariant),
        ),
      ),
      inputDecorationTheme: InputDecorationTheme(
        border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
      ),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: colorScheme.surface,
        indicatorColor: colorScheme.secondaryContainer,
      ),
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: const Size.fromHeight(48),
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size.fromHeight(48),
          shape:
              RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        ),
      ),
      extensions: [MoneyColors.forBrightness(brightness)],
    );
  }
}

/// Income/expense colours as a theme extension rather than hardcoded
/// `Colors.green`/`Colors.red` — a fixed shade that looks fine on a light
/// surface can fail contrast on a dark one, so each brightness gets its own
/// pair tuned against its own surfaces (accessibility bar, Phase 6). Read it
/// with `Theme.of(context).extension<MoneyColors>()!`.
@immutable
class MoneyColors extends ThemeExtension<MoneyColors> {
  const MoneyColors({required this.income, required this.expense});

  final Color income;
  final Color expense;

  static MoneyColors forBrightness(Brightness brightness) {
    return brightness == Brightness.dark
        ? const MoneyColors(
            income: Color(0xFF7DD8A6), expense: Color(0xFFFF9B8A))
        : const MoneyColors(
            income: Color(0xFF1B6B4F), expense: Color(0xFFB3261E));
  }

  @override
  MoneyColors copyWith({Color? income, Color? expense}) => MoneyColors(
      income: income ?? this.income, expense: expense ?? this.expense);

  @override
  MoneyColors lerp(ThemeExtension<MoneyColors>? other, double t) {
    if (other is! MoneyColors) return this;
    return MoneyColors(
      income: Color.lerp(income, other.income, t)!,
      expense: Color.lerp(expense, other.expense, t)!,
    );
  }
}
