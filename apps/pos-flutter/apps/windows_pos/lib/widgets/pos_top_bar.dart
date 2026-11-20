import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import '../controllers/pos_controller.dart';
import '../data/pilot_catalog.dart';
import '../theme/pos_theme.dart';

class PosTopBar extends StatelessWidget {
  const PosTopBar({
    super.key,
    required this.controller,
    required this.onKickDrawer,
    required this.onReprintLast,
  });

  final PosController controller;
  final VoidCallback onKickDrawer;
  final VoidCallback onReprintLast;

  static const _modeMeta = <(OrderMode, String, String, Color)>[
    (OrderMode.quickSale, 'Quick Sale', 'F1', PosColors.terracotta),
    (OrderMode.takeaway, 'Takeaway', 'F2', PosColors.amber),
    (OrderMode.dineIn, 'Dine-In', 'F3', PosColors.emerald),
    (OrderMode.delivery, 'Delivery', 'F4', Color(0xFF0284C7)),
    (OrderMode.qrOrder, 'QR Order', 'F5', Color(0xFF7C3AED)),
    (OrderMode.driveThru, 'Drive-Thru', 'F6', PosColors.rose),
  ];

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final borderColor = isDark ? PosColors.darkBorder : PosColors.lightBorder;
    final surfaceColor = isDark ? PosColors.darkCard : PosColors.lightCard;

    return Container(
      height: 64,
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: surfaceColor,
        border: Border(bottom: BorderSide(color: borderColor)),
      ),
      child: Row(
        children: [
          // Branch & Shift Pill (tap to switch F-7 Coffee Bar <-> G-9 Flagship Dine-In)
          InkWell(
            onTap: () {
              final next = controller.branch.branchId == PilotCatalog.defaultBranch.branchId
                  ? PilotCatalog.dineInBranch
                  : PilotCatalog.defaultBranch;
              controller.switchBranch(next);
            },
            borderRadius: BorderRadius.circular(10),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: borderColor),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(Icons.storefront, size: 18, color: PosColors.terracotta),
                  const SizedBox(width: 8),
                  Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        controller.branch.branchName,
                        style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w700),
                      ),
                      Text(
                        '${controller.activeCashierName} · Shift ${controller.shiftOpen ? "Open" : "Closed"}',
                        style: TextStyle(
                          fontSize: 11,
                          color: isDark ? PosColors.darkMuted : PosColors.lightMuted,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
          const SizedBox(width: 12),

          // 6-Mode Selector Tabs (F1-F6)
          Expanded(
            child: SingleChildScrollView(
              scrollDirection: Axis.horizontal,
              child: Row(
                children: _modeMeta.map((entry) {
                  final (mode, label, keyHint, accent) = entry;
                  final active = controller.orderMode == mode;
                  return Padding(
                    padding: const EdgeInsets.only(right: 6),
                    child: InkWell(
                      onTap: () => controller.setOrderMode(mode),
                      borderRadius: BorderRadius.circular(8),
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                        decoration: BoxDecoration(
                          color: active ? accent : Colors.transparent,
                          borderRadius: BorderRadius.circular(8),
                          border: Border.all(color: active ? accent : borderColor),
                        ),
                        child: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(
                              label,
                              style: TextStyle(
                                fontSize: 12,
                                fontWeight: FontWeight.w600,
                                color: active ? Colors.white : null,
                              ),
                            ),
                            const SizedBox(width: 5),
                            Text(
                              keyHint,
                              style: TextStyle(
                                fontSize: 10,
                                fontWeight: FontWeight.w600,
                                color: active
                                    ? Colors.white70
                                    : (isDark ? PosColors.darkMuted : PosColors.lightMuted),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                  );
                }).toList(),
              ),
            ),
          ),
          const SizedBox(width: 12),

          // Live Sync Status Badge
          ListenableBuilder(
            listenable: controller,
            builder: (_, __) => _SyncPill(
              mode: controller.connectionMode,
              pendingCount: controller.pendingOutboxCount,
              latencyMs: controller.lanLatencyMs,
            ),
          ),
          const SizedBox(width: 10),

          // Hardware & Theme Actions
          Tooltip(
            message: 'Open Cash Drawer (Ctrl+D)',
            child: IconButton(
              onPressed: onKickDrawer,
              icon: const Icon(Icons.point_of_sale, size: 20),
            ),
          ),
          Tooltip(
            message: 'Reprint Last Receipt (Ctrl+P)',
            child: IconButton(
              onPressed: onReprintLast,
              icon: const Icon(Icons.print_outlined, size: 20),
            ),
          ),
          Tooltip(
            message: 'Toggle Daylight / Dark Mode',
            child: IconButton(
              onPressed: controller.toggleTheme,
              icon: Icon(
                controller.isDarkMode ? Icons.light_mode : Icons.dark_mode,
                size: 20,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _SyncPill extends StatelessWidget {
  const _SyncPill({
    required this.mode,
    required this.pendingCount,
    required this.latencyMs,
  });

  final ConnectionMode mode;
  final int pendingCount;
  final int latencyMs;

  @override
  Widget build(BuildContext context) {
    final (label, color) = switch (mode) {
      ConnectionMode.cloudConnected => ('Online · Cloud Synced', PosColors.emerald),
      ConnectionMode.lanOfflineInternet => ('LAN Mode · Offline', PosColors.amber),
      ConnectionMode.isolatedTerminal => ('Offline · LAN Unreachable', PosColors.rose),
    };
    final suffix = mode == ConnectionMode.isolatedTerminal
        ? (pendingCount > 0 ? ' · $pendingCount queued' : '')
        : ' · ${latencyMs}ms${pendingCount > 0 ? " · $pendingCount queued" : ""}';

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: color.withValues(alpha: 0.35)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 6),
          Text(
            '$label$suffix',
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}
