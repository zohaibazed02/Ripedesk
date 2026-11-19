import 'dart:async';
import 'package:decimal/decimal.dart';
import 'package:flutter/foundation.dart' hide Category;
import 'package:http/http.dart' as http;
import 'package:pos_domain/pos_domain.dart';
import 'package:uuid/uuid.dart';
import '../data/pilot_catalog.dart';

class ReceiptConfirmation {
  const ReceiptConfirmation({
    required this.orderNumber,
    required this.grandTotal,
    required this.paymentMethod,
    required this.tendered,
    required this.changeDue,
    required this.drawerOpened,
  });

  final String orderNumber;
  final Decimal grandTotal;
  final String paymentMethod;
  final Decimal tendered;
  final Decimal changeDue;
  final bool drawerOpened;
}

class PosController extends ChangeNotifier {
  PosController({
    BranchConfig? branch,
    this.lanClient,
    this.printerAdapter,
    this.terminalAdapter,
  }) : branch = branch ?? PilotCatalog.defaultBranch {
    orderMode = this.branch.defaultOrderMode;
  }

  final BranchLanClient? lanClient;
  final PosPrinterAdapter? printerAdapter;
  final PaymentTerminalAdapter? terminalAdapter;
  final _uuid = const Uuid();

  BranchConfig branch;
  late OrderMode orderMode;
  ConnectionMode connectionMode = ConnectionMode.lanOfflineInternet;
  int pendingOutboxCount = 0;
  int lanLatencyMs = 2;
  bool isDarkMode = false;
  String activeCashierName = 'Ayesha Khan';
  bool shiftOpen = true;

  String selectedCategoryId = 'all';
  String searchQuery = '';
  String customerNameOrBuzzer = '';

  Decimal? orderDiscountPercent;
  Decimal? orderDiscountFixed;

  final List<Category> categories = List.of(PilotCatalog.categories);
  final List<MenuItem> menuItems = List.of(PilotCatalog.menuItems);
  final List<ModifierGroup> modifierGroups = List.of(PilotCatalog.modifierGroups);
  final List<OrderLineItem> cart = [];

  int _orderSeq = 1001;
  ReceiptConfirmation? lastReceipt;

  OrderCalculator get calculator => OrderCalculator(
        taxInclusive: branch.taxInclusive,
        scale: 2,
      );

  OrderTotals get totals => calculator.calculate(
        cart,
        orderDiscountPercent: orderDiscountPercent,
        orderDiscountFixed: orderDiscountFixed,
      );

  List<MenuItem> get filteredItems {
    return menuItems.where((item) {
      if (selectedCategoryId != 'all' && item.categoryId != selectedCategoryId) {
        return false;
      }
      if (searchQuery.trim().isNotEmpty) {
        final q = searchQuery.toLowerCase();
        return item.name.toLowerCase().contains(q) ||
            item.sku.toLowerCase().contains(q);
      }
      return true;
    }).toList(growable: false);
  }

  List<ModifierGroup> modifierGroupsForItem(MenuItem item) {
    return modifierGroups
        .where((g) => item.modifierGroupIds.contains(g.id))
        .toList(growable: false);
  }

  bool itemRequiresModifierDrawer(MenuItem item) {
    return modifierGroupsForItem(item).any((g) => g.required);
  }

  void setOrderMode(OrderMode mode) {
    orderMode = mode;
    notifyListeners();
  }

  void switchBranch(BranchConfig nextBranch) {
    branch = nextBranch;
    orderMode = nextBranch.defaultOrderMode;
    notifyListeners();
  }

  void setConnectionMode(ConnectionMode mode, {int? pending}) {
    connectionMode = mode;
    if (pending != null) pendingOutboxCount = pending;
    notifyListeners();
  }

  Timer? _healthTimer;

  void startHealthPolling({Duration interval = const Duration(seconds: 5)}) {
    _healthTimer?.cancel();
    checkLanHealth();
    _healthTimer = Timer.periodic(interval, (_) => checkLanHealth());
  }

  Future<void> checkLanHealth() async {
    final baseUrl = lanClient?.branchBaseUrl ?? 'http://127.0.0.1:8090';
    final sw = Stopwatch()..start();
    try {
      final res = await http
          .get(Uri.parse('$baseUrl/healthz'))
          .timeout(const Duration(seconds: 2));
      sw.stop();
      if (res.statusCode >= 200 && res.statusCode < 300) {
        lanLatencyMs = sw.elapsedMilliseconds;
        if (connectionMode != ConnectionMode.online) {
          connectionMode = ConnectionMode.online;
          notifyListeners();
        }
        return;
      }
    } catch (_) {
      // Unreachable — stay in LAN offline fallback mode
    }
    if (connectionMode != ConnectionMode.lanOfflineInternet) {
      connectionMode = ConnectionMode.lanOfflineInternet;
      notifyListeners();
    }
  }

  @override
  void dispose() {
    _healthTimer?.cancel();
    super.dispose();
  }

  void toggleTheme() {
    isDarkMode = !isDarkMode;
    notifyListeners();
  }

  void setCategory(String categoryId) {
    selectedCategoryId = categoryId;
    notifyListeners();
  }

  void setSearchQuery(String query) {
    searchQuery = query;
    notifyListeners();
  }

  void setCustomerNameOrBuzzer(String value) {
    customerNameOrBuzzer = value;
    notifyListeners();
  }

  void applyOrderDiscount({Decimal? percent, Decimal? fixed}) {
    orderDiscountPercent = percent;
    orderDiscountFixed = fixed;
    notifyListeners();
  }

  void addItemToCart(
    MenuItem item, {
    Decimal? quantity,
    List<OrderItemModifier> modifiers = const [],
    String notes = '',
  }) {
    if (!item.isAvailable) return;
    final qty = quantity ?? Decimal.one;
    cart.add(OrderLineItem(
      id: _uuid.v4(),
      menuItemId: item.id,
      itemName: item.name,
      prepStation: item.prepStation,
      quantity: qty,
      unitPrice: item.basePrice,
      taxRatePercent: item.taxRatePercent,
      modifiers: modifiers,
      notes: notes,
    ));
    notifyListeners();
  }

  bool handleBarcodeScan(String rawSku) {
    final normalized = rawSku.trim().toUpperCase();
    for (final item in menuItems) {
      if (item.sku.toUpperCase() == normalized && item.isAvailable) {
        addItemToCart(item);
        return true;
      }
    }
    return false;
  }

  void incrementLine(String lineId) {
    final idx = cart.indexWhere((l) => l.id == lineId);
    if (idx == -1) return;
    cart[idx] = cart[idx].copyWith(quantity: cart[idx].quantity + Decimal.one);
    notifyListeners();
  }

  void decrementLine(String lineId) {
    final idx = cart.indexWhere((l) => l.id == lineId);
    if (idx == -1) return;
    final nextQty = cart[idx].quantity - Decimal.one;
    if (nextQty <= Decimal.zero) {
      cart.removeAt(idx);
    } else {
      cart[idx] = cart[idx].copyWith(quantity: nextQty);
    }
    notifyListeners();
  }

  void removeLine(String lineId) {
    cart.removeWhere((l) => l.id == lineId);
    notifyListeners();
  }

  void clearCart() {
    cart.clear();
    customerNameOrBuzzer = '';
    orderDiscountPercent = null;
    orderDiscountFixed = null;
    notifyListeners();
  }

  Future<String> sendToBarista() async {
    final orderId = _uuid.v4();
    final orderNumber = '${branch.branchCode}-${_orderSeq++}';
    if (lanClient != null && connectionMode != ConnectionMode.isolatedTerminal) {
      await lanClient!.postCommand('/lan/commands/orders', {
        'order_id': orderId,
        'tenant_id': branch.tenantId,
        'branch_id': branch.branchId,
        'order_number': orderNumber,
        'order_mode': orderMode.wireValue,
        'expected_version': 0,
        'subtotal': totals.subtotal.toString(),
        'tax_total': totals.taxTotal.toString(),
        'discount_total': totals.discountTotal.toString(),
        'grand_total': totals.grandTotal.toString(),
        'idempotency_key': 'cmd-$orderId',
        'items': cart.map((i) => i.toJson()).toList(),
      });
    } else {
      pendingOutboxCount++;
    }
    notifyListeners();
    return orderNumber;
  }

  Future<ReceiptConfirmation> settlePayment({
    required String paymentMethod,
    required Decimal tenderedAmount,
  }) async {
    final currentTotals = totals;
    final grand = currentTotals.grandTotal;
    final change = tenderedAmount > grand ? tenderedAmount - grand : Decimal.zero;
    final orderId = _uuid.v4();
    final paymentId = _uuid.v4();
    final orderNumber = '${branch.branchCode}-${_orderSeq++}';
    final shouldOpenDrawer = paymentMethod == 'cash';

    if (lanClient != null && connectionMode != ConnectionMode.isolatedTerminal) {
      await lanClient!.postCommand('/lan/commands/orders', {
        'order_id': orderId,
        'tenant_id': branch.tenantId,
        'branch_id': branch.branchId,
        'order_number': orderNumber,
        'order_mode': orderMode.wireValue,
        'expected_version': 0,
        'subtotal': currentTotals.subtotal.toString(),
        'tax_total': currentTotals.taxTotal.toString(),
        'discount_total': currentTotals.discountTotal.toString(),
        'grand_total': grand.toString(),
        'idempotency_key': 'cmd-$orderId',
        'items': cart.map((i) => i.toJson()).toList(),
      });
      await lanClient!.postCommand('/lan/commands/payments', {
        'payment_id': paymentId,
        'order_id': orderId,
        'check_number': 1,
        'method': paymentMethod,
        'amount': grand.toString(),
        'tendered_amount': tenderedAmount.toString(),
        'change_due': change.toString(),
        'idempotency_key': 'pay-$paymentId',
      });
    } else {
      pendingOutboxCount++;
    }

    if (printerAdapter != null) {
      await printerAdapter!.printReceipt(PrintJobReceipt(
        orderId: orderId,
        orderNumber: orderNumber,
        orderMode: orderMode.wireValue,
        grandTotal: grand.toStringAsFixed(2),
        paymentMethod: paymentMethod,
        openDrawer: shouldOpenDrawer,
      ));
      if (shouldOpenDrawer) {
        await printerAdapter!.openCashDrawer(paymentId: paymentId);
      }
    }

    lastReceipt = ReceiptConfirmation(
      orderNumber: orderNumber,
      grandTotal: grand,
      paymentMethod: paymentMethod,
      tendered: tenderedAmount,
      changeDue: change,
      drawerOpened: shouldOpenDrawer,
    );
    clearCart();
    return lastReceipt!;
  }

  Future<void> kickDrawerWithManagerPin(String pin) async {
    if (printerAdapter != null) {
      await printerAdapter!.openCashDrawer(managerOverridePin: pin);
    }
  }
}
