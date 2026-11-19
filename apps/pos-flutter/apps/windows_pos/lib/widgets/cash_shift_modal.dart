import 'package:decimal/decimal.dart';
import 'package:flutter/material.dart';
import '../theme/pos_theme.dart';

class BlindShiftCloseResult {
  const BlindShiftCloseResult({
    required this.countedCash,
    required this.denominations,
  });

  final Decimal countedCash;
  final Map<String, int> denominations;
}

/// Blind Cash Shift Z-Report Close modal with denomination counter (exact Decimal math).
class CashShiftCloseModal extends StatefulWidget {
  const CashShiftCloseModal({
    super.key,
    required this.currencyCode,
  });

  final String currencyCode;

  @override
  State<CashShiftCloseModal> createState() => _CashShiftCloseModalState();
}

class _CashShiftCloseModalState extends State<CashShiftCloseModal> {
  final Map<int, int> _counts = {
    5000: 0,
    1000: 0,
    500: 0,
    100: 0,
    50: 0,
    10: 0,
  };

  Decimal get _totalCounted {
    var sum = Decimal.zero;
    for (final entry in _counts.entries) {
      sum += Decimal.fromInt(entry.key) * Decimal.fromInt(entry.value);
    }
    return sum;
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 460),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Text(
                'Blind Shift Close & Z-Report',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
              ),
              const SizedBox(height: 4),
              const Text(
                'Count drawer cash by denomination. Variance is calculated server-side on close.',
                style: TextStyle(fontSize: 12, color: PosColors.lightMuted),
              ),
              const Divider(height: 20),
              for (final denom in _counts.keys)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 4),
                  child: Row(
                    children: [
                      SizedBox(
                        width: 90,
                        child: Text(
                          '${widget.currencyCode} $denom',
                          style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.remove_circle_outline, size: 20),
                        onPressed: _counts[denom]! > 0
                            ? () => setState(() => _counts[denom] = _counts[denom]! - 1)
                            : null,
                      ),
                      SizedBox(
                        width: 36,
                        child: Text(
                          '${_counts[denom]}',
                          textAlign: TextAlign.center,
                          style: PosTheme.tabularPrice,
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.add_circle_outline, size: 20),
                        onPressed: () => setState(() => _counts[denom] = _counts[denom]! + 1),
                      ),
                      const Spacer(),
                      Text(
                        '${widget.currencyCode} ${denom * _counts[denom]!}',
                        style: PosTheme.tabularPrice.copyWith(fontSize: 13),
                      ),
                    ],
                  ),
                ),
              const Divider(height: 20),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Text('Total Blind Count', style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700)),
                  Text(
                    '${widget.currencyCode} ${_totalCounted.toStringAsFixed(2)}',
                    style: PosTheme.tabularPrice.copyWith(fontSize: 18, color: PosColors.terracotta),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(BlindShiftCloseResult(
                  countedCash: _totalCounted,
                  denominations: _counts.map((k, v) => MapEntry(k.toString(), v)),
                )),
                child: const Text('Close Shift & Print Z-Report'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
