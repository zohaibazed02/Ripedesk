import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:pos_domain/pos_domain.dart';
import '../controllers/pos_controller.dart';
import '../theme/pos_theme.dart';
import '../widgets/modifier_drawer.dart';
import '../widgets/pos_top_bar.dart';
import '../widgets/tender_modal.dart';

class QuickSaleScreen extends StatefulWidget {
  const QuickSaleScreen({super.key, required this.controller});

  final PosController controller;

  @override
  State<QuickSaleScreen> createState() => _QuickSaleScreenState();
}

class _QuickSaleScreenState extends State<QuickSaleScreen> {
  final _searchFocus = FocusNode();
  final _searchCtrl = TextEditingController();
  final _customerCtrl = TextEditingController();

  @override
  void dispose() {
    _searchFocus.dispose();
    _searchCtrl.dispose();
    _customerCtrl.dispose();
    super.dispose();
  }

  Future<void> _onTapProduct(MenuItem item) async {
    if (!item.isAvailable) return;
    final groups = widget.controller.modifierGroupsForItem(item);
    if (groups.isNotEmpty) {
      final result = await showDialog<ModifierSelectionResult>(
        context: context,
        builder: (_) => ModifierDrawer(
          item: item,
          groups: groups,
          currencyCode: widget.controller.branch.currencyCode,
        ),
      );
      if (result != null) {
        widget.controller.addItemToCart(
          item,
          quantity: result.quantity,
          modifiers: result.modifiers,
          notes: result.notes,
        );
      }
    } else {
      widget.controller.addItemToCart(item);
    }
  }

  Future<void> _openTenderModal() async {
    if (widget.controller.cart.isEmpty) return;
    final res = await showDialog<TenderResult>(
      context: context,
      builder: (_) => TenderModal(
        grandTotal: widget.controller.totals.grandTotal,
        currencyCode: widget.controller.branch.currencyCode,
        connectionMode: widget.controller.connectionMode,
      ),
    );
    if (res != null && mounted) {
      final conf = await widget.controller.settlePayment(
        paymentMethod: res.paymentMethod,
        tenderedAmount: res.tenderedAmount,
      );
      _customerCtrl.clear();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            backgroundColor: PosColors.emerald,
            content: Text(
              'Settled ${conf.orderNumber} · Change Due: ${widget.controller.branch.currencyCode} ${conf.changeDue.toStringAsFixed(2)}',
            ),
          ),
        );
      }
    }
  }

  Future<void> _sendToBarista() async {
    if (widget.controller.cart.isEmpty) return;
    final orderNo = await widget.controller.sendToBarista();
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Sent $orderNo to Barista / Kitchen KDS')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = widget.controller;
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final borderColor = isDark ? PosColors.darkBorder : PosColors.lightBorder;
    final totals = c.totals;
    final currency = c.branch.currencyCode;

    return CallbackShortcuts(
      bindings: {
        const SingleActivator(LogicalKeyboardKey.f1): () => c.setOrderMode(OrderMode.quickSale),
        const SingleActivator(LogicalKeyboardKey.f2): () => c.setOrderMode(OrderMode.takeaway),
        const SingleActivator(LogicalKeyboardKey.f3): () => c.setOrderMode(OrderMode.dineIn),
        const SingleActivator(LogicalKeyboardKey.f4): () => c.setOrderMode(OrderMode.delivery),
        const SingleActivator(LogicalKeyboardKey.f5): () => c.setOrderMode(OrderMode.qrOrder),
        const SingleActivator(LogicalKeyboardKey.f6): () => c.setOrderMode(OrderMode.driveThru),
        const SingleActivator(LogicalKeyboardKey.f9): _sendToBarista,
        const SingleActivator(LogicalKeyboardKey.f12): _openTenderModal,
        const SingleActivator(LogicalKeyboardKey.keyF, control: true): () => _searchFocus.requestFocus(),
        const SingleActivator(LogicalKeyboardKey.keyD, control: true): () => c.kickDrawerWithManagerPin('1111'),
      },
      child: Focus(
        autofocus: true,
        child: Scaffold(
          body: Column(
            children: [
              PosTopBar(
                controller: c,
                onKickDrawer: () => c.kickDrawerWithManagerPin('1111'),
                onReprintLast: () {
                  if (c.lastReceipt != null) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('Reprinting ${c.lastReceipt!.orderNumber}...')),
                    );
                  }
                },
              ),
              Expanded(
                child: Row(
                  children: [
                    // LEFT PANE (62%) — Search, Category Rail & Product Grid
                    Expanded(
                      flex: 62,
                      child: Padding(
                        padding: const EdgeInsets.all(16),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            TextField(
                              controller: _searchCtrl,
                              focusNode: _searchFocus,
                              onChanged: c.setSearchQuery,
                              onSubmitted: (val) {
                                if (c.handleBarcodeScan(val)) {
                                  _searchCtrl.clear();
                                  c.setSearchQuery('');
                                }
                              },
                              decoration: InputDecoration(
                                hintText: 'Search menu or scan SKU barcode (Ctrl+F)...',
                                prefixIcon: const Icon(Icons.search),
                                suffixIcon: const Icon(Icons.qr_code_scanner, size: 20),
                                border: OutlineInputBorder(
                                  borderRadius: BorderRadius.circular(10),
                                  borderSide: BorderSide(color: borderColor),
                                ),
                                contentPadding: const EdgeInsets.symmetric(vertical: 12),
                              ),
                            ),
                            const SizedBox(height: 12),

                            // Category Rail
                            SizedBox(
                              height: 40,
                              child: ListView(
                                scrollDirection: Axis.horizontal,
                                children: [
                                  _CategoryChip(
                                    label: 'All (${c.menuItems.length})',
                                    selected: c.selectedCategoryId == 'all',
                                    onTap: () => c.setCategory('all'),
                                  ),
                                  for (final cat in c.categories)
                                    _CategoryChip(
                                      label: cat.name,
                                      selected: c.selectedCategoryId == cat.id,
                                      onTap: () => c.setCategory(cat.id),
                                    ),
                                ],
                              ),
                            ),
                            const SizedBox(height: 14),

                            // Product Tile Grid
                            Expanded(
                              child: GridView.builder(
                                gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                                  maxCrossAxisExtent: 230,
                                  childAspectRatio: 1.25,
                                  crossAxisSpacing: 12,
                                  mainAxisSpacing: 12,
                                ),
                                itemCount: c.filteredItems.length,
                                itemBuilder: (_, idx) {
                                  final item = c.filteredItems[idx];
                                  return Card(
                                    child: InkWell(
                                      onTap: item.isAvailable ? () => _onTapProduct(item) : null,
                                      borderRadius: BorderRadius.circular(12),
                                      child: Padding(
                                        padding: const EdgeInsets.all(14),
                                        child: Column(
                                          crossAxisAlignment: CrossAxisAlignment.start,
                                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                          children: [
                                            Row(
                                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                                              children: [
                                                Text(
                                                  item.sku,
                                                  style: TextStyle(
                                                    fontSize: 10,
                                                    color: isDark ? PosColors.darkMuted : PosColors.lightMuted,
                                                  ),
                                                ),
                                                Container(
                                                  padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                                                  decoration: BoxDecoration(
                                                    color: item.isAvailable
                                                        ? PosColors.emerald.withValues(alpha: 0.12)
                                                        : PosColors.rose.withValues(alpha: 0.15),
                                                    borderRadius: BorderRadius.circular(4),
                                                  ),
                                                  child: Text(
                                                    item.isAvailable ? item.prepStation.toUpperCase() : 'OUT OF STOCK',
                                                    style: TextStyle(
                                                      fontSize: 9,
                                                      fontWeight: FontWeight.w700,
                                                      color: item.isAvailable ? PosColors.emerald : PosColors.rose,
                                                    ),
                                                  ),
                                                ),
                                              ],
                                            ),
                                            Text(
                                              item.name,
                                              maxLines: 2,
                                              overflow: TextOverflow.ellipsis,
                                              style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
                                            ),
                                            Text(
                                              '$currency ${item.basePrice.toStringAsFixed(2)}',
                                              style: PosTheme.tabularPrice.copyWith(
                                                fontSize: 15,
                                                color: PosColors.terracotta,
                                              ),
                                            ),
                                          ],
                                        ),
                                      ),
                                    ),
                                  );
                                },
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),

                    // RIGHT PANE (38%) — Live Ticket & Checkout Dock
                    Expanded(
                      flex: 38,
                      child: Container(
                        decoration: BoxDecoration(
                          color: isDark ? PosColors.darkCard : PosColors.lightCard,
                          border: Border(left: BorderSide(color: borderColor)),
                        ),
                        padding: const EdgeInsets.all(16),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            Row(
                              mainAxisAlignment: MainAxisAlignment.spaceBetween,
                              children: [
                                Text(
                                  'Current Ticket (${c.cart.length})',
                                  style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                                ),
                                if (c.cart.isNotEmpty)
                                  TextButton(
                                    onPressed: c.clearCart,
                                    child: const Text('Clear', style: TextStyle(color: PosColors.rose)),
                                  ),
                              ],
                            ),
                            const SizedBox(height: 8),
                            TextField(
                              controller: _customerCtrl,
                              onChanged: c.setCustomerNameOrBuzzer,
                              decoration: InputDecoration(
                                hintText: 'Customer name or buzzer # (e.g. Sarah / #14)',
                                prefixIcon: const Icon(Icons.person_outline, size: 18),
                                isDense: true,
                                border: OutlineInputBorder(borderRadius: BorderRadius.circular(8)),
                              ),
                            ),
                            const SizedBox(height: 12),
                            Expanded(
                              child: c.cart.isEmpty
                                  ? Center(
                                      child: Text(
                                        'Tap any item or scan a barcode to start an order',
                                        style: TextStyle(
                                          color: isDark ? PosColors.darkMuted : PosColors.lightMuted,
                                        ),
                                      ),
                                    )
                                  : ListView.separated(
                                      itemCount: c.cart.length,
                                      separatorBuilder: (_, __) => Divider(height: 12, color: borderColor),
                                      itemBuilder: (_, idx) {
                                        final line = c.cart[idx];
                                        final lineGross = c.calculator.lineGross(line);
                                        return Row(
                                          crossAxisAlignment: CrossAxisAlignment.start,
                                          children: [
                                            Expanded(
                                              child: Column(
                                                crossAxisAlignment: CrossAxisAlignment.start,
                                                children: [
                                                  Text(
                                                    line.itemName,
                                                    style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 13),
                                                  ),
                                                  for (final mod in line.modifiers)
                                                    Text(
                                                      '  + ${mod.modifierName}',
                                                      style: TextStyle(
                                                        fontSize: 11,
                                                        color: isDark ? PosColors.darkMuted : PosColors.lightMuted,
                                                      ),
                                                    ),
                                                  if (line.notes.isNotEmpty)
                                                    Text(
                                                      '  Note: ${line.notes}',
                                                      style: const TextStyle(fontSize: 11, color: PosColors.amber),
                                                    ),
                                                ],
                                              ),
                                            ),
                                            Row(
                                              mainAxisSize: MainAxisSize.min,
                                              children: [
                                                IconButton(
                                                  icon: const Icon(Icons.remove_circle_outline, size: 18),
                                                  onPressed: () => c.decrementLine(line.id),
                                                ),
                                                Text(
                                                  line.quantity.toString(),
                                                  style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                                                ),
                                                IconButton(
                                                  icon: const Icon(Icons.add_circle_outline, size: 18),
                                                  onPressed: () => c.incrementLine(line.id),
                                                ),
                                              ],
                                            ),
                                            const SizedBox(width: 6),
                                            Text(
                                              lineGross.toStringAsFixed(2),
                                              style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                                            ),
                                          ],
                                        );
                                      },
                                    ),
                            ),
                            Divider(height: 20, color: borderColor),
                            _TotalsRow('Subtotal', '$currency ${totals.subtotal.toStringAsFixed(2)}'),
                            const SizedBox(height: 4),
                            _TotalsRow('Tax (16%)', '$currency ${totals.taxTotal.toStringAsFixed(2)}'),
                            const SizedBox(height: 8),
                            _TotalsRow(
                              'Grand Total',
                              '$currency ${totals.grandTotal.toStringAsFixed(2)}',
                              bold: true,
                            ),
                            const SizedBox(height: 14),
                            Row(
                              children: [
                                Expanded(
                                  child: OutlinedButton(
                                    onPressed: c.cart.isEmpty ? null : _sendToBarista,
                                    child: const Text('Send to Barista (F9)'),
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Expanded(
                                  child: ElevatedButton(
                                    onPressed: c.cart.isEmpty ? null : _openTenderModal,
                                    child: const Text('Charge & Print (F12)'),
                                  ),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _CategoryChip extends StatelessWidget {
  const _CategoryChip({
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: ChoiceChip(
        label: Text(label),
        selected: selected,
        onSelected: (_) => onTap(),
      ),
    );
  }
}

class _TotalsRow extends StatelessWidget {
  const _TotalsRow(this.label, this.value, {this.bold = false});
  final String label;
  final String value;
  final bool bold;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: TextStyle(
            fontSize: bold ? 16 : 13,
            fontWeight: bold ? FontWeight.w700 : FontWeight.w500,
          ),
        ),
        Text(
          value,
          style: PosTheme.tabularPrice.copyWith(
            fontSize: bold ? 20 : 13,
            color: bold ? PosColors.terracotta : null,
          ),
        ),
      ],
    );
  }
}
