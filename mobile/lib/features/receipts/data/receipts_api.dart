import 'package:dio/dio.dart';
import 'package:http_parser/http_parser.dart';

import '../../../core/network/api_client.dart';
import 'receipt_draft.dart';

/// Calls POST /receipts/scan (Phase 4). This is the one endpoint that can
/// genuinely take a while (a real AI model call — see
/// docs/adr/0002-ai-receipt-scanning.md), which is why [ApiClient]'s
/// receiveTimeout is set generously.
class ReceiptsApi {
  ReceiptsApi(this._client);

  final ApiClient _client;

  Future<ReceiptDraft> scan(String filePath) async {
    final mimeType = _guessMimeType(filePath);
    final formData = FormData.fromMap({
      'image': await MultipartFile.fromFile(
        filePath,
        filename: 'receipt.${mimeType.subtype}',
        contentType: mimeType,
      ),
    });
    final response = await _client.dio
        .post<Map<String, dynamic>>('/receipts/scan', data: formData);
    return ReceiptDraft.fromJson(response.data!);
  }

  MediaType _guessMimeType(String path) {
    final lower = path.toLowerCase();
    if (lower.endsWith('.png')) return MediaType('image', 'png');
    if (lower.endsWith('.webp')) return MediaType('image', 'webp');
    return MediaType('image', 'jpeg');
  }
}
