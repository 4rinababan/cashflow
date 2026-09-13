import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../../core/network/error_message.dart';
import '../../../core/network/providers.dart';
import '../data/receipts_api.dart';

/// The entry point for AI receipt scanning. It only ever produces a
/// [ReceiptDraft] to hand to TransactionFormPage — it never saves anything
/// itself (docs/adr/0002-ai-receipt-scanning.md).
class ScanReceiptPage extends ConsumerStatefulWidget {
  const ScanReceiptPage({super.key});

  @override
  ConsumerState<ScanReceiptPage> createState() => _ScanReceiptPageState();
}

class _ScanReceiptPageState extends ConsumerState<ScanReceiptPage> {
  bool _loading = false;

  Future<void> _pickAndScan(ImageSource source) async {
    final picker = ImagePicker();
    final XFile? picked;
    try {
      picked = await picker.pickImage(source: source, imageQuality: 85);
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text(
                'Could not open the camera/gallery. Check app permissions.')),
      );
      return;
    }
    if (picked == null) return;

    setState(() => _loading = true);
    try {
      final api = ReceiptsApi(ref.read(apiClientProvider));
      final draft = await api.scan(picked.path);
      if (!mounted) return;
      context.pushReplacement('/transactions/new', extra: draft);
    } on DioException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(apiErrorMessage(e))));
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Scan Receipt')),
      body: Center(
        child: _loading
            ? Semantics(
                liveRegion: true,
                child: const Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    CircularProgressIndicator(),
                    SizedBox(height: 16),
                    Text('Reading your receipt…'),
                  ],
                ),
              )
            : Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Icon(Icons.receipt_long_outlined, size: 64),
                    const SizedBox(height: 16),
                    const Text(
                      'Take a photo of a receipt, or pick one from your gallery. '
                      'You will always get to review and edit before saving.',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 24),
                    FilledButton.icon(
                      onPressed: () => _pickAndScan(ImageSource.camera),
                      icon: const Icon(Icons.camera_alt_outlined),
                      label: const Text('Take Photo'),
                    ),
                    const SizedBox(height: 12),
                    OutlinedButton.icon(
                      onPressed: () => _pickAndScan(ImageSource.gallery),
                      icon: const Icon(Icons.photo_library_outlined),
                      label: const Text('Choose from Gallery'),
                    ),
                    const SizedBox(height: 12),
                    TextButton(
                      onPressed: () =>
                          context.pushReplacement('/transactions/new'),
                      child: const Text('Enter manually instead'),
                    ),
                  ],
                ),
              ),
      ),
    );
  }
}
