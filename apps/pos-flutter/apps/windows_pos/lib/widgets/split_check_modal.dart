import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../controllers/dine_in_controller.dart';
import '../theme/pos_theme.dart';

class SplitCheckModal extends StatefulWidget {
  const SplitCheckModal({
    super.key,
    required this.dineIn,
    required this.tableId,
    required this.currencyCode,
  });

  final DineInController dineIn;
  final String tableId;
  final String currencyCode;

  @override
  State<SplitCheckModal> createState() => _SplitCheckModalState();
}

class _SplitCheckModalState extends State<SplitCheckModal> {
  int _evenWays = 2;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.dineIn,
      builder: (_, __) {
        final session = widget.dineIn.sessions[widget.tableId];
        if (session == null) {
          return const SizedBox.shrink();
        }
        final totals = widget.dineIn.calculator.calculate(session.allItems);
        final remaining = totals.grandTotal - widget.dineIn.totalPaid(widget.tableId);
        final evenShares = DineInController.splitEvenly(
          remaining > Decimal.zero ? remaining : Decimal.zero,
          _evenWays,
        );

        return Dialog(
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 680, maxHeight: 680),
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        'Split Checks & Settlement · ${session.order.orderNumber}',
                        style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
                      ),
                      IconButton(
                        onPressed: () => Navigator.of(context).pop(),
                        icon: const Icon(Icons.close),
                      ),
                    ],
                  ),
                  const Divider(height: 20),

                  // Even Split Preview Bar
                  Row(
                    children: [
                      const Text('Split Remaining Evenly:', style: TextStyle(fontWeight: FontWeight.w600)),
                      const SizedBox(width: 10),
                      for (final n in [2, 3, 4])
                        Padding(
                          padding: const EdgeInsets.only(right: 6),
                          child: ChoiceChip(
                            label: Text('$n Ways'),
                            selected: _evenWays == n,
                            onSelected: (_) => setState(() => _evenWays = n),
                          ),
                        ),
                      const Spacer(),
                      Text(
                        'Check 1: ${widget.currencyCode} ${evenShares.first.toStringAsFixed(2)}',
                        style: PosTheme.tabularPrice.copyWith(color: PosColors.terracotta),
                      ),
                    ],
                  ),
                  const SizedBox(height: 14),

                  // Per-Item Check Assignment
                  Expanded(
                    child: ListView(
                      children: [
                        const Text('Assign Items to Checks (Tap Check #)', style: TextStyle(fontWeight: FontWeight.w700)),
                        const SizedBox(height: 8),
                        for (final item in session.allItems)
                          ListTile(
                            dense: true,
                            contentPadding: EdgeInsets.zero,
                            title: Text('${item.quantity}x ${item.itemName}'),
                            subtitle: Text(item.seatNumber != null ? 'Seat ${item.seatNumber}' : 'Shared'),
                            trailing: Wrap(
                              spacing: 4,
                              children: [1, 2, 3].map((cNum) {
                                final active = item.checkNumber == cNum;
                                return ChoiceChip(
                                  label: Text('C$cNum'),
                                  selected: active,
                                  onSelected: (_) => widget.dineIn.assignItemCheck(widget.tableId, item.id, cNum),
                                );
                              }).toList(),
                            ),
                          ),
                        const Divider(height: 24),
                        const Text('Independent Check Settlement', style: TextStyle(fontWeight: FontWeight.w700)),
                        const SizedBox(height: 8),
                        for (final entry in totals.checks.entries) ...[
                          Builder(builder: (_) {
                            final checkNum = entry.key;
                            final ct = entry.value;
                            final paid = widget.dineIn.paidForCheck(widget.tableId, checkNum);
                            final isPaid = paid >= ct.grandTotal;
                            final due = ct.grandTotal - paid;
                            return Card(
                              child: Padding(
                                padding: const EdgeInsets.all(12),
                                child: Row(
                                  children: [
                                    Expanded(
                                      child: Column(
                                        crossAxisAlignment: CrossAxisAlignment.start,
                                        children: [
                                          Text(
                                            'Check #$checkNum · ${isPaid ? "PAID" : (paid > Decimal.zero ? "PARTIALLY PAID" : "UNPAID")}',
                                            style: TextStyle(
                                              fontWeight: FontWeight.w700,
                                              color: isPaid ? PosColors.emerald : PosColors.terracotta,
                                            ),
                                          ),
                                          Text(
                                            'Total: ${widget.currencyCode} ${ct.grandTotal.toStringAsFixed(2)} · Due: ${due.toStringAsFixed(2)}',
                                            style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                                          ),
                                        ],
                                      ),
                                    ),
                                    if (!isPaid) ...[
                                      OutlinedButton(
                                        onPressed: () async {
                                          final closed = await widget.dineIn.recordCheckPayment(
                                            widget.tableId,
                                            checkNumber: checkNum,
                                            method: 'cash',
                                            amount: due,
                                            tenderedAmount: due,
                                          );
                                          if (closed && context.mounted) {
                                            Navigator.of(context).pop(true);
                                          }
                                        },
                                        child: const Text('Pay Cash'),
                                      ),
                                      const SizedBox(width: 8),
                                      ElevatedButton(
                                        onPressed: () async {
                                          final closed = await widget.dineIn.recordCheckPayment(
                                            widget.tableId,
                                            checkNumber: checkNum,
                                            method: 'card_terminal',
                                            amount: due,
                                            tenderedAmount: due,
                                          );
                                          if (closed && context.mounted) {
                                            Navigator.of(context).pop(true);
                                          }
                                        },
                                        child: const Text('Pay Card'),
                                      ),
                                    ],
                                  ],
                                ),
                              ),
                            );
                          }),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
