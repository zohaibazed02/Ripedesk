import 'package:decimal/decimal.dart';

enum OrderMode {
  quickSale('quick_sale'),
  takeaway('takeaway'),
  dineIn('dine_in'),
  delivery('delivery'),
  qrOrder('qr_order'),
  driveThru('drive_thru');

  const OrderMode(this.wireValue);
  final String wireValue;

  static OrderMode fromWire(String? raw) {
    final normalized = raw?.toLowerCase();
    return OrderMode.values.firstWhere(
      (m) => m.wireValue == normalized,
      orElse: () => OrderMode.quickSale,
    );
  }
}

enum SyncState {
  onlineSynced('ONLINE_SYNCED'),
  lanMode('LAN_MODE'),
  isolatedTerminal('ISOLATED_TERMINAL');

  const SyncState(this.wireValue);
  final String wireValue;
}

class OrderItemModifier {
  const OrderItemModifier({
    required this.modifierId,
    required this.modifierName,
    required this.priceDelta,
    required this.quantity,
  });

  final String modifierId;
  final String modifierName;
  final Decimal priceDelta;
  final Decimal quantity;

  Map<String, dynamic> toJson() => {
        'modifier_id': modifierId,
        'modifier_name': modifierName,
        'price_delta': priceDelta.toString(),
        'quantity': quantity.toString(),
      };

  factory OrderItemModifier.fromJson(Map<String, dynamic> json) =>
      OrderItemModifier(
        modifierId: json['modifier_id'] as String,
        modifierName: json['modifier_name'] as String,
        priceDelta: Decimal.parse((json['price_delta'] ?? '0').toString()),
        quantity: Decimal.parse((json['quantity'] ?? '1').toString()),
      );
}

class OrderLineItem {
  const OrderLineItem({
    required this.id,
    required this.menuItemId,
    required this.itemName,
    required this.prepStation,
    required this.quantity,
    required this.unitPrice,
    required this.taxRatePercent,
    this.lineDiscount = const _ZeroDecimal(),
    this.seatNumber,
    this.checkNumber = 1,
    this.modifiers = const [],
    this.notes = '',
  });

  final String id;
  final String menuItemId;
  final String itemName;
  final String prepStation;
  final Decimal quantity;
  final Decimal unitPrice;
  final Decimal taxRatePercent;
  final Object lineDiscount;
  final int? seatNumber;
  final int checkNumber;
  final List<OrderItemModifier> modifiers;
  final String notes;

  Decimal get discountAmount =>
      lineDiscount is Decimal ? lineDiscount as Decimal : Decimal.zero;

  OrderLineItem copyWith({
    Decimal? quantity,
    int? seatNumber,
    int? checkNumber,
    List<OrderItemModifier>? modifiers,
    String? notes,
    Decimal? discount,
  }) =>
      OrderLineItem(
        id: id,
        menuItemId: menuItemId,
        itemName: itemName,
        prepStation: prepStation,
        quantity: quantity ?? this.quantity,
        unitPrice: unitPrice,
        taxRatePercent: taxRatePercent,
        lineDiscount: discount ?? discountAmount,
        seatNumber: seatNumber ?? this.seatNumber,
        checkNumber: checkNumber ?? this.checkNumber,
        modifiers: modifiers ?? this.modifiers,
        notes: notes ?? this.notes,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'menu_item_id': menuItemId,
        'item_name': itemName,
        'prep_station': prepStation,
        'quantity': quantity.toString(),
        'unit_price': unitPrice.toString(),
        'tax_rate_percent': taxRatePercent.toString(),
        'line_discount': discountAmount.toString(),
        'seat_number': seatNumber,
        'check_number': checkNumber,
        'modifiers': modifiers.map((m) => m.toJson()).toList(),
        'notes': notes,
      };
}

class _ZeroDecimal {
  const _ZeroDecimal();
}

class OrderRound {
  const OrderRound({
    required this.roundNumber,
    required this.items,
    required this.createdAt,
  });

  final int roundNumber;
  final List<OrderLineItem> items;
  final DateTime createdAt;

  Map<String, dynamic> toJson() => {
        'round_number': roundNumber,
        'items': items.map((i) => i.toJson()).toList(),
        'created_at': createdAt.toUtc().toIso8601String(),
      };
}

class OrderPayment {
  const OrderPayment({
    required this.id,
    required this.checkNumber,
    required this.method,
    required this.status,
    required this.amount,
    required this.tenderedAmount,
    required this.changeDue,
    this.providerRef = '',
  });

  final String id;
  final int checkNumber;
  final String method; // cash | card_terminal | online_gateway | cod
  final String status; // pending | authorized | captured | ambiguous | reversed | refunded
  final Decimal amount;
  final Decimal tenderedAmount;
  final Decimal changeDue;
  final String providerRef;

  Map<String, dynamic> toJson() => {
        'id': id,
        'check_number': checkNumber,
        'method': method,
        'status': status,
        'amount': amount.toString(),
        'tendered_amount': tenderedAmount.toString(),
        'change_due': changeDue.toString(),
        'provider_ref': providerRef,
      };
}

class PosOrder {
  const PosOrder({
    required this.id,
    required this.orderNumber,
    required this.orderMode,
    required this.orderVersion,
    required this.status,
    required this.rounds,
    required this.payments,
    required this.syncState,
    this.tableId,
  });

  final String id;
  final String orderNumber;
  final OrderMode orderMode;
  final String? tableId;
  final int orderVersion;
  final String status;
  final List<OrderRound> rounds;
  final List<OrderPayment> payments;
  final SyncState syncState;

  List<OrderLineItem> get allItems =>
      rounds.expand((r) => r.items).toList(growable: false);

  PosOrder copyWith({
    int? orderVersion,
    String? status,
    List<OrderRound>? rounds,
    List<OrderPayment>? payments,
    SyncState? syncState,
  }) =>
      PosOrder(
        id: id,
        orderNumber: orderNumber,
        orderMode: orderMode,
        tableId: tableId,
        orderVersion: orderVersion ?? this.orderVersion,
        status: status ?? this.status,
        rounds: rounds ?? this.rounds,
        payments: payments ?? this.payments,
        syncState: syncState ?? this.syncState,
      );
}
