import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:pos_domain/pos_domain.dart';
import 'package:android_staff/controllers/staff_app_controller.dart';

void main() {
  test('Waiter PIN login, tableside multi-round firing, and KDS Ready-to-Serve alert flow', () async {
    final ctrl = StaffAppController();

    // 1. Bounded offline waiter PIN login
    expect(ctrl.loginWithPin('9999'), isFalse);
    expect(ctrl.loginWithPin('4444'), isTrue);
    expect(ctrl.activeSession!.role, 'waiter');
    expect(ctrl.activeSession!.canOverride, isFalse);

    // 2. Open Table T1, add Seat 1 item, and fire Round 1 to kitchen
    ctrl.openTable('tbl-t1', guestCount: 2);
    ctrl.addDraftItem('tbl-t1', ctrl.menuItems.first, seatNumber: 1);
    await ctrl.fireRoundToKitchen('tbl-t1');

    final active = ctrl.tableOrders['tbl-t1']!;
    expect(active.order.orderVersion, 1);
    expect(active.order.rounds.length, 1);

    // 3. Receive KDS bumped SSE event -> triggers Ready-to-Serve alert
    ctrl.handleLanEvent(LanStreamEvent(
      seqId: 10,
      eventType: 'kds.ticket_updated',
      aggregateId: 'tk-1',
      payloadJson: jsonEncode({
        'order_id': active.order.id,
        'status': 'bumped',
        'station': 'hot_kitchen',
      }),
      createdAt: DateTime.now().toUtc().toIso8601String(),
    ));

    expect(ctrl.readyAlerts.length, 1);
    expect(ctrl.readyAlerts.first.tableCode, 'T1');

    // 4. Acknowledge served
    ctrl.acknowledgeServed(ctrl.readyAlerts.first.id);
    expect(ctrl.readyAlerts, isEmpty);
  });
}
