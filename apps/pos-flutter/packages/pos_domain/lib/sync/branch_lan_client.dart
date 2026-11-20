import 'dart:async';
import 'dart:convert';
import 'package:http/http.dart' as http;
import '../models/order_models.dart';

enum ConnectionMode {
  cloudConnected,
  lanOfflineInternet,
  isolatedTerminal,
}

class LanHealthStatus {
  const LanHealthStatus({
    required this.branchId,
    required this.cloudReachable,
    required this.pendingOutboxCount,
    required this.latestLanSeq,
    this.oldestPendingAt,
  });

  final String branchId;
  final bool cloudReachable;
  final int pendingOutboxCount;
  final int latestLanSeq;
  final String? oldestPendingAt;

  factory LanHealthStatus.fromJson(Map<String, dynamic> json) => LanHealthStatus(
        branchId: json['branch_id'] as String? ?? '',
        cloudReachable: json['cloud_reachable'] as bool? ?? false,
        pendingOutboxCount: json['pending_outbox_count'] as int? ?? 0,
        latestLanSeq: json['latest_lan_seq'] as int? ?? 0,
        oldestPendingAt: json['oldest_pending_at'] as String?,
      );
}

class LanStreamEvent {
  const LanStreamEvent({
    required this.seqId,
    required this.eventType,
    required this.aggregateId,
    required this.payloadJson,
    required this.createdAt,
  });

  final int seqId;
  final String eventType;
  final String aggregateId;
  final String payloadJson;
  final String createdAt;

  factory LanStreamEvent.fromJson(Map<String, dynamic> json) => LanStreamEvent(
        seqId: json['seq_id'] as int? ?? 0,
        eventType: json['event_type'] as String? ?? '',
        aggregateId: json['aggregate_id'] as String? ?? '',
        payloadJson: json['payload_json'] as String? ?? '{}',
        createdAt: json['created_at'] as String? ?? '',
      );
}

/// Coordinates Flutter POS / Staff terminals with the local Branch Service over LAN,
/// including SSE live updates, cursor-based catch-up, and Isolated Terminal guards.
class BranchLanClient {
  BranchLanClient({
    required this.branchBaseUrl,
    this.isolatedModeEnabled = false,
    http.Client? httpClient,
  })  : _injectedClient = httpClient,
        _client = httpClient ?? http.Client();

  final String branchBaseUrl;
  final bool isolatedModeEnabled;
  final http.Client? _injectedClient;
  final http.Client _client;

  ConnectionMode _mode = ConnectionMode.lanOfflineInternet;
  int _lastSeenSeq = 0;

  final _modeController = StreamController<ConnectionMode>.broadcast();
  final _eventController = StreamController<LanStreamEvent>.broadcast();

  ConnectionMode get currentMode => _mode;
  int get lastSeenSeq => _lastSeenSeq;
  Stream<ConnectionMode> get modeStream => _modeController.stream;
  Stream<LanStreamEvent> get eventStream => _eventController.stream;

  /// Guard: Shared dining tables and cross-terminal split settlements are locked in Isolated Terminal mode.
  bool canMutateSharedTable() => _mode != ConnectionMode.isolatedTerminal;

  /// Guard: Independent counter Quick Sale cash orders are allowed in Isolated Terminal mode when enabled.
  bool canAcceptOrder(OrderMode orderMode, {required bool isCashOnly}) {
    if (_mode != ConnectionMode.isolatedTerminal) return true;
    return isolatedModeEnabled && orderMode == OrderMode.quickSale && isCashOnly;
  }

  void _setMode(ConnectionMode next) {
    if (_mode != next) {
      _mode = next;
      _modeController.add(next);
    }
  }

  Future<LanHealthStatus?> checkHealth() async {
    try {
      final resp = await _client
          .get(Uri.parse('$branchBaseUrl/lan/health'))
          .timeout(const Duration(seconds: 3));
      if (resp.statusCode != 200) {
        _setMode(ConnectionMode.isolatedTerminal);
        return null;
      }
      final status = LanHealthStatus.fromJson(
        jsonDecode(resp.body) as Map<String, dynamic>,
      );
      _setMode(status.cloudReachable
          ? ConnectionMode.cloudConnected
          : ConnectionMode.lanOfflineInternet);
      return status;
    } catch (_) {
      _setMode(ConnectionMode.isolatedTerminal);
      return null;
    }
  }

  Future<String> fetchCatalogJson() async {
    final resp = await _client
        .get(Uri.parse('$branchBaseUrl/lan/catalog'))
        .timeout(const Duration(seconds: 5));
    if (resp.statusCode != 200) {
      throw Exception('Failed to fetch LAN catalog: HTTP ${resp.statusCode}');
    }
    return resp.body;
  }

  Future<Map<String, dynamic>> postCommand(
    String path,
    Map<String, dynamic> payload,
  ) async {
    // Use a dedicated short-lived client with Connection: close on live runs
    // so Windows keep-alive / SSE socket pooling never stalls command POSTs.
    final cmdClient = _injectedClient ?? http.Client();
    try {
      final resp = await cmdClient
          .post(
            Uri.parse('$branchBaseUrl$path'),
            headers: {
              'Content-Type': 'application/json',
              'Connection': 'close',
            },
            body: jsonEncode(payload),
          )
          .timeout(const Duration(seconds: 5));
      final body = jsonDecode(resp.body) as Map<String, dynamic>;
      if (resp.statusCode >= 400) {
        throw Exception('LAN command $path failed (${resp.statusCode}): ${body['error'] ?? resp.body}');
      }
      return body;
    } finally {
      if (_injectedClient == null) {
        cmdClient.close();
      }
    }
  }

  Future<List<LanStreamEvent>> catchUpEvents({int? sinceSeq}) async {
    final cursor = sinceSeq ?? _lastSeenSeq;
    final resp = await _client
        .get(Uri.parse('$branchBaseUrl/lan/events?since_seq=$cursor'))
        .timeout(const Duration(seconds: 5));
    if (resp.statusCode != 200) return const [];
    final decoded = jsonDecode(resp.body) as Map<String, dynamic>;
    final rawList = decoded['events'] as List<dynamic>? ?? const [];
    final events = rawList
        .map((e) => LanStreamEvent.fromJson(e as Map<String, dynamic>))
        .toList();
    for (final ev in events) {
      if (ev.seqId > _lastSeenSeq) {
        _lastSeenSeq = ev.seqId;
      }
      _eventController.add(ev);
    }
    return events;
  }

  /// Opens an SSE connection to GET /lan/stream and emits parsed LanStreamEvents.
  Future<void> connectSseStream({Duration reconnectDelay = const Duration(seconds: 3)}) async {
    while (!_eventController.isClosed) {
      try {
        await catchUpEvents();
        final req = http.Request('GET', Uri.parse('$branchBaseUrl/lan/stream'));
        req.headers['Accept'] = 'text/event-stream';
        final streamed = await _client.send(req);
        if (streamed.statusCode == 200) {
          await for (final line in streamed.stream
              .transform(utf8.decoder)
              .transform(const LineSplitter())) {
            if (line.startsWith('data: ')) {
              final raw = line.substring(6).trim();
              if (raw.startsWith('{') && raw.contains('event_type')) {
                final ev = LanStreamEvent.fromJson(
                  jsonDecode(raw) as Map<String, dynamic>,
                );
                if (ev.seqId > _lastSeenSeq) {
                  _lastSeenSeq = ev.seqId;
                }
                _eventController.add(ev);
              }
            }
          }
        }
      } catch (_) {
        _setMode(ConnectionMode.isolatedTerminal);
      }
      if (!_eventController.isClosed) {
        await Future<void>.delayed(reconnectDelay);
      }
    }
  }

  Future<void> dispose() async {
    await _modeController.close();
    await _eventController.close();
    _client.close();
  }
}
