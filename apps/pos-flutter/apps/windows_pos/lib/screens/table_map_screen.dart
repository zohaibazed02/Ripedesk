import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../controllers/dine_in_controller.dart';
import '../controllers/pos_controller.dart';
import '../theme/pos_theme.dart';
import '../widgets/pos_top_bar.dart';
import '../widgets/table_order_panel.dart';

class TableMapScreen extends StatelessWidget {
  const TableMapScreen({
    super.key,
    required this.posController,
    required this.dineInController,
  });

  final PosController posController;
  final DineInController dineInController;

  Color _statusColor(TableStatus status) {
    return switch (status) {
      TableStatus.available => PosColors.emerald,
      TableStatus.occupied => PosColors.terracotta,
      TableStatus.reserved => const Color(0xFF0284C7),
      TableStatus.dirty => PosColors.rose,
    };
  }

  void _onTapTable(BuildContext context, DiningTable table) {
    if (dineInController.isTableLocked) return;
    if (table.status == TableStatus.available) {
      dineInController.openTable(
        table.id,
        guestCount: table.capacity >= 4 ? 4 : 2,
        waiterName: posController.activeCashierName,
      );
    } else if (table.status == TableStatus.occupied) {
      dineInController.selectTable(table.id);
    } else if (table.status == TableStatus.dirty) {
      dineInController.markTableClean(table.id);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Table ${table.tableCode} marked Clean / Available')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: dineInController,
      builder: (_, __) {
        final floor = dineInController.activeFloor;
        final selectedTable = dineInController.selectedTableId != null
            ? dineInController.findTable(dineInController.selectedTableId!)
            : null;

        return Scaffold(
          body: Column(
            children: [
              PosTopBar(
                controller: posController,
                onKickDrawer: () => posController.kickDrawerWithManagerPin('1111'),
                onReprintLast: () {},
              ),

              // Isolated Terminal Guard Banner
              if (dineInController.isTableLocked)
                Container(
                  width: double.infinity,
                  color: PosColors.rose.withValues(alpha: 0.14),
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                  child: Row(
                    children: [
                      const Icon(Icons.wifi_off, color: PosColors.rose, size: 18),
                      const SizedBox(width: 8),
                      const Expanded(
                        child: Text(
                          'Branch LAN Offline — Shared Tables & Cross-Terminal Split Checks Paused',
                          style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: PosColors.rose),
                        ),
                      ),
                      TextButton(
                        onPressed: () => posController.setOrderMode(OrderMode.quickSale),
                        child: const Text('Switch to Quick Sale Cash'),
                      ),
                    ],
                  ),
                ),

              // Floor Tabs & Status Legend
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
                child: Row(
                  children: [
                    for (final f in dineInController.floors)
                      Padding(
                        padding: const EdgeInsets.only(right: 8),
                        child: ChoiceChip(
                          label: Text(f.name),
                          selected: f.id == floor.id,
                          onSelected: (_) => dineInController.selectFloor(f.id),
                        ),
                      ),
                    const Spacer(),
                    _LegendDot('Available', PosColors.emerald),
                    const SizedBox(width: 12),
                    _LegendDot('Occupied', PosColors.terracotta),
                    const SizedBox(width: 12),
                    _LegendDot('Reserved', const Color(0xFF0284C7)),
                    const SizedBox(width: 12),
                    _LegendDot('Dirty', PosColors.rose),
                  ],
                ),
              ),

              Expanded(
                child: Row(
                  children: [
                    // Spatial Floor Plan Canvas
                    Expanded(
                      flex: 65,
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(16),
                        child: SizedBox(
                          height: 420,
                          width: 720,
                          child: Stack(
                            children: floor.tables.map((table) {
                              final color = _statusColor(table.status);
                              final session = dineInController.sessions[table.id];
                              final balance = session != null
                                  ? dineInController.calculator.calculate(session.allItems).grandTotal
                                  : null;

                              return Positioned(
                                left: table.posX.toDouble(),
                                top: table.posY.toDouble(),
                                child: InkWell(
                                  onTap: () => _onTapTable(context, table),
                                  borderRadius: BorderRadius.circular(table.shape == 'circle' ? 60 : 12),
                                  child: Container(
                                    width: 124,
                                    height: 112,
                                    padding: const EdgeInsets.all(10),
                                    decoration: BoxDecoration(
                                      color: color.withValues(alpha: 0.12),
                                      borderRadius: BorderRadius.circular(table.shape == 'circle' ? 56 : 12),
                                      border: Border.all(
                                        color: dineInController.selectedTableId == table.id
                                            ? PosColors.espresso
                                            : color,
                                        width: 2,
                                      ),
                                    ),
                                    child: Column(
                                      mainAxisAlignment: MainAxisAlignment.center,
                                      children: [
                                        Text(
                                          table.tableCode,
                                          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w800, color: color),
                                        ),
                                        Text(
                                          session != null
                                              ? '${session.guestCount}G · R${session.order.rounds.length}'
                                              : '${table.capacity} Seats',
                                          style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w600),
                                        ),
                                        if (balance != null)
                                          Text(
                                            '${posController.branch.currencyCode} ${balance.toStringAsFixed(0)}',
                                            style: PosTheme.tabularPrice.copyWith(fontSize: 11),
                                          ),
                                      ],
                                    ),
                                  ),
                                ),
                              );
                            }).toList(),
                          ),
                        ),
                      ),
                    ),

                    if (selectedTable != null && dineInController.sessions.containsKey(selectedTable.id))
                      Expanded(
                        flex: 35,
                        child: TableOrderPanel(
                          dineIn: dineInController,
                          table: selectedTable,
                          currencyCode: posController.branch.currencyCode,
                        ),
                      ),
                  ],
                ),
              ),
            ],
          ),
        );
      },
    );
  }
}

class _LegendDot extends StatelessWidget {
  const _LegendDot(this.label, this.color);
  final String label;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(width: 8, height: 8, decoration: BoxDecoration(color: color, shape: BoxShape.circle)),
        const SizedBox(width: 4),
        Text(label, style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w600)),
      ],
    );
  }
}
