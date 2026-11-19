import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../theme/pos_theme.dart';

class ModifierSelectionResult {
  const ModifierSelectionResult({
    required this.quantity,
    required this.modifiers,
    required this.notes,
  });

  final Decimal quantity;
  final List<OrderItemModifier> modifiers;
  final String notes;
}

class ModifierDrawer extends StatefulWidget {
  const ModifierDrawer({
    super.key,
    required this.item,
    required this.groups,
    required this.currencyCode,
  });

  final MenuItem item;
  final List<ModifierGroup> groups;
  final String currencyCode;

  @override
  State<ModifierDrawer> createState() => _ModifierDrawerState();
}

class _ModifierDrawerState extends State<ModifierDrawer> {
  final Map<String, Set<String>> _selectedByGroup = {};
  int _quantity = 1;
  String _notes = '';

  static const _quickNotes = ['Extra Hot', 'No Foam', 'Decaf', 'Less Ice', 'Light Sweet'];

  @override
  void initState() {
    super.initState();
    for (final g in widget.groups) {
      _selectedByGroup[g.id] = {};
      if (g.minSelections == 1 && g.modifiers.isNotEmpty) {
        _selectedByGroup[g.id]!.add(g.modifiers.first.id);
      }
    }
  }

  bool get _isValid {
    for (final g in widget.groups) {
      final count = _selectedByGroup[g.id]?.length ?? 0;
      if (count < g.minSelections || count > g.maxSelections) return false;
    }
    return true;
  }

  List<OrderItemModifier> get _chosenModifiers {
    final list = <OrderItemModifier>[];
    for (final g in widget.groups) {
      final ids = _selectedByGroup[g.id] ?? {};
      for (final m in g.modifiers) {
        if (ids.contains(m.id)) {
          list.add(OrderItemModifier(
            modifierId: m.id,
            modifierName: m.name,
            priceDelta: m.priceDelta,
            quantity: Decimal.one,
          ));
        }
      }
    }
    return list;
  }

  Decimal get _liveUnitPrice {
    var total = widget.item.basePrice;
    for (final m in _chosenModifiers) {
      total += m.priceDelta;
    }
    return total;
  }

  void _toggleModifier(ModifierGroup group, Modifier mod) {
    setState(() {
      final set = _selectedByGroup.putIfAbsent(group.id, () => {});
      if (set.contains(mod.id)) {
        if (set.length > group.minSelections) {
          set.remove(mod.id);
        }
      } else {
        if (group.maxSelections == 1) {
          set.clear();
          set.add(mod.id);
        } else if (set.length < group.maxSelections) {
          set.add(mod.id);
        }
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final lineTotal = _liveUnitPrice * Decimal.fromInt(_quantity);

    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 520, maxHeight: 680),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          widget.item.name,
                          style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
                        ),
                        Text(
                          '${widget.currencyCode} ${_liveUnitPrice.toStringAsFixed(2)} each',
                          style: PosTheme.tabularPrice.copyWith(
                            fontSize: 13,
                            color: PosColors.terracotta,
                          ),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              const Divider(height: 24),
              Expanded(
                child: ListView(
                  children: [
                    for (final group in widget.groups) ...[
                      Row(
                        mainAxisAlignment: MainAxisAlignment.spaceBetween,
                        children: [
                          Text(
                            group.name,
                            style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w700),
                          ),
                          Text(
                            group.required
                                ? 'Required (${group.minSelections}-${group.maxSelections})'
                                : 'Optional (up to ${group.maxSelections})',
                            style: TextStyle(
                              fontSize: 11,
                              color: group.required ? PosColors.terracotta : PosColors.lightMuted,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: group.modifiers.map((mod) {
                          final selected = _selectedByGroup[group.id]?.contains(mod.id) ?? false;
                          final deltaText = mod.priceDelta > Decimal.zero
                              ? ' (+${mod.priceDelta.toStringAsFixed(0)})'
                              : '';
                          return FilterChip(
                            selected: selected,
                            onSelected: mod.isAvailable ? (_) => _toggleModifier(group, mod) : null,
                            label: Text('${mod.name}$deltaText'),
                            selectedColor: PosColors.terracotta.withValues(alpha: 0.18),
                            checkmarkColor: PosColors.terracotta,
                          );
                        }).toList(),
                      ),
                      const SizedBox(height: 16),
                    ],
                    const Text(
                      'Kitchen Prep Notes',
                      style: TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
                    ),
                    const SizedBox(height: 8),
                    Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: _quickNotes.map((note) {
                        final active = _notes.contains(note);
                        return ActionChip(
                          label: Text(note, style: const TextStyle(fontSize: 12)),
                          backgroundColor: active ? PosColors.terracotta.withValues(alpha: 0.16) : null,
                          onPressed: () {
                            setState(() {
                              _notes = _notes.isEmpty ? note : '$_notes, $note';
                            });
                          },
                        );
                      }).toList(),
                    ),
                  ],
                ),
              ),
              const Divider(height: 20),
              Row(
                children: [
                  IconButton.outlined(
                    onPressed: _quantity > 1 ? () => setState(() => _quantity--) : null,
                    icon: const Icon(Icons.remove),
                  ),
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 14),
                    child: Text('$_quantity', style: PosTheme.tabularPrice.copyWith(fontSize: 16)),
                  ),
                  IconButton.outlined(
                    onPressed: () => setState(() => _quantity++),
                    icon: const Icon(Icons.add),
                  ),
                  const SizedBox(width: 14),
                  Expanded(
                    child: ElevatedButton(
                      onPressed: _isValid
                          ? () {
                              Navigator.of(context).pop(ModifierSelectionResult(
                                quantity: Decimal.fromInt(_quantity),
                                modifiers: _chosenModifiers,
                                notes: _notes,
                              ));
                            }
                          : null,
                      child: Text(
                        'Add to Order · ${widget.currencyCode} ${lineTotal.toStringAsFixed(2)}',
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
