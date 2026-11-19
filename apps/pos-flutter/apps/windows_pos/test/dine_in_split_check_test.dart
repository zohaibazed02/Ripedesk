import 'package:decimal/decimal.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pos_domain/pos_domain.dart';
import 'package:windows_pos/controllers/dine_in_controller.dart';
import 'package:windows_pos/controllers/pos_controller.dart';
import 'package:windows_pos/data/pilot_catalog.dart';

void main() {
  test('Dine-In multi-round service, split checks, uneven 3-way split, and isolated mode lock', () async {
    final dineIn = DineInController();
    final pos = PosController();

    final burrata = PilotCatalog.menuItems.firstWhere((i) => i.sku == 'CLD-BUR-01');
    final latte = PilotCatalog.menuItems.firstWhere((i) => i.sku == 'COF-OAT-LG');
    final pasta = PilotCatalog.menuItems.firstWhere((i) => i.sku == 'KIT-PAS-01');

    // 1. Open Table T1 & fire Round 1 (appetizer + drink)
    dineIn.openTable('tbl-t1', guestCount: 2, waiterName: 'Hamza');
    dineIn.addDraftItem('tbl-t1', burrata, seatNumber: 1, checkNumber: 1);
    dineIn.addDraftItem('tbl-t1', latte, seatNumber: 2, checkNumber: 2);
    await dineIn.fireDraftRound('tbl-t1');

    expect(dineIn.sessions['tbl-t1']!.order.orderVersion, 1);
    expect(dineIn.sessions['tbl-t1']!.order.rounds.length, 1);

    // Fire Round 2 (main course on Check 1)
    dineIn.addDraftItem('tbl-t1', pasta, seatNumber: 1, checkNumber: 1);
    await dineIn.fireDraftRound('tbl-t1');

    expect(dineIn.sessions['tbl-t1']!.order.orderVersion, 2);
    expect(dineIn.sessions['tbl-t1']!.order.rounds.length, 2);

    // 2. Verify uneven 3-way even split allocates penny remainder deterministically to Check 1
    final shares = DineInController.splitEvenly(Decimal.parse('100.00'), 3);
    expect(shares, [Decimal.parse('33.34'), Decimal.parse('33.33'), Decimal.parse('33.33')]);
    expect(shares.fold(Decimal.zero, (a, b) => a + b), Decimal.parse('100.00'));

    // 3. Settle Check 1 (Cash) -> table remains occupied; settle Check 2 (Card) -> table transitions to dirty
    final totals = dineIn.calculator.calculate(dineIn.sessions['tbl-t1']!.allItems);
    final check1Total = totals.checks[1]!.grandTotal;
    final check2Total = totals.checks[2]!.grandTotal;

    final closedAfterC1 = await dineIn.recordCheckPayment(
      'tbl-t1',
      checkNumber: 1,
      method: 'cash',
      amount: check1Total,
      tenderedAmount: check1Total,
    );
    expect(closedAfterC1, isFalse);
    expect(dineIn.findTable('tbl-t1')!.status, TableStatus.occupied);

    final closedAfterC2 = await dineIn.recordCheckPayment(
      'tbl-t1',
      checkNumber: 2,
      method: 'card_terminal',
      amount: check2Total,
      tenderedAmount: check2Total,
    );
    expect(closedAfterC2, isTrue);
    expect(dineIn.findTable('tbl-t1')!.status, TableStatus.dirty);

    // 4. Isolated Terminal mode blocks table mutations while allowing Quick Sale cash
    dineIn.setConnectionMode(ConnectionMode.isolatedTerminal);
    expect(() => dineIn.openTable('tbl-t2', guestCount: 2, waiterName: 'Hamza'), throwsStateError);

    final lanClient = BranchLanClient(branchBaseUrl: 'http://127.0.0.1:8090', isolatedModeEnabled: true);
    pos.setConnectionMode(ConnectionMode.isolatedTerminal);
    expect(lanClient.canAcceptOrder(OrderMode.quickSale, isCashOnly: true), isTrue);
  });
}
