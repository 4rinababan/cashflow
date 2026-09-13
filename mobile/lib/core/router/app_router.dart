import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/application/auth_controller.dart';
import '../../features/auth/application/auth_state.dart';
import '../../features/auth/presentation/login_page.dart';
import '../../features/auth/presentation/register_page.dart';
import '../../features/ledger/presentation/account_form_page.dart';
import '../../features/ledger/presentation/accounts_page.dart';
import '../../features/ledger/presentation/categories_page.dart';
import '../../features/ledger/presentation/transaction_form_page.dart';
import '../../features/ledger/presentation/transactions_page.dart';
import '../../features/receipts/data/receipt_draft.dart';
import '../../features/receipts/presentation/scan_receipt_page.dart';
import '../../features/reports/presentation/reports_page.dart';
import '../../features/more/presentation/more_page.dart';
import 'app_shell.dart';

/// One flat route list — see app_shell.dart for why there's no
/// StatefulShellRoute. [routerProvider] rebuilds the whole [GoRouter] when
/// [AuthState] changes, which is simpler to reason about correctly than
/// wiring a `refreshListenable`, at the cost of being a slightly heavier
/// rebuild — fine at this app's scale.
final routerProvider = Provider<GoRouter>((ref) {
  final authState = ref.watch(authControllerProvider);

  return GoRouter(
    initialLocation: '/transactions',
    redirect: (context, state) {
      // AuthInitial (restoring a session) is handled entirely by
      // lib/app.dart showing a splash screen instead of the router — by the
      // time this runs, authState is always either Authenticated or not.
      final loggedIn = authState is AuthAuthenticated;
      final loggingIn = state.matchedLocation == '/login' ||
          state.matchedLocation == '/register';

      if (!loggedIn && !loggingIn) return '/login';
      if (loggedIn && loggingIn) return '/transactions';
      return null;
    },
    routes: [
      GoRoute(path: '/login', builder: (context, state) => const LoginPage()),
      GoRoute(
          path: '/register', builder: (context, state) => const RegisterPage()),
      GoRoute(
        path: '/transactions',
        builder: (context, state) =>
            const AppShell(currentIndex: 0, child: TransactionsPage()),
      ),
      GoRoute(
        path: '/transactions/new',
        builder: (context, state) =>
            TransactionFormPage(draft: state.extra as ReceiptDraft?),
      ),
      GoRoute(
          path: '/transactions/scan',
          builder: (context, state) => const ScanReceiptPage()),
      GoRoute(
        path: '/accounts',
        builder: (context, state) =>
            const AppShell(currentIndex: 1, child: AccountsPage()),
      ),
      GoRoute(
          path: '/accounts/new',
          builder: (context, state) => const AccountFormPage()),
      GoRoute(
        path: '/reports',
        builder: (context, state) =>
            const AppShell(currentIndex: 2, child: ReportsPage()),
      ),
      GoRoute(
        path: '/more',
        builder: (context, state) =>
            const AppShell(currentIndex: 3, child: MorePage()),
      ),
      GoRoute(
          path: '/categories',
          builder: (context, state) => const CategoriesPage()),
    ],
  );
});
