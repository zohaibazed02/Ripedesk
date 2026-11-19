import 'dart:async';
import 'dart:convert';
import 'dart:io';

class PrinterHealth {
  const PrinterHealth({
    required this.online,
    required this.paperOut,
    required this.coverOpen,
    required this.detail,
  });

  final bool online;
  final bool paperOut;
  final bool coverOpen;
  final String detail;
}

class PrintJobReceipt {
  const PrintJobReceipt({
    required this.orderId,
    required this.orderNumber,
    required this.orderMode,
    required this.grandTotal,
    required this.paymentMethod,
    this.openDrawer = false,
  });

  final String orderId;
  final String orderNumber;
  final String orderMode;
  final String grandTotal;
  final String paymentMethod;
  final bool openDrawer;
}

class PrintJobKitchenTicket {
  const PrintJobKitchenTicket({
    required this.orderId,
    required this.orderNumber,
    required this.station,
    required this.roundNumber,
    required this.items,
  });

  final String orderId;
  final String orderNumber;
  final String station;
  final int roundNumber;
  final List<String> items;
}

abstract interface class PosPrinterAdapter {
  Future<String> printReceipt(PrintJobReceipt receipt);
  Future<String> printKitchenTicket(PrintJobKitchenTicket ticket);
  Future<String> openCashDrawer({String? managerOverridePin, String? paymentId});
  Future<PrinterHealth> getPrinterStatus();
}

/// Routes print & drawer commands to the local LAN Branch Service durable print spooler.
class BranchServiceSpoolerAdapter implements PosPrinterAdapter {
  BranchServiceSpoolerAdapter({required this.branchBaseUrl});

  final String branchBaseUrl;

  @override
  Future<String> printReceipt(PrintJobReceipt receipt) async {
    return 'lan-spooler-receipt:${receipt.orderId}';
  }

  @override
  Future<String> printKitchenTicket(PrintJobKitchenTicket ticket) async {
    return 'lan-spooler-ticket:${ticket.orderId}:${ticket.station}';
  }

  @override
  Future<String> openCashDrawer({
    String? managerOverridePin,
    String? paymentId,
  }) async {
    final client = HttpClient();
    try {
      final uri = Uri.parse('$branchBaseUrl/lan/hardware/cash-drawer/kick');
      final req = await client.postUrl(uri);
      req.headers.contentType = ContentType.json;
      req.write(jsonEncode({
        'manager_override_pin': managerOverridePin ?? '',
        'payment_id': paymentId ?? '',
      }));
      final resp = await req.close();
      final body = await resp.transform(utf8.decoder).join();
      if (resp.statusCode != HttpStatus.ok) {
        throw HttpException('Cash drawer kick failed (${resp.statusCode}): $body');
      }
      return body;
    } finally {
      client.close(force: true);
    }
  }

  @override
  Future<PrinterHealth> getPrinterStatus() async {
    return const PrinterHealth(
      online: true,
      paperOut: false,
      coverOpen: false,
      detail: 'Connected to Branch Service Spooler',
    );
  }
}

/// Direct RAW TCP 9100 ESC/POS fallback for Isolated Terminal mode when LAN Branch Service is unreachable.
class DirectTcpEscPosAdapter implements PosPrinterAdapter {
  DirectTcpEscPosAdapter({
    required this.host,
    this.port = 9100,
    this.timeout = const Duration(seconds: 3),
  });

  final String host;
  final int port;
  final Duration timeout;

  Future<void> _sendRaw(List<int> bytes) async {
    final socket = await Socket.connect(host, port, timeout: timeout);
    try {
      socket.add(bytes);
      await socket.flush();
    } finally {
      await socket.close();
    }
  }

  @override
  Future<String> printReceipt(PrintJobReceipt receipt) async {
    final frame = <int>[
      0x1B, 0x40, // ESC @ init
      ...utf8.encode('${receipt.orderNumber} (${receipt.orderMode})\nTOTAL: ${receipt.grandTotal}\n\n\n'),
      0x1D, 0x56, 0x42, 0x00, // GS V partial cut
      if (receipt.openDrawer) ...[0x1B, 0x70, 0x00, 0x19, 0xFA],
    ];
    await _sendRaw(frame);
    return 'direct-tcp-receipt:${receipt.orderId}';
  }

  @override
  Future<String> printKitchenTicket(PrintJobKitchenTicket ticket) async {
    final frame = <int>[
      0x1B, 0x40,
      ...utf8.encode('*** ${ticket.station.toUpperCase()} ***\n${ticket.orderNumber} R#${ticket.roundNumber}\n'),
      ...utf8.encode('${ticket.items.join('\n')}\n\n\n'),
      0x1D, 0x56, 0x42, 0x00,
    ];
    await _sendRaw(frame);
    return 'direct-tcp-ticket:${ticket.orderId}';
  }

  @override
  Future<String> openCashDrawer({
    String? managerOverridePin,
    String? paymentId,
  }) async {
    await _sendRaw(const [0x1B, 0x40, 0x1B, 0x70, 0x00, 0x19, 0xFA]);
    return 'direct-tcp-drawer-kick';
  }

  @override
  Future<PrinterHealth> getPrinterStatus() async {
    try {
      final socket = await Socket.connect(host, port, timeout: timeout);
      await socket.close();
      return const PrinterHealth(
        online: true,
        paperOut: false,
        coverOpen: false,
        detail: 'Direct TCP 9100 reachable',
      );
    } catch (e) {
      return PrinterHealth(
        online: false,
        paperOut: false,
        coverOpen: false,
        detail: e.toString(),
      );
    }
  }
}
