import 'package:decimal/decimal.dart';
import 'package:flutter/foundation.dart';
import 'package:pos_domain/pos_domain.dart';
import 'package:uuid/uuid.dart';
import '../data/pilot_catalog.dart';

class TableSession {
  TableSession({
    required this.order,
    required this.guestCount,
    required this.waiterName,
    required this.seatedAt,
    List<OrderLineItem>? draftItems,
  }) : draftItems = draftItems ?? [];

  PosOrder order;
  int guestCount;
  String waiterName;
  DateTime seatedAt;
  List<OrderLineItem> draftItems;

  List<OrderLineItem> get allItems => [...order.allItems, ...draftItems];

  int get distinctCheckCount {
    final checks = allItems.map((i) => i.checkNumber).toSet();
    return checks.isEmpty ? 1 : checks.length;
  }
}

class DineInController extends ChangeNotifier {
  DineInController({
    this.lanClient,
    this.printerAdapter,
    this.taxInclusive = false,
  }) {
    for (final f in PilotCatalog.dineInFloors) {
      floors.add(DiningFloor(
        id: f.id,
        name: f.name,
        sortOrder: f.sortOrder,
        tables: List.of(f.tables),
      ));
    }
  }

  final BranchLanClient? lanClient;
  final PosPrinterAdapter? printerAdapter;
  final bool taxInclusive;
  final _uuid = const Uuid();

  final List<DiningFloor> floors = [];
  final Map<String, TableSession> sessions = {};

  ConnectionMode connectionMode = ConnectionMode.lanOfflineInternet;
  String selectedFloorId = 'fl-main';
  String? selectedTableId;
  String? conflictMessage;
  int _orderSeq = 2001;

  OrderCalculator get calculator => OrderCalculator(taxInclusive: taxInclusive, scale: 2);

  bool get isTableLocked => connectionMode == ConnectionMode.isolatedTerminal;

  DiningFloor get activeFloor =>
      floors.firstWhere((f) => f.id == selectedFloorId, orElse: () => floors.first);

  TableSession? get activeSession =>
      selectedTableId != null ? sessions[selectedTableId!] : null;

  void setConnectionMode(ConnectionMode mode) {
    connectionMode = mode;
    notifyListeners();
  }

  void selectFloor(String floorId) {
    selectedFloorId = floorId;
    notifyListeners();
  }

  void selectTable(String? tableId) {
    selectedTableId = tableId;
    conflictMessage = null;
    notifyListeners();
  }

  DiningTable? findTable(String tableId) {
    for (final f in floors) {
      for (final t in f.tables) {
        if (t.id == tableId) return t;
      }
    }
    return null;
  }

  void _replaceTable(DiningTable updated) {
    for (final f in floors) {
      final idx = f.tables.indexWhere((t) => t.id == updated.id);
      if (idx != -1) {
        f.tables[idx] = updated;
        return;
      }
    }
  }

  /// Exact N-way even split with deterministic penny allocation to Check 1 (zero penny loss).
  static List<Decimal> splitEvenly(Decimal total, int ways) {
    if (ways <= 1) return [total.round(scale: 2)];
    final totalCents = (total * Decimal.fromInt(100)).round(scale: 0).toBigInt().toInt();
    final baseCents = totalCents ~/ ways;
    final remainderCents = totalCents % ways;
    return List.generate(ways, (idx) {
      final cents = idx == 0 ? baseCents + remainderCents : baseCents;
      return (Decimal.fromInt(cents) / Decimal.fromInt(100)).toDecimal(scaleOnInfinitePrecision: 2);
    });
  }

  void openTable(String tableId, {required int guestCount, required String waiterName}) {
    if (isTableLocked) {
      throw StateError('Shared dining tables are locked in Isolated Terminal mode');
    }
    final table = findTable(tableId);
    if (table == null) return;

    final orderId = _uuid.v4();
    final order = PosOrder(
      id: orderId,
      orderNumber: 'G9-${_orderSeq++}',
      orderMode: OrderMode.dineIn,
      tableId: tableId,
      orderVersion: 0,
      status: 'open',
      rounds: const [],
      payments: const [],
      syncState: SyncState.lanMode,
    );

    sessions[tableId] = TableSession(
      order: order,
      guestCount: guestCount,
      waiterName: waiterName,
      seatedAt: DateTime.now().toUtc(),
    );
    _replaceTable(table.copyWith(
      status: TableStatus.occupied,
      activeOrderId: orderId,
      orderVersion: 0,
    ));
    selectedTableId = tableId;
    notifyListeners();
  }

  void addDraftItem(
    String tableId,
    MenuItem item, {
    int? seatNumber,
    int checkNumber = 1,
    Decimal? quantity,
    List<OrderItemModifier> modifiers = const [],
    String notes = '',
  }) {
    if (isTableLocked) {
      throw StateError('Shared dining tables are locked in Isolated Terminal mode');
    }
    final session = sessions[tableId];
    if (session == null) return;

    session.draftItems.add(OrderLineItem(
      id: _uuid.v4(),
      menuItemId: item.id,
      itemName: item.name,
      prepStation: item.prepStation,
      quantity: quantity ?? Decimal.one,
      unitPrice: item.basePrice,
      taxRatePercent: item.taxRatePercent,
      seatNumber: seatNumber,
      checkNumber: checkNumber,
      modifiers: modifiers,
      notes: notes,
    ));
    notifyListeners();
  }

  Future<void> fireDraftRound(String tableId) async {
    if (isTableLocked) {
      throw StateError('Shared dining tables are locked in Isolated Terminal mode');
    }
    final session = sessions[tableId];
    if (session == null || session.draftItems.isEmpty) return;

    final nextRoundNum = session.order.rounds.length + 1;
    final nextVersion = session.order.orderVersion + 1;
    final roundItems = List<OrderLineItem>.of(session.draftItems);

    if (lanClient != null) {
      try {
        await lanClient!.postCommand('/lan/commands/orders', {
          'order_id': session.order.id,
          'order_number': session.order.orderNumber,
          'order_mode': OrderMode.dineIn.wireValue,
          'table_id': tableId,
          'expected_version': session.order.orderVersion,
          'grand_total': calculator.calculate(session.allItems).grandTotal.toString(),
          'items': roundItems.map((i) => i.toJson()).toList(),
        });
      } catch (e) {
        if (e.toString().contains('409')) {
          conflictMessage =
              'Another terminal updated Table ${findTable(tableId)?.tableCode ?? tableId}. Refresh & merge before firing Round #$nextRoundNum.';
          notifyListeners();
          return;
        }
        rethrow;
      }
    }

    final newRound = OrderRound(
      roundNumber: nextRoundNum,
      items: roundItems,
      createdAt: DateTime.now().toUtc(),
    );
    session.order = session.order.copyWith(
      orderVersion: nextVersion,
      status: 'in_prep',
      rounds: [...session.order.rounds, newRound],
    );
    session.draftItems.clear();

    final table = findTable(tableId);
    if (table != null) {
      _replaceTable(table.copyWith(orderVersion: nextVersion));
    }

    if (printerAdapter != null) {
      final stations = roundItems.map((i) => i.prepStation).toSet();
      for (final station in stations) {
        await printerAdapter!.printKitchenTicket(PrintJobKitchenTicket(
          orderId: session.order.id,
          orderNumber: session.order.orderNumber,
          station: station,
          roundNumber: nextRoundNum,
          items: roundItems
              .where((i) => i.prepStation == station)
              .map((i) => '${i.quantity}x ${i.itemName}')
              .toList(),
        ));
      }
    }

    conflictMessage = null;
    notifyListeners();
  }

  void assignItemCheck(String tableId, String lineId, int checkNumber) {
    final session = sessions[tableId];
    if (session == null || checkNumber < 1) return;

    final updatedRounds = session.order.rounds.map((r) {
      final items = r.items
          .map((i) => i.id == lineId ? i.copyWith(checkNumber: checkNumber) : i)
          .toList();
      return OrderRound(roundNumber: r.roundNumber, items: items, createdAt: r.createdAt);
    }).toList();
    session.order = session.order.copyWith(rounds: updatedRounds);

    for (var i = 0; i < session.draftItems.length; i++) {
      if (session.draftItems[i].id == lineId) {
        session.draftItems[i] = session.draftItems[i].copyWith(checkNumber: checkNumber);
      }
    }
    notifyListeners();
  }

  void transferTable(String fromTableId, String toTableId) {
    if (isTableLocked) return;
    final fromTable = findTable(fromTableId);
    final toTable = findTable(toTableId);
    final session = sessions.remove(fromTableId);
    if (fromTable == null || toTable == null || session == null) return;
    if (toTable.status != TableStatus.available) return;

    sessions[toTableId] = session;
    _replaceTable(fromTable.copyWith(status: TableStatus.available, activeOrderId: null, orderVersion: 0));
    _replaceTable(toTable.copyWith(
      status: TableStatus.occupied,
      activeOrderId: session.order.id,
      orderVersion: session.order.orderVersion,
    ));
    if (selectedTableId == fromTableId) {
      selectedTableId = toTableId;
    }
    notifyListeners();
  }

  void markTableClean(String tableId) {
    if (isTableLocked) return;
    final table = findTable(tableId);
    if (table == null) return;
    _replaceTable(table.copyWith(status: TableStatus.available, activeOrderId: null, orderVersion: 0));
    notifyListeners();
  }

  Decimal paidForCheck(String tableId, int checkNumber) {
    final session = sessions[tableId];
    if (session == null) return Decimal.zero;
    return session.order.payments
        .where((p) => p.checkNumber == checkNumber && p.status == 'captured')
        .fold<Decimal>(Decimal.zero, (acc, p) => acc + p.amount);
  }

  Decimal totalPaid(String tableId) {
    final session = sessions[tableId];
    if (session == null) return Decimal.zero;
    return session.order.payments
        .where((p) => p.status == 'captured')
        .fold<Decimal>(Decimal.zero, (acc, p) => acc + p.amount);
  }

  /// Settles a single check (or partial split tender). Closes order & marks table dirty ONLY when full grandTotal is paid.
  Future<bool> recordCheckPayment(
    String tableId, {
    required int checkNumber,
    required String method,
    required Decimal amount,
    required Decimal tenderedAmount,
  }) async {
    if (isTableLocked) {
      throw StateError('Shared table settlement is locked in Isolated Terminal mode');
    }
    final session = sessions[tableId];
    if (session == null) return false;

    final change = tenderedAmount > amount ? tenderedAmount - amount : Decimal.zero;
    final payment = OrderPayment(
      id: _uuid.v4(),
      checkNumber: checkNumber,
      method: method,
      status: 'captured',
      amount: amount,
      tenderedAmount: tenderedAmount,
      changeDue: change,
    );

    session.order = session.order.copyWith(
      payments: [...session.order.payments, payment],
    );

    if (printerAdapter != null) {
      await printerAdapter!.printReceipt(PrintJobReceipt(
        orderId: session.order.id,
        orderNumber: '${session.order.orderNumber}-C$checkNumber',
        orderMode: OrderMode.dineIn.wireValue,
        grandTotal: amount.toStringAsFixed(2),
        paymentMethod: method,
        openDrawer: method == 'cash',
      ));
    }

    final orderGrandTotal = calculator.calculate(session.allItems).grandTotal;
    final settled = totalPaid(tableId) >= orderGrandTotal;
    if (settled) {
      session.order = session.order.copyWith(status: 'completed');
      final table = findTable(tableId);
      if (table != null) {
        _replaceTable(table.copyWith(status: TableStatus.dirty, activeOrderId: null));
      }
      sessions.remove(tableId);
      if (selectedTableId == tableId) {
        selectedTableId = null;
      }
    }

    notifyListeners();
    return settled;
  }
}
