import 'package:flutter/material.dart';
import '../theme/pos_theme.dart';
import 'manager_pin_modal.dart';

class VoidRefundDecision {
  const VoidRefundDecision({
    required this.actionType,
    required this.preserveInventoryUsage,
    required this.recordWaste,
    required this.approval,
  });

  final String actionType; // void | refund
  final bool preserveInventoryUsage;
  final bool recordWaste;
  final ManagerOverrideApproval approval;
}

/// Pre-prep void vs Post-prep refund dialog with inventory/waste controls and Manager PIN gate.
class VoidRefundModal extends StatefulWidget {
  const VoidRefundModal({
    super.key,
    required this.orderNumber,
    required this.alreadyInPrep,
  });

  final String orderNumber;
  final bool alreadyInPrep;

  @override
  State<VoidRefundModal> createState() => _VoidRefundModalState();
}

class _VoidRefundModalState extends State<VoidRefundModal> {
  String _actionType = 'refund';
  bool _recordWaste = true;

  Future<void> _authorizeAndSubmit() async {
    final approval = await showDialog<ManagerOverrideApproval>(
      context: context,
      builder: (_) => ManagerPinModal(
        actionLabel: '${_actionType.toUpperCase()} Order ${widget.orderNumber}',
      ),
    );
    if (approval != null && mounted) {
      Navigator.of(context).pop(VoidRefundDecision(
        actionType: _actionType,
        preserveInventoryUsage: widget.alreadyInPrep,
        recordWaste: _recordWaste,
        approval: approval,
      ));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Dialog(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Void / Refund · ${widget.orderNumber}',
                style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: ChoiceChip(
                      label: const Center(child: Text('Void Order')),
                      selected: _actionType == 'void',
                      onSelected: (_) => setState(() => _actionType = 'void'),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: ChoiceChip(
                      label: const Center(child: Text('Post-Prep Refund')),
                      selected: _actionType == 'refund',
                      onSelected: (_) => setState(() => _actionType = 'refund'),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              if (widget.alreadyInPrep)
                Container(
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: PosColors.amber.withValues(alpha: 0.14),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: const Text(
                    'Order already entered prep: ingredient deduction will be preserved (preserve_inventory_usage = true).',
                    style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
                  ),
                ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('Record as Kitchen Waste', style: TextStyle(fontSize: 13)),
                value: _recordWaste,
                onChanged: (v) => setState(() => _recordWaste = v),
              ),
              const SizedBox(height: 12),
              ElevatedButton(
                style: ElevatedButton.styleFrom(backgroundColor: PosColors.rose),
                onPressed: _authorizeAndSubmit,
                child: const Text('Continue to Manager PIN Override'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
