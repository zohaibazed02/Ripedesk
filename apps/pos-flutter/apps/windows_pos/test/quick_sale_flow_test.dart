import 'package:decimal/decimal.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:pos_domain/pos_domain.dart';
import 'package:windows_pos/controllers/pos_controller.dart';
import 'package:windows_pos/data/pilot_catalog.dart';

class _SpyPrinterAdapter implements PosPrinterAdapter {
  final List<PrintJobReceipt> printedReceipts = [];
  int drawerKicks = 0;

  @override
  Future<String> printReceipt(PrintJobReceipt receipt) async {
    printedReceipts.add(receipt);
    return 'spy-receipt-ok';
  }

  @override
  Future<String> printKitchenTicket(PrintJobKitchenTicket ticket) async => 'spy-kt-ok';

  @override
  Future<String> openCashDrawer({String? managerOverridePin, String? paymentId}) async {
    drawerKicks++;
    return 'spy-drawer-ok';
  }

  @override
  Future<PrinterHealth> getPrinterStatus() async => const PrinterHealth(
        online: true,
        paperOut: false,
        coverOpen: false,
        detail: 'spy',
      );
}

void main() {
  test('Pilot Quick-Sale vertical slice: Large Oat Latte + Extra Shot cash tender & drawer kick', () async {
    final spyPrinter = _SpyPrinterAdapter();
    final controller = PosController(printerAdapter: spyPrinter);

    final oatLatte = PilotCatalog.menuItems.firstWhere((i) => i.sku == 'COF-OAT-LG');
    final extraShot = PilotCatalog.modifierGroups
        .firstWhere((g) => g.id == 'mg-shots')
        .modifiers
        .firstWhere((m) => m.id == 'mod-extra-shot');

    controller.addItemToCart(
      oatLatte,
      quantity: Decimal.one,
      modifiers: [
        OrderItemModifier(
          modifierId: extraShot.id,
          modifierName: extraShot.name,
          priceDelta: extraShot.priceDelta,
          quantity: Decimal.one,
        ),
      ],
      notes: 'Extra Hot',
    );

    // 950.00 + 220.00 = 1,170.00 subtotal; 16% tax = 187.20; grand total = 1,357.20
    expect(controller.totals.subtotal, Decimal.parse('1170.00'));
    expect(controller.totals.taxTotal, Decimal.parse('187.20'));
    expect(controller.totals.grandTotal, Decimal.parse('1357.20'));

    final receipt = await controller.settlePayment(
      paymentMethod: 'cash',
      tenderedAmount: Decimal.parse('1500.00'),
    );

    expect(receipt.changeDue, Decimal.parse('142.80'));
    expect(receipt.drawerOpened, isTrue);
    expect(spyPrinter.printedReceipts.length, 1);
    expect(spyPrinter.drawerKicks, 1);
    expect(controller.cart, isEmpty);
  });
}
