import 'dart:async';
import 'dart:convert';
import 'package:decimal/decimal.dart';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import 'package:pos_domain/pos_domain.dart';
import 'package:uuid/uuid.dart';

class ReadyToServeAlert {
  const ReadyToServeAlert({
    required this.id,
    required this.tableCode,
    required this.orderNumber,
    required this.station,
    required this.bumpedAt,
  });

  final String id;
  final String tableCode;
  final String orderNumber;
  final String station;
  final DateTime bumpedAt;
}

class StaffTableOrder {
  StaffTableOrder({
    required this.order,
    required this.waiterUserId,
    required this.guestCount,
    List<OrderLineItem>? draftItems,
  }) : draftItems = draftItems ?? [];

  PosOrder order;
  final String waiterUserId;
  int guestCount;
  bool billRequested = false;
  final List<OrderLineItem> draftItems;

  List<OrderLineItem> get allItems => [...order.allItems, ...draftItems];
}

class StaffAppController extends ChangeNotifier {
  StaffAppController({this.lanClient}) {
    _seedPilotData();
    _eventSub = lanClient?.eventStream.listen(_handleLanEvent);
  }

  BranchLanClient? lanClient;
  final _uuid = const Uuid();
  StreamSubscription<LanStreamEvent>? _eventSub;

  void updateLanUrl(String nextUrl) {
    final clean = nextUrl.trim().replaceAll(RegExp(r'/$'), '');
    if (clean.isEmpty) return;
    _eventSub?.cancel();
    lanClient = BranchLanClient(branchBaseUrl: clean);
    _eventSub = lanClient!.eventStream.listen(_handleLanEvent);
    checkLanHealth();
    notifyListeners();
  }

  final String branchName = 'G-9 Flagship Dine-In';
  final String currencyCode = 'PKR';

  StaffSession? activeSession;
  ConnectionMode connectionMode = ConnectionMode.lanOfflineInternet;
  int pendingOutboxCount = 0;
  bool isDarkMode = false;
  bool onlyMyTables = true;
  Timer? _healthTimer;

  void startHealthPolling({Duration interval = const Duration(seconds: 5)}) {
    _healthTimer?.cancel();
    checkLanHealth();
    _healthTimer = Timer.periodic(interval, (_) => checkLanHealth());
  }

  Future<void> checkLanHealth() async {
    final baseUrl = lanClient?.branchBaseUrl ?? 'http://192.168.1.10:8090';
    try {
      final res = await http
          .get(Uri.parse('$baseUrl/healthz'))
          .timeout(const Duration(seconds: 2));
      if (res.statusCode >= 200 && res.statusCode < 300) {
        final body = jsonDecode(res.body) as Map<String, dynamic>;
        final cloudReachable = body['cloud_reachable'] as bool? ?? false;
        pendingOutboxCount = body['pending_outbox_count'] as int? ?? 0;
        connectionMode = cloudReachable
            ? ConnectionMode.cloudConnected
            : ConnectionMode.lanOfflineInternet;
        notifyListeners();
        return;
      }
    } catch (_) {
      // Unreachable — stay in LAN offline mode
    }
    if (connectionMode != ConnectionMode.lanOfflineInternet) {
      connectionMode = ConnectionMode.lanOfflineInternet;
      notifyListeners();
    }
  }

  final List<DiningTable> tables = [];
  final List<MenuItem> menuItems = [];
  final Map<String, StaffTableOrder> tableOrders = {};
  final List<ReadyToServeAlert> readyAlerts = [];

  int _orderSeq = 3001;

  OrderCalculator get calculator => const OrderCalculator(taxInclusive: false, scale: 2);

  static final Map<String, ({String userId, String name, String role, bool canOverride})> _pilotPins = {
    '4444': (userId: 'usr-waiter-1', name: 'Hamza Raza', role: 'waiter', canOverride: false),
    '3333': (userId: 'usr-waiter-2', name: 'Ayesha Khan', role: 'waiter', canOverride: false),
    '2222': (userId: 'usr-mgr-1', name: 'Bilal Ahmed', role: 'branch_manager', canOverride: true),
  };

  void _seedPilotData() {
    tables.addAll(const [
      DiningTable(id: 'tbl-t1', floorId: 'fl-main', tableCode: 'T1', capacity: 2, status: TableStatus.available),
      DiningTable(id: 'tbl-t2', floorId: 'fl-main', tableCode: 'T2', capacity: 4, status: TableStatus.available),
      DiningTable(id: 'tbl-t3', floorId: 'fl-main', tableCode: 'T3', capacity: 4, status: TableStatus.reserved),
      DiningTable(id: 'tbl-t4', floorId: 'fl-main', tableCode: 'T4', capacity: 6, status: TableStatus.available),
      DiningTable(id: 'tbl-p1', floorId: 'fl-patio', tableCode: 'P1', capacity: 2, status: TableStatus.dirty),
      DiningTable(id: 'tbl-p2', floorId: 'fl-patio', tableCode: 'P2', capacity: 4, status: TableStatus.available),
    ]);

    menuItems.addAll([
      MenuItem(
        id: 'item-pasta',
        categoryId: 'cat-kitchen',
        sku: 'KIT-PAS-01',
        name: 'Truffle Mushroom Pasta',
        basePrice: Decimal.parse('1850.00'),
        taxRatePercent: Decimal.parse('16'),
        prepStation: 'hot_kitchen',
        isAvailable: true,
      ),
      MenuItem(
        id: 'item-pizza',
        categoryId: 'cat-kitchen',
        sku: 'KIT-PIZ-02',
        name: 'Wood-Fired Margherita',
        basePrice: Decimal.parse('1650.00'),
        taxRatePercent: Decimal.parse('16'),
        prepStation: 'hot_kitchen',
        isAvailable: true,
      ),
      MenuItem(
        id: 'item-burrata',
        categoryId: 'cat-cold',
        sku: 'CLD-BUR-01',
        name: 'Burrata Salad',
        basePrice: Decimal.parse('1400.00'),
        taxRatePercent: Decimal.parse('16'),
        prepStation: 'cold_prep',
        isAvailable: true,
      ),
      MenuItem(
        id: 'item-latte',
        categoryId: 'cat-coffee',
        sku: 'COF-OAT-LG',
        name: 'Large Oat Latte',
        basePrice: Decimal.parse('950.00'),
        taxRatePercent: Decimal.parse('16'),
        prepStation: 'barista',
        isAvailable: true,
      ),
    ]);
  }

  bool loginWithPin(String pin) {
    final staff = _pilotPins[pin];
    if (staff == null) return false;
    final now = DateTime.now().toUtc();
    activeSession = StaffSession(
      userId: staff.userId,
      name: staff.name,
      role: staff.role,
      canOverride: staff.canOverride,
      pinVerifiedAt: now,
      expiresAt: now.add(const Duration(hours: 8)),
    );
    notifyListeners();
    return true;
  }

  void lockSession() {
    activeSession = null;
    notifyListeners();
  }

  void toggleTheme() {
    isDarkMode = !isDarkMode;
    notifyListeners();
  }

  void setOnlyMyTables(bool value) {
    onlyMyTables = value;
    notifyListeners();
  }

  List<DiningTable> get visibleTables {
    if (!onlyMyTables || activeSession == null) return tables;
    return tables.where((t) {
      if (t.status == TableStatus.available) return true;
      final ord = tableOrders[t.id];
      return ord != null && ord.waiterUserId == activeSession!.userId;
    }).toList(growable: false);
  }

  void openTable(String tableId, {int guestCount = 2}) {
    if (activeSession == null) return;
    final idx = tables.indexWhere((t) => t.id == tableId);
    if (idx == -1) return;

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
    tableOrders[tableId] = StaffTableOrder(
      order: order,
      waiterUserId: activeSession!.userId,
      guestCount: guestCount,
    );
    tables[idx] = tables[idx].copyWith(
      status: TableStatus.occupied,
      activeOrderId: orderId,
      orderVersion: 0,
    );
    notifyListeners();
  }

  void addDraftItem(String tableId, MenuItem item, {int seatNumber = 1}) {
    final active = tableOrders[tableId];
    if (active == null) return;
    active.draftItems.add(OrderLineItem(
      id: _uuid.v4(),
      menuItemId: item.id,
      itemName: item.name,
      prepStation: item.prepStation,
      quantity: Decimal.one,
      unitPrice: item.basePrice,
      taxRatePercent: item.taxRatePercent,
      seatNumber: seatNumber,
      checkNumber: seatNumber,
    ));
    notifyListeners();
  }

  Future<void> fireRoundToKitchen(String tableId) async {
    final active = tableOrders[tableId];
    if (active == null || active.draftItems.isEmpty) return;

    final nextRound = active.order.rounds.length + 1;
    final nextVersion = active.order.orderVersion + 1;
    final items = List<OrderLineItem>.of(active.draftItems);

    final totals = calculator.calculate(active.allItems);
    if (lanClient != null) {
      await lanClient!.postCommand('/lan/commands/orders', {
        'order_id': active.order.id,
        'order_number': active.order.orderNumber,
        'order_mode': OrderMode.dineIn.wireValue,
        'table_id': tableId,
        'expected_version': active.order.orderVersion,
        'subtotal': totals.subtotal.toString(),
        'tax_total': totals.taxTotal.toString(),
        'discount_total': totals.discountTotal.toString(),
        'grand_total': totals.grandTotal.toString(),
        'idempotency_key': 'cmd-${active.order.id}-r$nextRound',
        'items': items
            .map((i) => {
                  'menu_item_id': i.menuItemId,
                  'item_name': i.itemName,
                  'prep_station': i.prepStation,
                  'quantity': i.quantity.toString(),
                  'unit_price': i.unitPrice.toString(),
                  'line_total': (i.unitPrice * i.quantity).toString(),
                  'check_number': i.checkNumber,
                })
            .toList(),
      });
    }

    active.order = active.order.copyWith(
      orderVersion: nextVersion,
      status: 'in_prep',
      rounds: [
        ...active.order.rounds,
        OrderRound(roundNumber: nextRound, items: items, createdAt: DateTime.now().toUtc()),
      ],
    );
    active.draftItems.clear();
    notifyListeners();
  }

  void requestBill(String tableId) {
    final active = tableOrders[tableId];
    if (active == null) return;
    active.billRequested = true;
    notifyListeners();
  }

  /// Handles live LAN SSE events including KDS `bumped` Ready-to-Serve alerts.
  void handleLanEvent(LanStreamEvent ev) => _handleLanEvent(ev);

  void _handleLanEvent(LanStreamEvent ev) {
    if (ev.eventType != 'kds.ticket_updated') return;
    final data = jsonDecode(ev.payloadJson) as Map<String, dynamic>;
    if (data['status'] != 'bumped') return;

    final orderId = data['order_id'] as String? ?? '';
    for (final entry in tableOrders.entries) {
      if (entry.value.order.id == orderId) {
        final table = tables.firstWhere(
          (t) => t.id == entry.key,
          orElse: () => const DiningTable(
            id: '',
            floorId: '',
            tableCode: 'TBL',
            capacity: 2,
            status: TableStatus.occupied,
          ),
        );
        readyAlerts.insert(
          0,
          ReadyToServeAlert(
            id: _uuid.v4(),
            tableCode: table.tableCode,
            orderNumber: entry.value.order.orderNumber,
            station: (data['station'] as String? ?? 'kitchen').toUpperCase(),
            bumpedAt: DateTime.now().toUtc(),
          ),
        );
        notifyListeners();
        break;
      }
    }
  }

  void acknowledgeServed(String alertId) {
    readyAlerts.removeWhere((a) => a.id == alertId);
    notifyListeners();
  }

  @override
  void dispose() {
    _healthTimer?.cancel();
    _eventSub?.cancel();
    super.dispose();
  }
}
