import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../theme/pos_theme.dart';

class TenderResult {
  const TenderResult({
    required this.paymentMethod,
    required this.tenderedAmount,
  });

  final String paymentMethod;
  final Decimal tenderedAmount;
}

class TenderModal extends StatefulWidget {
  const TenderModal({
    super.key,
    required this.grandTotal,
    required this.currencyCode,
    required this.connectionMode,
  });

  final Decimal grandTotal;
  final String currencyCode;
  final ConnectionMode connectionMode;

  @override
  State<TenderModal> createState() => _TenderModalState();
}

class _TenderModalState extends State<TenderModal> {
  String _method = 'cash';
  String _rawInput = '';

  Decimal get _tendered {
    if (_rawInput.isEmpty) return widget.grandTotal;
    return Decimal.tryParse(_rawInput) ?? Decimal.zero;
  }

  Decimal get _changeDue =>
      _tendered > widget.grandTotal ? _tendered - widget.grandTotal : Decimal.zero;

  bool get _canComplete => _tendered >= widget.grandTotal;

  void _appendKey(String key) {
    setState(() {
      if (key == 'C') {
        _rawInput = '';
      } else if (key == '⌫') {
        if (_rawInput.isNotEmpty) {
          _rawInput = _rawInput.substring(0, _rawInput.length - 1);
        }
      } else if (key == '.' && _rawInput.contains('.')) {
        return;
      } else {
        _rawInput += key;
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final isIsolated = widget.connectionMode == ConnectionMode.isolatedTerminal;
    final quickDenominations = [
      ('Exact', widget.grandTotal),
      ('1,000', Decimal.fromInt(1000)),
      ('2,000', Decimal.fromInt(2000)),
      ('5,000', Decimal.fromInt(5000)),
    ];

    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 480),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Text(
                    'Charge & Tender',
                    style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
                  ),
                  IconButton(
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              const SizedBox(height: 12),

              // Payment method tabs
              Row(
                children: [
                  Expanded(
                    child: ChoiceChip(
                      label: const Center(child: Text('Cash')),
                      selected: _method == 'cash',
                      onSelected: (_) => setState(() => _method = 'cash'),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: ChoiceChip(
                      label: Center(
                        child: Text(isIsolated ? 'Card (Offline Lock)' : 'Card Terminal'),
                      ),
                      selected: _method == 'card_terminal',
                      onSelected: isIsolated
                          ? null
                          : (_) => setState(() {
                                _method = 'card_terminal';
                                _rawInput = widget.grandTotal.toStringAsFixed(2);
                              }),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 16),

              // Total & Change Due summary
              Container(
                padding: const EdgeInsets.all(14),
                decoration: BoxDecoration(
                  color: PosColors.terracotta.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(12),
                ),
                child: Column(
                  children: [
                    _RowSummary('Grand Total', '${widget.currencyCode} ${widget.grandTotal.toStringAsFixed(2)}'),
                    const SizedBox(height: 6),
                    _RowSummary('Tendered', '${widget.currencyCode} ${_tendered.toStringAsFixed(2)}'),
                    const Divider(height: 16),
                    _RowSummary(
                      'Change Due',
                      '${widget.currencyCode} ${_changeDue.toStringAsFixed(2)}',
                      highlight: true,
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 12),

              // Quick Cash Denomination Buttons
              if (_method == 'cash') ...[
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: quickDenominations.map((d) {
                    final (label, amount) = d;
                    return OutlinedButton(
                      onPressed: () => setState(() => _rawInput = amount.toStringAsFixed(2)),
                      child: Text(label),
                    );
                  }).toList(),
                ),
                const SizedBox(height: 12),
                GridView.count(
                  crossAxisCount: 3,
                  shrinkWrap: true,
                  mainAxisSpacing: 8,
                  crossAxisSpacing: 8,
                  childAspectRatio: 2.3,
                  physics: const NeverScrollableScrollPhysics(),
                  children: ['7', '8', '9', '4', '5', '6', '1', '2', '3', 'C', '0', '⌫']
                      .map((k) => OutlinedButton(
                            onPressed: () => _appendKey(k),
                            child: Text(k, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                          ))
                      .toList(),
                ),
                const SizedBox(height: 14),
              ],

              ElevatedButton(
                onPressed: _canComplete
                    ? () => Navigator.of(context).pop(TenderResult(
                          paymentMethod: _method,
                          tenderedAmount: _tendered,
                        ))
                    : null,
                child: Text(
                  _method == 'cash' ? 'Complete Cash & Open Drawer' : 'Authorize Card Terminal',
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _RowSummary extends StatelessWidget {
  const _RowSummary(this.label, this.value, {this.highlight = false});
  final String label;
  final String value;
  final bool highlight;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: TextStyle(
            fontSize: highlight ? 15 : 13,
            fontWeight: highlight ? FontWeight.w700 : FontWeight.w500,
          ),
        ),
        Text(
          value,
          style: PosTheme.tabularPrice.copyWith(
            fontSize: highlight ? 18 : 14,
            color: highlight ? PosColors.emerald : null,
          ),
        ),
      ],
    );
  }
}
