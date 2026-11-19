import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import 'controllers/dine_in_controller.dart';
import 'controllers/pos_controller.dart';
import 'screens/quick_sale_screen.dart';
import 'screens/table_map_screen.dart';
import 'theme/pos_theme.dart';

void main() {
  final lanClient = BranchLanClient(
    branchBaseUrl: 'http://127.0.0.1:8090',
    isolatedModeEnabled: true,
  );
  final printerAdapter = BranchServiceSpoolerAdapter(
    branchBaseUrl: 'http://127.0.0.1:8090',
  );
  final posController = PosController(
    lanClient: lanClient,
    printerAdapter: printerAdapter,
  )..startHealthPolling();
  final dineInController = DineInController(
    lanClient: lanClient,
    printerAdapter: printerAdapter,
  );

  runApp(WindowsPosApp(
    controller: posController,
    dineInController: dineInController,
  ));
}

class WindowsPosApp extends StatelessWidget {
  const WindowsPosApp({
    super.key,
    required this.controller,
    required this.dineInController,
  });

  final PosController controller;
  final DineInController dineInController;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: controller,
      builder: (_, __) {
        return MaterialApp(
          title: 'Hospitality OS — Windows POS',
          debugShowCheckedModeBanner: false,
          theme: PosTheme.light(),
          darkTheme: PosTheme.dark(),
          themeMode: controller.isDarkMode ? ThemeMode.dark : ThemeMode.light,
          home: controller.orderMode == OrderMode.dineIn
              ? TableMapScreen(
                  posController: controller,
                  dineInController: dineInController,
                )
              : QuickSaleScreen(controller: controller),
        );
      },
    );
  }
}
