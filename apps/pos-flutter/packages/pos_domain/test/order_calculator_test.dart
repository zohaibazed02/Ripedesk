import 'package:decimal/decimal.dart';
import 'package:pos_domain/pos_domain.dart';
import 'package:test/test.dart';

void main() {
  test('OrderCalculator computes exact Decimal totals with modifiers and split checks', () {
    const calc = OrderCalculator(taxInclusive: false, scale: 2);
    final items = [
      OrderLineItem(
        id: 'line-1',
        menuItemId: 'item-latte',
        itemName: 'Large Oat Latte',
        prepStation: 'barista',
        quantity: Decimal.fromInt(1),
        unitPrice: Decimal.parse('950.00'),
        taxRatePercent: Decimal.parse('16'),
        checkNumber: 1,
        modifiers: [
          OrderItemModifier(
            modifierId: 'mod-shot',
            modifierName: 'Extra Shot',
            priceDelta: Decimal.parse('220.00'),
            quantity: Decimal.fromInt(1),
          ),
        ],
      ),
      OrderLineItem(
        id: 'line-2',
        menuItemId: 'item-croissant',
        itemName: 'Butter Croissant',
        prepStation: 'pastry',
        quantity: Decimal.fromInt(2),
        unitPrice: Decimal.parse('450.00'),
        taxRatePercent: Decimal.parse('16'),
        checkNumber: 2,
      ),
    ];

    final totals = calc.calculate(items);
    // Check 1: 950 + 220 = 1170.00, tax 16% = 187.20, grand = 1357.20
    expect(totals.checks[1]!.subtotal, Decimal.parse('1170.00'));
    expect(totals.checks[1]!.taxTotal, Decimal.parse('187.20'));
    expect(totals.checks[1]!.grandTotal, Decimal.parse('1357.20'));

    // Check 2: 2 * 450 = 900.00, tax 16% = 144.00, grand = 1044.00
    expect(totals.checks[2]!.subtotal, Decimal.parse('900.00'));
    expect(totals.checks[2]!.grandTotal, Decimal.parse('1044.00'));

    // Combined order total
    expect(totals.grandTotal, Decimal.parse('2401.20'));
  });
}
