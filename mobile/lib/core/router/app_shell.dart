import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Bottom-nav chrome shared by the four top-level tabs. Each tab page keeps
/// its own [Scaffold] (own AppBar/FAB) nested inside this one's body — a
/// deliberately simpler alternative to go_router's StatefulShellRoute: every
/// route lives in a single flat list (see app_router.dart), which keeps
/// "push a full-screen form on top of a tab" trivial (a plain
/// `context.push`, no navigator-key plumbing) at the cost of each tab not
/// remembering its own scroll/nav position when you switch away and back.
/// Fine for v1; revisit if that trade-off starts to matter.
class AppShell extends StatelessWidget {
  const AppShell({super.key, required this.currentIndex, required this.child});

  final int currentIndex;
  final Widget child;

  static const _tabs = [
    _Tab(
        label: 'Transactions',
        icon: Icons.receipt_long_outlined,
        path: '/transactions'),
    _Tab(
        label: 'Accounts',
        icon: Icons.account_balance_wallet_outlined,
        path: '/accounts'),
    _Tab(label: 'Reports', icon: Icons.bar_chart_outlined, path: '/reports'),
    _Tab(label: 'More', icon: Icons.more_horiz, path: '/more'),
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: child,
      bottomNavigationBar: NavigationBar(
        selectedIndex: currentIndex,
        onDestinationSelected: (index) => context.go(_tabs[index].path),
        destinations: [
          for (final tab in _tabs)
            NavigationDestination(icon: Icon(tab.icon), label: tab.label),
        ],
      ),
    );
  }
}

class _Tab {
  const _Tab({required this.label, required this.icon, required this.path});

  final String label;
  final IconData icon;
  final String path;
}
