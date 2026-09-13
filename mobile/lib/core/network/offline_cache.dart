import 'dart:async';
import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// True while a screen is showing cached data because its last real network
/// call failed — see [withOfflineFallback]. Watched by [OfflineBanner].
final isOfflineProvider = StateProvider<bool>((ref) => false);

/// A tiny disk-backed cache of raw JSON API responses, so a momentary
/// network blip shows the last real data (marked stale via
/// [isOfflineProvider]) instead of a blank error screen — the
/// "offline-friendly" bar in docs/roadmap.md Phase 6.
///
/// Deliberately NOT a sync engine: nothing written while offline is queued,
/// and a cached read is always overwritten by the next successful network
/// call. This is "don't lose sight of your own money because the wifi
/// hiccuped," not full offline editing — a conscious v1 scope cut, not an
/// oversight (see mobile/README.md).
class OfflineCache {
  const OfflineCache();

  static const _prefix = 'cashflow.cache.';

  Future<void> put(String key, dynamic json) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('$_prefix$key', jsonEncode(json));
  }

  Future<dynamic> get(String key) async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString('$_prefix$key');
    if (raw == null) return null;
    try {
      return jsonDecode(raw);
    } catch (_) {
      return null;
    }
  }
}

/// Runs [call] (expected to return JSON-decoded data — a `Map`/`List`/etc.),
/// caching the result under [key] on success and reporting online via
/// [onOfflineChanged]. On failure, falls back to whatever was last cached
/// under [key], reporting offline via [onOfflineChanged] — or rethrows the
/// original error if nothing has ever been cached for [key] (e.g. the very
/// first launch with no connection).
Future<dynamic> withOfflineFallback({
  required OfflineCache cache,
  required String key,
  required Future<dynamic> Function() call,
  void Function(bool offline)? onOfflineChanged,
}) async {
  try {
    final json = await call();
    onOfflineChanged?.call(false);
    unawaited(cache.put(key, json));
    return json;
  } catch (error) {
    final cached = await cache.get(key);
    if (cached != null) {
      onOfflineChanged?.call(true);
      return cached;
    }
    rethrow;
  }
}
