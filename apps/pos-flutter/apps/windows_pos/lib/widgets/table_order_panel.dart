import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../controllers/dine_in_controller.dart';
import '../data/pilot_catalog.dart';
import '../theme/pos_theme.dart';
import 'split_check_modal.dart';

class TableOrderPanel extends StatelessWidget {
  const TableOrderPanel({
    super.key,
    required this.dineIn,
    required this.table,
    required this.currencyCode,
  });

  final DineInController dineIn;
  final DiningTable table;
  final String currencyCode;

  @override
  Widget build(BuildContext context) {
    final session = dineIn.sessions[table.id];
    if (session == null) return const SizedBox.shrink();

    final totals = dineIn.calculator.calculate(session.allItems);
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final borderColor = isDark ? PosColors.darkBorder : PosColors.lightBorder;

    return Container(
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
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Table ${table.tableCode} · ${session.order.orderNumber}',
                    style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                  ),
                  Text(
                    '${session.guestCount} Guests · ${session.waiterName} · v${session.order.orderVersion}',
                    style: TextStyle(fontSize: 11, color: isDark ? PosColors.darkMuted : PosColors.lightMuted),
                  ),
                ],
              ),
              IconButton(
                onPressed: () => dineIn.selectTable(null),
                icon: const Icon(Icons.close),
              ),
            ],
          ),

          if (dineIn.conflictMessage != null) ...[
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.all(10),
              decoration: BoxDecoration(
                color: PosColors.rose.withValues(alpha: 0.12),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: PosColors.rose),
              ),
              child: Text(
                dineIn.conflictMessage!,
                style: const TextStyle(fontSize: 12, color: PosColors.rose, fontWeight: FontWeight.w600),
              ),
            ),
          ],

          const SizedBox(height: 10),
          // Quick-add Dine-In items bar
          SizedBox(
            height: 38,
            child: ListView(
              scrollDirection: Axis.horizontal,
              children: PilotCatalog.menuItems.where((i) => i.isAvailable).take(5).map((item) {
                return Padding(
                  padding: const EdgeInsets.only(right: 6),
                  child: ActionChip(
                    label: Text('+ ${item.name}', style: const TextStyle(fontSize: 11)),
                    onPressed: dineIn.isTableLocked
                        ? null
                        : () => dineIn.addDraftItem(table.id, item, seatNumber: 1, checkNumber: 1),
                  ),
                );
              }).toList(),
            ),
          ),
          const Divider(height: 20),

          // Sent Rounds + Draft Round
          Expanded(
            child: ListView(
              children: [
                for (final round in session.order.rounds) ...[
                  Text(
                    'Round #${round.roundNumber} · SENT TO KITCHEN',
                    style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: PosColors.emerald),
                  ),
                  const SizedBox(height: 4),
                  for (final item in round.items)
                    ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      title: Text('${item.quantity}x ${item.itemName}', style: const TextStyle(fontSize: 13)),
                      subtitle: Text('${item.prepStation.toUpperCase()} · Check #${item.checkNumber}', style: const TextStyle(fontSize: 11)),
                      trailing: Text(
                        dineIn.calculator.lineGross(item).toStringAsFixed(2),
                        style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                      ),
                    ),
                  const Divider(height: 16),
                ],
                if (session.draftItems.isNotEmpty) ...[
                  Text(
                    'Round #${session.order.rounds.length + 1} · DRAFT (UNSENT)',
                    style: const TextStyle(fontSize: 11, fontWeight: FontWeight.w700, color: PosColors.amber),
                  ),
                  const SizedBox(height: 4),
                  for (final item in session.draftItems)
                    ListTile(
                      dense: true,
                      contentPadding: EdgeInsets.zero,
                      title: Text('${item.quantity}x ${item.itemName}', style: const TextStyle(fontSize: 13)),
                      subtitle: Text('${item.prepStation.toUpperCase()} · Check #${item.checkNumber}', style: const TextStyle(fontSize: 11)),
                      trailing: Text(
                        dineIn.calculator.lineGross(item).toStringAsFixed(2),
                        style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                      ),
                    ),
                ],
              ],
            ),
          ),

          Divider(height: 16, color: borderColor),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text('Table Total', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
              Text(
                '$currencyCode ${totals.grandTotal.toStringAsFixed(2)}',
                style: PosTheme.tabularPrice.copyWith(fontSize: 18, color: PosColors.terracotta),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: session.draftItems.isEmpty || dineIn.isTableLocked
                      ? null
                      : () => dineIn.fireDraftRound(table.id),
                  child: const Text('Fire Round (F9)'),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: ElevatedButton(
                  onPressed: session.allItems.isEmpty || dineIn.isTableLocked
                      ? null
                      : () => showDialog(
                            context: context,
                            builder: (_) => SplitCheckModal(
                              dineIn: dineIn,
                              tableId: table.id,
                              currencyCode: currencyCode,
                            ),
                          ),
                  child: const Text('Split / Settle'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
