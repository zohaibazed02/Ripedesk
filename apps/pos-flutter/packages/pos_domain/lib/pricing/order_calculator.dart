import 'package:decimal/decimal.dart';
import '../models/order_models.dart';

class CheckTotals {
  const CheckTotals({
    required this.checkNumber,
    required this.subtotal,
    required this.discountTotal,
    required this.taxTotal,
    required this.grandTotal,
  });

  final int checkNumber;
  final Decimal subtotal;
  final Decimal discountTotal;
  final Decimal taxTotal;
  final Decimal grandTotal;
}

class OrderTotals {
  const OrderTotals({
    required this.subtotal,
    required this.discountTotal,
    required this.taxTotal,
    required this.grandTotal,
    required this.checks,
  });

  final Decimal subtotal;
  final Decimal discountTotal;
  final Decimal taxTotal;
  final Decimal grandTotal;
  final Map<int, CheckTotals> checks;
}

/// Exact fixed-precision Decimal pricing & split-check calculator (zero floating-point math).
class OrderCalculator {
  const OrderCalculator({
    this.taxInclusive = false,
    this.scale = 2,
  });

  final bool taxInclusive;
  final int scale;

  static final Decimal _hundred = Decimal.fromInt(100);

  Decimal round(Decimal value) {
    return value.round(scale: scale);
  }

  Decimal lineGross(OrderLineItem item) {
    var modSum = Decimal.zero;
    for (final mod in item.modifiers) {
      modSum += mod.priceDelta * mod.quantity;
    }
    return (item.unitPrice + modSum) * item.quantity;
  }

  OrderTotals calculate(
    List<OrderLineItem> items, {
    Decimal? orderDiscountPercent,
    Decimal? orderDiscountFixed,
  }) {
    final grouped = <int, List<OrderLineItem>>{};
    for (final item in items) {
      grouped.putIfAbsent(item.checkNumber, () => []).add(item);
    }

    var totalSubtotal = Decimal.zero;
    var totalDiscount = Decimal.zero;
    var totalTax = Decimal.zero;
    var totalGrand = Decimal.zero;
    final checks = <int, CheckTotals>{};

    final orderGross = items.fold<Decimal>(Decimal.zero, (acc, it) => acc + lineGross(it));

    for (final entry in grouped.entries) {
      final checkNum = entry.key;
      final checkItems = entry.value;

      var checkSubtotal = Decimal.zero;
      var checkLineDiscount = Decimal.zero;
      var checkTax = Decimal.zero;

      for (final item in checkItems) {
        final gross = lineGross(item);
        final lineDisc = item.discountAmount > gross ? gross : item.discountAmount;
        var net = gross - lineDisc;

        // Pro-rate order-level discount across lines
        var orderDiscShare = Decimal.zero;
        if (orderDiscountPercent != null && orderDiscountPercent > Decimal.zero) {
          orderDiscShare += (net * orderDiscountPercent / _hundred).toDecimal(scaleOnInfinitePrecision: 6);
        }
        if (orderDiscountFixed != null && orderDiscountFixed > Decimal.zero && orderGross > Decimal.zero) {
          final ratio = (gross / orderGross).toDecimal(scaleOnInfinitePrecision: 8);
          orderDiscShare += orderDiscountFixed * ratio;
        }
        if (orderDiscShare > net) {
          orderDiscShare = net;
        }

        final taxable = net - orderDiscShare;
        Decimal itemTax = Decimal.zero;
        Decimal itemNetSubtotal = taxable;

        if (item.taxRatePercent > Decimal.zero) {
          if (taxInclusive) {
            final divisor = Decimal.one + (item.taxRatePercent / _hundred).toDecimal(scaleOnInfinitePrecision: 6);
            itemNetSubtotal = (taxable / divisor).toDecimal(scaleOnInfinitePrecision: 6);
            itemTax = taxable - itemNetSubtotal;
          } else {
            itemTax = (taxable * item.taxRatePercent / _hundred).toDecimal(scaleOnInfinitePrecision: 6);
          }
        }

        checkSubtotal += itemNetSubtotal;
        checkLineDiscount += lineDisc + orderDiscShare;
        checkTax += itemTax;
      }

      final rSub = round(checkSubtotal);
      final rDisc = round(checkLineDiscount);
      final rTax = round(checkTax);
      final rGrand = round(rSub + rTax);

      checks[checkNum] = CheckTotals(
        checkNumber: checkNum,
        subtotal: rSub,
        discountTotal: rDisc,
        taxTotal: rTax,
        grandTotal: rGrand,
      );

      totalSubtotal += rSub;
      totalDiscount += rDisc;
      totalTax += rTax;
      totalGrand += rGrand;
    }

    return OrderTotals(
      subtotal: round(totalSubtotal),
      discountTotal: round(totalDiscount),
      taxTotal: round(totalTax),
      grandTotal: round(totalGrand),
      checks: checks,
    );
  }
}
