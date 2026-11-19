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
}
