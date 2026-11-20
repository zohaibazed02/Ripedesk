import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../controllers/staff_app_controller.dart';
import '../theme/staff_theme.dart';
import 'tableside_order_screen.dart';

class StaffTablesScreen extends StatelessWidget {
  const StaffTablesScreen({super.key, required this.controller});

  final StaffAppController controller;

  Color _statusColor(TableStatus status) {
    return switch (status) {
      TableStatus.available => StaffColors.emerald,
      TableStatus.occupied => StaffColors.terracotta,
      TableStatus.reserved => const Color(0xFF0284C7),
      TableStatus.dirty => StaffColors.rose,
    };
  }

  @override
  Widget build(BuildContext context) {
    final session = controller.activeSession;
    final tables = controller.visibleTables;

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(controller.branchName, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
            if (session != null)
              Text(
                '${session.name} (${session.role})',
                style: const TextStyle(fontSize: 11, color: StaffColors.lightMuted),
              ),
          ],
        ),
        actions: [
          IconButton(
            tooltip: 'Branch LAN URL',
            onPressed: () {
              final urlCtrl = TextEditingController(
                text: controller.lanClient?.branchBaseUrl ?? 'http://192.168.1.10:8090',
              );
              showDialog<void>(
                context: context,
                builder: (ctx) => AlertDialog(
                  title: const Text('Branch LAN Server'),
                  content: TextField(
                    controller: urlCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Branch PC URL',
                      hintText: 'http://192.168.1.10:8090',
                    ),
                  ),
                  actions: [
                    TextButton(
                      onPressed: () => Navigator.pop(ctx),
                      child: const Text('Cancel'),
                    ),
                    ElevatedButton(
                      onPressed: () {
                        controller.updateLanUrl(urlCtrl.text);
                        Navigator.pop(ctx);
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text('LAN URL updated to ${urlCtrl.text.trim()}')),
                        );
                      },
                      child: const Text('Connect'),
                    ),
                  ],
                ),
              );
            },
            icon: const Icon(Icons.settings_ethernet),
          ),
          IconButton(
            tooltip: 'Toggle Theme',
            onPressed: controller.toggleTheme,
            icon: Icon(controller.isDarkMode ? Icons.light_mode : Icons.dark_mode),
          ),
          TextButton.icon(
            onPressed: controller.lockSession,
            icon: const Icon(Icons.lock_outline, size: 18),
            label: const Text('Switch'),
          ),
        ],
      ),
      body: Column(
        children: [
          // Live KDS Ready-to-Serve Alert Banner
          if (controller.readyAlerts.isNotEmpty)
            Container(
              width: double.infinity,
              color: StaffColors.emerald.withValues(alpha: 0.14),
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
              child: Row(
                children: [
                  const Icon(Icons.notifications_active, color: StaffColors.emerald),
                  const SizedBox(width: 10),
                  Expanded(
                    child: Text(
                      'READY TO SERVE: Table ${controller.readyAlerts.first.tableCode} · ${controller.readyAlerts.first.station}',
                      style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700, color: StaffColors.emerald),
                    ),
                  ),
                  TextButton(
                    onPressed: () => controller.acknowledgeServed(controller.readyAlerts.first.id),
                    child: const Text('Mark Served'),
                  ),
                ],
              ),
            ),

          // My Tables / All Tables Toggle
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
            child: Row(
              children: [
                ChoiceChip(
                  label: const Text('My Tables'),
                  selected: controller.onlyMyTables,
                  onSelected: (_) => controller.setOnlyMyTables(true),
                ),
                const SizedBox(width: 8),
                ChoiceChip(
                  label: const Text('All Tables'),
                  selected: !controller.onlyMyTables,
                  onSelected: (_) => controller.setOnlyMyTables(false),
                ),
              ],
            ),
          ),

          Expanded(
            child: GridView.builder(
              padding: const EdgeInsets.all(16),
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 2,
                mainAxisSpacing: 12,
                crossAxisSpacing: 12,
                childAspectRatio: 1.25,
              ),
              itemCount: tables.length,
              itemBuilder: (_, idx) {
                final table = tables[idx];
                final color = _statusColor(table.status);
                final ord = controller.tableOrders[table.id];
                final total = ord != null
                    ? controller.calculator.calculate(ord.allItems).grandTotal
                    : null;

                return InkWell(
                  onTap: () {
                    if (table.status == TableStatus.available) {
                      controller.openTable(table.id, guestCount: 2);
                    }
                    if (controller.tableOrders.containsKey(table.id)) {
                      Navigator.of(context).push(MaterialPageRoute(
                        builder: (_) => TablesideOrderScreen(
                          controller: controller,
                          tableId: table.id,
                        ),
                      ));
                    }
                  },
                  borderRadius: BorderRadius.circular(14),
                  child: Container(
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: color.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(14),
                      border: Border.all(color: color, width: 2),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text(
                              table.tableCode,
                              style: TextStyle(fontSize: 20, fontWeight: FontWeight.w800, color: color),
                            ),
                            Text(
                              table.status.wireValue.toUpperCase(),
                              style: TextStyle(fontSize: 10, fontWeight: FontWeight.w700, color: color),
                            ),
                          ],
                        ),
                        Text(
                          ord != null
                              ? '${ord.guestCount} Guests · Round ${ord.order.rounds.length}'
                              : '${table.capacity} Seats',
                          style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
                        ),
                        if (total != null)
                          Text(
                            '${controller.currencyCode} ${total.toStringAsFixed(2)}',
                            style: StaffTheme.tabularPrice.copyWith(fontSize: 14),
                          ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
