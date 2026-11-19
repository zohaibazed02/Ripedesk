import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../controllers/staff_app_controller.dart';
import '../theme/staff_theme.dart';

class StaffPinScreen extends StatefulWidget {
  const StaffPinScreen({super.key, required this.controller});

  final StaffAppController controller;

  @override
  State<StaffPinScreen> createState() => _StaffPinScreenState();
}

class _StaffPinScreenState extends State<StaffPinScreen> {
  String _pin = '';
  String? _error;

  void _tapKey(String k) {
    setState(() {
      _error = null;
      if (k == 'C') {
        _pin = '';
      } else if (k == '⌫') {
        if (_pin.isNotEmpty) _pin = _pin.substring(0, _pin.length - 1);
      } else if (_pin.length < 4) {
        _pin += k;
        if (_pin.length == 4) {
          final ok = widget.controller.loginWithPin(_pin);
          if (!ok) {
            _error = 'Invalid PIN (try 4444 Waiter or 2222 Manager)';
            _pin = '';
          }
        }
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final mode = widget.controller.connectionMode;
    final syncLabel = mode == ConnectionMode.cloudConnected
        ? 'Online • Synced'
        : (mode == ConnectionMode.lanOfflineInternet ? 'LAN Mode • Active' : 'Offline');
    final syncColor = mode == ConnectionMode.cloudConnected ? StaffColors.emerald : StaffColors.amber;

    return Scaffold(
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                decoration: BoxDecoration(
                  color: syncColor.withValues(alpha: 0.14),
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  syncLabel,
                  style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: syncColor),
                ),
              ),
              const SizedBox(height: 16),
              Text(
                widget.controller.branchName,
                style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w800),
              ),
              const SizedBox(height: 4),
              const Text(
                'Enter 4-Digit Server PIN to Unlock Handheld',
                style: TextStyle(fontSize: 13, color: StaffColors.lightMuted),
              ),
              const SizedBox(height: 24),
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: List.generate(4, (i) {
                  final filled = i < _pin.length;
                  return Container(
                    margin: const EdgeInsets.symmetric(horizontal: 10),
                    width: 18,
                    height: 18,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: filled ? StaffColors.terracotta : Colors.transparent,
                      border: Border.all(color: StaffColors.terracotta, width: 2),
                    ),
                  );
                }),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(
                  _error!,
                  style: const TextStyle(fontSize: 12, color: StaffColors.rose, fontWeight: FontWeight.w600),
                ),
              ],
              const SizedBox(height: 28),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 320),
                child: GridView.count(
                  crossAxisCount: 3,
                  shrinkWrap: true,
                  mainAxisSpacing: 12,
                  crossAxisSpacing: 12,
                  childAspectRatio: 1.5,
                  physics: const NeverScrollableScrollPhysics(),
                  children: ['1', '2', '3', '4', '5', '6', '7', '8', '9', 'C', '0', '⌫']
                      .map((k) => OutlinedButton(
                            onPressed: () => _tapKey(k),
                            child: Text(k, style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700)),
                          ))
                      .toList(),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
