import 'package:flutter/material.dart';
import 'package:pos_domain/pos_domain.dart';
import 'controllers/staff_app_controller.dart';
import 'screens/staff_pin_screen.dart';
import 'screens/staff_tables_screen.dart';
import 'theme/staff_theme.dart';

const _defaultBranchLanUrl = String.fromEnvironment(
  'BRANCH_LAN_URL',
  defaultValue: 'http://192.168.1.10:8090',
);

void main() {
  final lanClient = BranchLanClient(branchBaseUrl: _defaultBranchLanUrl);
  final controller = StaffAppController(lanClient: lanClient)..startHealthPolling();
  runApp(AndroidStaffApp(controller: controller));
}

class AndroidStaffApp extends StatelessWidget {
  const AndroidStaffApp({super.key, required this.controller});

  final StaffAppController controller;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: controller,
      builder: (_, __) {
        final loggedIn = controller.activeSession != null &&
            controller.activeSession!.isValidAt(DateTime.now().toUtc());
        return MaterialApp(
          title: 'Hospitality OS — Staff Handheld',
          debugShowCheckedModeBanner: false,
          theme: StaffTheme.light(),
          darkTheme: StaffTheme.dark(),
          themeMode: controller.isDarkMode ? ThemeMode.dark : ThemeMode.light,
          home: loggedIn
              ? StaffTablesScreen(controller: controller)
              : StaffPinScreen(controller: controller),
        );
      },
    );
  }
}
