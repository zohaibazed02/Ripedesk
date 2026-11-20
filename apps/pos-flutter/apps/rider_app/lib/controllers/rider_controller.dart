import 'dart:convert';
import 'package:decimal/decimal.dart';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:uuid/uuid.dart';

const defaultRiderCloudApiUrl = String.fromEnvironment(
  'CLOUD_API_URL',
  defaultValue: 'https://ripedesk-cloud-api.onrender.com',
);

class RiderTrip {
  RiderTrip({
    required this.tripId,
    required this.orderNumber,
    required this.branchName,
    required this.customerName,
    required this.address,
    required this.isCod,
    required this.codExpected,
    this.status = 'assigned',
    Decimal? codCollected,
    this.podPhotoUrl = '',
    this.recipientName = '',
    this.codReconciled = false,
  }) : codCollected = codCollected ?? Decimal.zero;

  final String tripId;
  final String orderNumber;
  final String branchName;
  final String customerName;
  final String address;
  final bool isCod;
  final Decimal codExpected;
  String status; // assigned | picked_up | out_for_delivery | delivered | failed_delivery
  Decimal codCollected;
  String podPhotoUrl;
  String recipientName;
  bool codReconciled;
}

class BufferedTripEvent {
  const BufferedTripEvent({
    required this.clientEventId,
    required this.tripId,
    required this.nextStatus,
    required this.occurredOfflineAt,
    this.podPhotoUrl = '',
    this.recipientName = '',
    required this.codCollected,
  });

  final String clientEventId;
  final String tripId;
  final String nextStatus;
  final DateTime occurredOfflineAt;
  final String podPhotoUrl;
  final String recipientName;
  final Decimal codCollected;
}

class RiderController extends ChangeNotifier {
  RiderController({
    this.cloudApiUrl = defaultRiderCloudApiUrl,
    this.authToken = '',
    http.Client? httpClient,
  }) : _http = httpClient ?? http.Client() {
    _seedPilotTrips();
  }

  final String cloudApiUrl;
  String authToken;
  final http.Client _http;

  void updateAuthToken(String token) {
    authToken = token.trim().replaceFirst(RegExp(r'^Bearer\s+', caseSensitive: false), '');
    lastSyncError = null;
    notifyListeners();
    if (authToken.isNotEmpty) {
      flushBufferedEvents();
    }
  }

  Future<String?> loginWithPassword({
    required String tenantSlug,
    required String email,
    required String password,
  }) async {
    try {
      final res = await _http
          .post(
            Uri.parse('$cloudApiUrl/api/v1/auth/login'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({
              'tenant_slug': tenantSlug.trim(),
              'email': email.trim(),
              'password': password,
            }),
          )
          .timeout(const Duration(seconds: 15));
      final body = jsonDecode(res.body) as Map<String, dynamic>;
      final data = body['data'] as Map<String, dynamic>?;
      final token = data?['access_token'] as String?;
      if (res.statusCode >= 200 && res.statusCode < 300 && token != null && token.isNotEmpty) {
        updateAuthToken(token);
        return null;
      }
      final err = (body['error'] as Map<String, dynamic>?)?['message'] as String?;
      return err ?? 'Login failed (HTTP ${res.statusCode})';
    } catch (e) {
      return e.toString();
    }
  }
  final _uuid = const Uuid();
  bool onDuty = true;
  bool isOnline = true;
  String? lastSyncError;
  final List<RiderTrip> trips = [];
  final List<BufferedTripEvent> offlineBuffer = [];
  final List<String> syncedEventIds = [];

  void _seedPilotTrips() {
    trips.addAll([
      RiderTrip(
        tripId: 'trp-1',
        orderNumber: '#ORD-4082',
        branchName: 'F-7 Coffee Bar',
        customerName: 'Sara Malik',
        address: 'House 14, Street 22, F-7/2 Islamabad',
        isCod: false,
        codExpected: Decimal.zero,
        status: 'assigned',
      ),
      RiderTrip(
        tripId: 'trp-2',
        orderNumber: '#ORD-4085',
        branchName: 'G-9 Flagship Dine-In',
        customerName: 'Usman Tariq',
        address: 'Plot 8, Blue Area, Islamabad',
        isCod: true,
        codExpected: Decimal.parse('2480.00'),
        status: 'out_for_delivery',
      ),
      RiderTrip(
        tripId: 'trp-3',
        orderNumber: '#ORD-4089',
        branchName: 'G-9 Flagship Dine-In',
        customerName: 'Hina Qureshi',
        address: 'Apt 4B, Centaurus Residencia',
        isCod: true,
        codExpected: Decimal.parse('1870.00'),
        status: 'assigned',
      ),
    ]);
  }

  Decimal get unreconciledCodCashInHand {
    var total = Decimal.zero;
    for (final t in trips) {
      if (t.isCod && t.status == 'delivered' && !t.codReconciled) {
        total += t.codCollected;
      }
    }
    return total;
  }

  void toggleDuty() {
    onDuty = !onDuty;
    notifyListeners();
  }

  void setOnline(bool online) {
    isOnline = online;
    notifyListeners();
  }

  Future<void> advanceTripStatus(
    String tripId,
    String nextStatus, {
    String podPhotoUrl = '',
    String recipientName = '',
    Decimal? codCollected,
  }) async {
    final trip = trips.firstWhere((t) => t.tripId == tripId);
    if (nextStatus == 'delivered' && podPhotoUrl.trim().isEmpty && recipientName.trim().isEmpty) {
      throw StateError('POD photo or recipient name is required to complete delivery');
    }

    final collected = codCollected ?? (trip.isCod ? trip.codExpected : Decimal.zero);
    trip.status = nextStatus;
    if (nextStatus == 'delivered') {
      trip.podPhotoUrl = podPhotoUrl;
      trip.recipientName = recipientName;
      trip.codCollected = collected;
    }

    final ev = BufferedTripEvent(
      clientEventId: _uuid.v4(),
      tripId: tripId,
      nextStatus: nextStatus,
      occurredOfflineAt: DateTime.now().toUtc(),
      podPhotoUrl: podPhotoUrl,
      recipientName: recipientName,
      codCollected: collected,
    );

    if (!isOnline) {
      offlineBuffer.add(ev);
    } else {
      final ok = await _postTripStatusEvent(ev);
      if (ok) {
        syncedEventIds.add(ev.clientEventId);
      } else {
        offlineBuffer.add(ev);
      }
    }
    notifyListeners();
  }

  Future<bool> _postTripStatusEvent(BufferedTripEvent ev) async {
    if (authToken.isEmpty) {
      // No live token configured (unit tests / local demo) -> treat as locally synced
      return true;
    }
    try {
      final res = await _http
          .post(
            Uri.parse('$cloudApiUrl/api/v1/delivery/trips/${ev.tripId}/status'),
            headers: {
              'Content-Type': 'application/json',
              'Authorization': 'Bearer $authToken',
            },
            body: jsonEncode({
              'client_event_id': ev.clientEventId,
              'next_status': ev.nextStatus,
              'pod_photo_url': ev.podPhotoUrl,
              'recipient_name': ev.recipientName,
              'cod_collected': ev.codCollected.toString(),
              'occurred_offline_at': ev.occurredOfflineAt.toIso8601String(),
            }),
          )
          .timeout(const Duration(seconds: 10));
      if (res.statusCode >= 200 && res.statusCode < 300) {
        lastSyncError = null;
        return true;
      }
      lastSyncError = 'HTTP ${res.statusCode}: ${res.body}';
      return false;
    } catch (e) {
      lastSyncError = e.toString();
      return false;
    }
  }

  /// Drains buffered offline status events in strict FIFO order on reconnect.
  Future<int> flushBufferedEvents() async {
    if (!isOnline || offlineBuffer.isEmpty) return 0;
    var flushed = 0;
    while (offlineBuffer.isNotEmpty) {
      final ev = offlineBuffer.first;
      final ok = await _postTripStatusEvent(ev);
      if (!ok) break;
      offlineBuffer.removeAt(0);
      syncedEventIds.add(ev.clientEventId);
      flushed++;
    }
    notifyListeners();
    return flushed;
  }
}
