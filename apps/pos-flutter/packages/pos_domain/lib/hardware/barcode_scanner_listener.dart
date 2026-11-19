import 'dart:async';

enum BarcodePayloadKind { sku, orderQr, staffBadge }

class BarcodeScanEvent {
  const BarcodeScanEvent({
    required this.rawValue,
    required this.kind,
    required this.scannedAt,
  });

  final String rawValue;
  final BarcodePayloadKind kind;
  final DateTime scannedAt;
}

/// Parses keyboard-wedge or serial/HID barcode & QR scanner input streams with debounce protection.
class BarcodeScannerListener {
  BarcodeScannerListener({
    this.debounceDuration = const Duration(milliseconds: 250),
  });

  final Duration debounceDuration;
  final _controller = StreamController<BarcodeScanEvent>.broadcast();

  String? _lastValue;
  DateTime? _lastAcceptedAt;

  Stream<BarcodeScanEvent> get stream => _controller.stream;

  void onRawScan(String input) {
    final trimmed = input.trim();
    if (trimmed.isEmpty) return;

    final now = DateTime.now().toUtc();
    if (_lastValue == trimmed &&
        _lastAcceptedAt != null &&
        now.difference(_lastAcceptedAt!) < debounceDuration) {
      return;
    }

    _lastValue = trimmed;
    _lastAcceptedAt = now;

    _controller.add(BarcodeScanEvent(
      rawValue: trimmed,
      kind: classify(trimmed),
      scannedAt: now,
    ));
  }

  static BarcodePayloadKind classify(String value) {
    if (value.startsWith('ORD:') || value.contains('/ord/')) {
      return BarcodePayloadKind.orderQr;
    }
    if (value.startsWith('STAFF:') || value.startsWith('BADGE:')) {
      return BarcodePayloadKind.staffBadge;
    }
    return BarcodePayloadKind.sku;
  }

  Future<void> dispose() async {
    await _controller.close();
  }
}
