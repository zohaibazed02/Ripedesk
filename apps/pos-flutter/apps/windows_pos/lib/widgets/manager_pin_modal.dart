import 'package:flutter/material.dart';
import '../theme/pos_theme.dart';

class ManagerOverrideApproval {
  const ManagerOverrideApproval({
    required this.approverPin,
    required this.reasonCode,
    required this.approvedAt,
  });

  final String approverPin;
  final String reasonCode;
  final DateTime approvedAt;
}

/// Fast 4-digit Manager PIN Override modal with bounded offline validation and reason code capture.
class ManagerPinModal extends StatefulWidget {
  const ManagerPinModal({
    super.key,
    required this.actionLabel,
    this.validPins = const {'1111', '2222'},
  });

  final String actionLabel;
  final Set<String> validPins;

  @override
  State<ManagerPinModal> createState() => _ManagerPinModalState();
}

class _ManagerPinModalState extends State<ManagerPinModal> {
  String _pin = '';
  String _reasonCode = 'CUSTOMER_REQUEST';
  String? _error;

  static const _reasons = [
    'CUSTOMER_REQUEST',
    'QUALITY_ISSUE',
    'WRONG_ORDER_ENTERED',
    'PETTY_CASH_PAYOUT',
    'NO_SALE_DRAWER',
  ];

  void _tapDigit(String d) {
    setState(() {
      _error = null;
      if (d == 'C') {
        _pin = '';
      } else if (d == '⌫') {
        if (_pin.isNotEmpty) _pin = _pin.substring(0, _pin.length - 1);
      } else if (_pin.length < 4) {
        _pin += d;
      }
    });
  }

  void _submit() {
    if (!widget.validPins.contains(_pin)) {
      setState(() {
        _error = 'Invalid Manager PIN — override denied';
        _pin = '';
      });
      return;
    }
    Navigator.of(context).pop(ManagerOverrideApproval(
      approverPin: _pin,
      reasonCode: _reasonCode,
      approvedAt: DateTime.now().toUtc(),
    ));
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 380),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Manager Override Required',
                style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
              ),
              const SizedBox(height: 4),
              Text(
                widget.actionLabel,
                style: const TextStyle(fontSize: 12, color: PosColors.lightMuted),
              ),
              const SizedBox(height: 14),
              DropdownButtonFormField<String>(
                value: _reasonCode,
                decoration: const InputDecoration(
                  labelText: 'Audit Reason Code',
                  border: OutlineInputBorder(),
                  isDense: true,
                ),
                items: _reasons
                    .map((r) => DropdownMenuItem(value: r, child: Text(r)))
                    .toList(),
                onChanged: (v) => setState(() => _reasonCode = v ?? _reasonCode),
              ),
              const SizedBox(height: 16),

              // 4-dot PIN indicator
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: List.generate(4, (i) {
                  final filled = i < _pin.length;
                  return Container(
                    margin: const EdgeInsets.symmetric(horizontal: 8),
                    width: 16,
                    height: 16,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: filled ? PosColors.terracotta : Colors.transparent,
                      border: Border.all(color: PosColors.terracotta, width: 2),
                    ),
                  );
                }),
              ),
              if (_error != null) ...[
                const SizedBox(height: 8),
                Text(
                  _error!,
                  textAlign: TextAlign.center,
                  style: const TextStyle(fontSize: 12, color: PosColors.rose, fontWeight: FontWeight.w600),
                ),
              ],
              const SizedBox(height: 16),
              GridView.count(
                crossAxisCount: 3,
                shrinkWrap: true,
                mainAxisSpacing: 8,
                crossAxisSpacing: 8,
                childAspectRatio: 2.1,
                physics: const NeverScrollableScrollPhysics(),
                children: ['1', '2', '3', '4', '5', '6', '7', '8', '9', 'C', '0', '⌫']
                    .map((k) => OutlinedButton(
                          onPressed: () => _tapDigit(k),
                          child: Text(k, style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700)),
                        ))
                    .toList(),
              ),
              const SizedBox(height: 14),
              ElevatedButton(
                onPressed: _pin.length == 4 ? _submit : null,
                child: const Text('Authorize Override'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
