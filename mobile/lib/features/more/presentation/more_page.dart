import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../auth/application/auth_controller.dart';
import '../../auth/application/auth_state.dart';

class MorePage extends ConsumerWidget {
  const MorePage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    final user = authState is AuthAuthenticated ? authState.user : null;
    final colorScheme = Theme.of(context).colorScheme;

    return Scaffold(
      appBar: AppBar(title: const Text('More')),
      body: ListView(
        children: [
          if (user != null)
            ListTile(
              leading: const CircleAvatar(child: Icon(Icons.person_outline)),
              title: Text(user.displayName),
              subtitle: Text(user.email),
            ),
          const Divider(height: 1),
          ListTile(
            leading: const Icon(Icons.category_outlined),
            title: const Text('Categories'),
            onTap: () => context.push('/categories'),
          ),
          const Divider(height: 1),
          ListTile(
            leading: const Icon(Icons.logout),
            title: const Text('Log out'),
            onTap: () => ref.read(authControllerProvider.notifier).logout(),
          ),
          const SizedBox(height: 24),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Text(
              'Danger zone',
              style: Theme.of(context)
                  .textTheme
                  .labelLarge
                  ?.copyWith(color: colorScheme.error),
            ),
          ),
          ListTile(
            leading:
                Icon(Icons.delete_forever_outlined, color: colorScheme.error),
            title: Text('Delete my account',
                style: TextStyle(color: colorScheme.error)),
            subtitle: const Text(
                'Permanently deletes your account and all your data.'),
            onTap: () => _confirmDeleteAccount(context, ref),
          ),
        ],
      ),
    );
  }

  Future<void> _confirmDeleteAccount(
      BuildContext context, WidgetRef ref) async {
    final confirmController = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) {
          final canConfirm =
              confirmController.text.trim().toUpperCase() == 'DELETE';
          return AlertDialog(
            title: const Text('Delete your account?'),
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'This permanently deletes your account, every account/category/'
                  'transaction you have recorded, and your login credentials. '
                  'This cannot be undone.',
                ),
                const SizedBox(height: 16),
                TextField(
                  controller: confirmController,
                  autofocus: true,
                  decoration: const InputDecoration(
                      labelText: 'Type DELETE to confirm'),
                  onChanged: (_) => setDialogState(() {}),
                ),
              ],
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                child: const Text('Cancel'),
              ),
              TextButton(
                onPressed: canConfirm
                    ? () => Navigator.of(dialogContext).pop(true)
                    : null,
                style: TextButton.styleFrom(
                    foregroundColor: Theme.of(dialogContext).colorScheme.error),
                child: const Text('Delete permanently'),
              ),
            ],
          );
        },
      ),
    );

    if (confirmed != true) return;

    final error =
        await ref.read(authControllerProvider.notifier).deleteAccount();
    if (error != null && context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(error)));
    }
    // On success there is no session left — the router's auth redirect
    // (app_router.dart) takes the user to /login automatically once
    // AuthState becomes AuthUnauthenticated; no explicit navigation needed.
  }
}
