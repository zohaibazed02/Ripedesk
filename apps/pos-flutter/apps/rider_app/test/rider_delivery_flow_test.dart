import 'package:decimal/decimal.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:rider_app/controllers/rider_controller.dart';

void main() {
  test('Rider offline buffering, FIFO flush on reconnect, POD completion & exact Decimal COD wallet', () async {
    final ctrl = RiderController();

    // Simulate cellular drop
    ctrl.setOnline(false);

    await ctrl.advanceTripStatus(
      'trp-2',
      'delivered',
      podPhotoUrl: 'pod://trp-2.jpg',
      recipientName: 'Usman Tariq',
      codCollected: Decimal.parse('2480.00'),
    );

    expect(ctrl.offlineBuffer.length, 1);
    expect(ctrl.unreconciledCodCashInHand, Decimal.parse('2480.00'));

    // Reconnect & flush FIFO queue
    ctrl.setOnline(true);
    final flushed = await ctrl.flushBufferedEvents();
    expect(flushed, 1);
    expect(ctrl.offlineBuffer, isEmpty);
    expect(ctrl.syncedEventIds.length, 1);
  });
}
