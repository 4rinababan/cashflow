import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'api_client.dart';

/// The single, app-wide [ApiClient] — every feature's API class and the auth
/// controller all read this same instance, so there is exactly one token
/// store and one refresh-in-flight guard for the whole app.
final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());
