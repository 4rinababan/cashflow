import 'package:cashflow/app.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('shows a splash screen while the session is being restored',
      (tester) async {
    // AuthController's constructor kicks off an async session-restore call
    // (see auth_controller.dart) — this test deliberately does not
    // pumpAndSettle past it, since that would require a real or mocked
    // backend. AuthInitial is the synchronous starting state, so a single
    // pump is enough to see the splash screen it produces.
    await tester.pumpWidget(const ProviderScope(child: CashflowApp()));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });
}
