import 'package:flutter/material.dart';
import '../controllers/staff_app_controller.dart';
import '../theme/staff_theme.dart';

class TablesideOrderScreen extends StatefulWidget {
  const TablesideOrderScreen({
    super.key,
    required this.controller,
    required this.tableId,
  });

  final StaffAppController controller;
  final String tableId;

  @override
  State<TablesideOrderScreen> createState() => _TablesideOrderScreenState();
}

class _TablesideOrderScreenState extends State<TablesideOrderScreen> {
  int _selectedSeat = 1;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: widget.controller,
      builder: (_, __) {
        final active = widget.controller.tableOrders[widget.tableId];
        if (active == null) {
          return const Scaffold(body: Center(child: Text('No active table session')));
        }
        final totals = widget.controller.calculator.calculate(active.allItems);

        return Scaffold(
          appBar: AppBar(
            title: Text('${active.order.orderNumber} · v${active.order.orderVersion}'),
            actions: [
              TextButton(
                onPressed: () {
                  widget.controller.requestBill(widget.tableId);
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Bill requested at cashier desk')),
                  );
                },
                child: Text(active.billRequested ? 'Bill Sent' : 'Request Bill'),
              ),
            ],
          ),
          body: Column(
            children: [
              // Seat selector
              SizedBox(
                height: 48,
                child: ListView(
                  scrollDirection: Axis.horizontal,
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                  children: List.generate(6, (idx) {
                    final seat = idx + 1;
                    final isSelected = _selectedSeat == seat;
                    return Padding(
                      padding: const EdgeInsets.only(right: 6),
                      child: ChoiceChip(
                        label: Text('Seat $seat'),
                        selected: isSelected,
                        selectedColor: const Color(0xFF0F172A),
                        backgroundColor: Colors.white,
                        checkmarkColor: Colors.white,
                        labelStyle: TextStyle(
                          color: isSelected ? Colors.white : const Color(0xFF0F172A),
                          fontWeight: FontWeight.w600,
                        ),
                        onSelected: (_) => setState(() => _selectedSeat = seat),
                      ),
                    );
                  }),
                ),
              ),

              // Menu items tap-to-add list
              Expanded(
                child: ListView.separated(
                  padding: const EdgeInsets.all(14),
                  itemCount: widget.controller.menuItems.length,
                  separatorBuilder: (_, __) => const SizedBox(height: 8),
                  itemBuilder: (_, idx) {
                    final item = widget.controller.menuItems[idx];
                    return Card(
                      child: ListTile(
                        title: Text(item.name, style: const TextStyle(fontWeight: FontWeight.w700)),
                        subtitle: Text(item.prepStation.toUpperCase(), style: const TextStyle(fontSize: 11)),
                        trailing: ElevatedButton(
                          onPressed: () => widget.controller.addDraftItem(
                            widget.tableId,
                            item,
                            seatNumber: _selectedSeat,
                          ),
                          child: Text('+ ${item.basePrice.toStringAsFixed(0)}'),
                        ),
                      ),
                    );
                  },
                ),
              ),

              // Bottom Draft & Fire Round Dock
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  border: Border(top: BorderSide(color: StaffColors.lightBorder)),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          'Draft: ${active.draftItems.length} · Sent Rounds: ${active.order.rounds.length}',
                          style: const TextStyle(fontWeight: FontWeight.w600),
                        ),
                        Text(
                          '${widget.controller.currencyCode} ${totals.grandTotal.toStringAsFixed(2)}',
                          style: StaffTheme.tabularPrice.copyWith(fontSize: 17, color: StaffColors.terracotta),
                        ),
                      ],
                    ),
                    const SizedBox(height: 10),
                    ElevatedButton(
                      onPressed: active.draftItems.isEmpty
                          ? null
                          : () async {
                              await widget.controller.fireRoundToKitchen(widget.tableId);
                              if (context.mounted) {
                                ScaffoldMessenger.of(context).showSnackBar(
                                  const SnackBar(content: Text('Round fired to Kitchen KDS!')),
                                );
                              }
                            },
                      child: Text('Fire Round #${active.order.rounds.length + 1} to Kitchen'),
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
