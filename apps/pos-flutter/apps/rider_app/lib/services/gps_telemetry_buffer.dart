import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:uuid/uuid.dart';

class GpsPing {
  const GpsPing({
    required this.clientPingId,
    required this.tripId,
    required this.lat,
    required this.lng,
    required this.speedKmh,
    required this.headingDeg,
    required this.recordedAt,
  });

  final String clientPingId;
  final String tripId;
  final double lat;
  final double lng;
  final double speedKmh;
  final double headingDeg;
  final DateTime recordedAt;

  Map<String, dynamic> toJson() => {
        'client_ping_id': clientPingId,
        'trip_id': tripId,
        'lat': lat,
        'lng': lng,
        'speed_kmh': speedKmh,
        'heading_deg': headingDeg,
        'recorded_at': recordedAt.toUtc().toIso8601String(),
      };
}

/// Captures GPS pings ONLY while a trip is actively picked_up or out_for_delivery,
/// buffering up to 200 pings during cellular dead zones and flushing independently.
class GpsTelemetryBuffer {
  GpsTelemetryBuffer({this.maxBufferSize = 200});

  final int maxBufferSize;
  final _uuid = const Uuid();
  final List<GpsPing> _queue = [];

  bool _trackingActive = false;
  String? _activeTripId;

  bool get isTrackingActive => _trackingActive;
  int get bufferedCount => _queue.length;

  void updateTripState({
    required String tripId,
    required String tripStatus,
    required bool riderOnDuty,
  }) {
    final active = riderOnDuty &&
        (tripStatus == 'picked_up' || tripStatus == 'out_for_delivery');
    _trackingActive = active;
    _activeTripId = active ? tripId : null;
  }

  bool recordCoordinate({
    required double lat,
    required double lng,
    double speedKmh = 0,
    double headingDeg = 0,
  }) {
    if (!_trackingActive || _activeTripId == null) {
      return false; // Privacy guard: drop pings when not on active trip
    }
    if (_queue.length >= maxBufferSize) {
      _queue.removeAt(0);
    }
    _queue.add(GpsPing(
      clientPingId: _uuid.v4(),
      tripId: _activeTripId!,
      lat: lat,
      lng: lng,
      speedKmh: speedKmh,
      headingDeg: headingDeg,
      recordedAt: DateTime.now().toUtc(),
    ));
    return true;
  }

  List<GpsPing> takeBatch({int maxBatch = 50}) {
    final count = _queue.length < maxBatch ? _queue.length : maxBatch;
    final batch = _queue.sublist(0, count);
    _queue.removeRange(0, count);
    return batch;
  }

  /// Flushes up to 50 buffered pings to POST /api/v1/telemetry/gps/batch,
  /// re-queueing at the front if the network request fails.
  Future<int> flushToCloud({
    required String cloudApiUrl,
    required String authToken,
    http.Client? httpClient,
  }) async {
    if (_queue.isEmpty) return 0;
    final batch = takeBatch(maxBatch: 50);
    final client = httpClient ?? http.Client();
    try {
      final res = await client
          .post(
            Uri.parse('$cloudApiUrl/api/v1/telemetry/gps/batch'),
            headers: {
              'Content-Type': 'application/json',
              if (authToken.isNotEmpty) 'Authorization': 'Bearer $authToken',
            },
            body: jsonEncode({'pings': batch.map((p) => p.toJson()).toList()}),
          )
          .timeout(const Duration(seconds: 10));
      if (res.statusCode >= 200 && res.statusCode < 300) {
        return batch.length;
      }
    } catch (_) {
      // Re-queue on network failure
    }
    _queue.insertAll(0, batch);
    return 0;
  }
}
