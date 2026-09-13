import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

import '../../../core/money/money.dart';
import '../../../core/network/error_message.dart';
import '../../receipts/data/receipt_draft.dart';
import '../../reports/application/reports_providers.dart';
import '../application/ledger_providers.dart';
import '../data/ledger_models.dart';

/// Create-transaction form. Shared by manual entry and by AI receipt
/// scanning (`draft` pre-fills it) — there is no separate auto-post path;
/// the user always reviews and presses Save (docs/adr/0002-ai-receipt-scanning.md).
///
/// v1 does not support editing an existing transaction — only create and
/// delete (see TransactionsPage's swipe-to-delete). That's a real, known gap,
/// not an oversight; see mobile/README.md.
class TransactionFormPage extends ConsumerStatefulWidget {
  const TransactionFormPage({super.key, this.draft});

  final ReceiptDraft? draft;

  @override
  ConsumerState<TransactionFormPage> createState() =>
      _TransactionFormPageState();
}

class _TransactionFormPageState extends ConsumerState<TransactionFormPage> {
  final _formKey = GlobalKey<FormState>();
  final _amountController = TextEditingController();
  final _noteController = TextEditingController();

  TransactionDirection _direction = TransactionDirection.expense;
  String? _accountId;
  String? _categoryId;
  late DateTime _occurredAt;
  bool _submitting = false;

  @override
  void initState() {
    super.initState();
    final draft = widget.draft;
    _occurredAt = draft?.occurredAt ?? DateTime.now();
    if (draft != null) {
      if (draft.totalMinor > 0) {
        _amountController.text = (draft.totalMinor / 100).toStringAsFixed(0);
      }
      _noteController.text = draft.merchant;
    }
  }

  @override
  void dispose() {
    _amountController.dispose();
    _noteController.dispose();
    super.dispose();
  }

  Future<void> _pickDate() async {
    final picked = await showDatePicker(
      context: context,
      initialDate: _occurredAt,
      firstDate: DateTime(2000),
      lastDate: DateTime.now().add(const Duration(days: 1)),
    );
    if (picked != null) setState(() => _occurredAt = picked);
  }

  Future<void> _submit() async {
    if (!(_formKey.currentState?.validate() ?? false)) return;
    if (_accountId == null || _categoryId == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text('Please choose an account and a category.')),
      );
      return;
    }
    final amountMinor = Money.parseToMinor(_amountController.text);
    if (amountMinor == null || amountMinor <= 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text('Enter a valid amount greater than zero.')),
      );
      return;
    }

    setState(() => _submitting = true);
    try {
      final accounts = await ref.read(accountsProvider.future);
      final account = accounts.firstWhere((a) => a.id == _accountId);
      await ref.read(ledgerApiProvider).createTransaction(
            accountId: _accountId!,
            categoryId: _categoryId!,
            direction: _direction,
            amountMinor: amountMinor,
            currency: account.currency,
            occurredAt: _occurredAt,
            note: _noteController.text.trim(),
          );
      ref.invalidate(transactionsProvider);
      ref.invalidate(accountsProvider);
      ref.invalidate(reportSummaryProvider);
      ref.invalidate(categoryBreakdownProvider);
      ref.invalidate(monthlyTrendProvider);
      if (!mounted) return;
      if (context.canPop()) {
        context.pop();
      } else {
        context.go('/transactions');
      }
    } on DioException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(apiErrorMessage(e))));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final accountsAsync = ref.watch(accountsProvider);
    final categoriesAsync = ref.watch(categoriesProvider);

    return Scaffold(
      appBar: AppBar(
          title: Text(widget.draft != null
              ? 'Confirm Scanned Receipt'
              : 'New Transaction')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (widget.draft != null) ...[
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(12),
                    child: Row(
                      children: [
                        const Icon(Icons.auto_awesome_outlined),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            'Read from your photo (confidence '
                            '${(widget.draft!.confidence * 100).round()}%). '
                            'Please check everything before saving.',
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: 16),
              ],
              SegmentedButton<TransactionDirection>(
                segments: const [
                  ButtonSegment(
                      value: TransactionDirection.expense,
                      label: Text('Expense')),
                  ButtonSegment(
                      value: TransactionDirection.income,
                      label: Text('Income')),
                ],
                selected: {_direction},
                onSelectionChanged: (selection) => setState(() {
                  _direction = selection.first;
                  _categoryId = null;
                }),
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _amountController,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(
                    labelText: 'Amount', prefixText: 'Rp '),
                validator: (value) => Money.parseToMinor(value ?? '') == null
                    ? 'Enter a valid amount'
                    : null,
              ),
              const SizedBox(height: 16),
              accountsAsync.when(
                data: (accounts) {
                  final active = accounts.where((a) => !a.archived).toList();
                  _accountId ??= active.isNotEmpty ? active.first.id : null;
                  return DropdownButtonFormField<String>(
                    // `value` (not `initialValue`) is required here: this widget
                    // rebuilds whenever accountsAsync/_accountId change (e.g. the
                    // default-account assignment above, on first load), and
                    // `initialValue` would only seed the field once rather than
                    // reflect that.
                    // ignore: deprecated_member_use
                    value: _accountId,
                    decoration: const InputDecoration(labelText: 'Account'),
                    items: [
                      for (final account in active)
                        DropdownMenuItem(
                            value: account.id, child: Text(account.name)),
                    ],
                    onChanged: (value) => setState(() => _accountId = value),
                  );
                },
                loading: () => const LinearProgressIndicator(),
                error: (error, _) =>
                    Text('Could not load accounts: ${apiErrorMessage(error)}'),
              ),
              const SizedBox(height: 16),
              categoriesAsync.when(
                data: (categories) {
                  final matching = categories
                      .where((c) => !c.archived && c.kind == _direction)
                      .toList();
                  if (_categoryId != null &&
                      !matching.any((c) => c.id == _categoryId)) {
                    _categoryId = null;
                  }
                  _categoryId ??=
                      matching.isNotEmpty ? matching.first.id : null;
                  return DropdownButtonFormField<String>(
                    // Same reasoning as the account dropdown above: `value` must
                    // stay reactive to _categoryId, which changes on
                    // category-list reload too.
                    // ignore: deprecated_member_use
                    value: _categoryId,
                    decoration: const InputDecoration(labelText: 'Category'),
                    items: [
                      for (final category in matching)
                        DropdownMenuItem(
                            value: category.id, child: Text(category.name)),
                    ],
                    onChanged: (value) => setState(() => _categoryId = value),
                  );
                },
                loading: () => const LinearProgressIndicator(),
                error: (error, _) => Text(
                    'Could not load categories: ${apiErrorMessage(error)}'),
              ),
              const SizedBox(height: 16),
              ListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Date'),
                subtitle: Text(DateFormat('d MMMM yyyy').format(_occurredAt)),
                trailing: const Icon(Icons.edit_calendar_outlined),
                onTap: _pickDate,
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _noteController,
                decoration: const InputDecoration(labelText: 'Note (optional)'),
              ),
              const SizedBox(height: 24),
              FilledButton(
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(
                        height: 20,
                        width: 20,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Text('Save'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
