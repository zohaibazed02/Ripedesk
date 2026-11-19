import 'package:flutter/material.dart';
import 'controllers/rider_controller.dart';
import 'screens/rider_home_screen.dart';

void main() {
  runApp(RiderApp(controller: RiderController()));
}

class RiderApp extends StatelessWidget {
  const RiderApp({super.key, required this.controller});

  final RiderController controller;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: controller,
      builder: (_, __) => MaterialApp(
        title: 'Hospitality OS — Rider App',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(
          useMaterial3: true,
          colorScheme: const ColorScheme.light(
            primary: Color(0xFFEA580C),
            secondary: Color(0xFF0F172A),
          ),
        ),
        home: RiderHomeScreen(controller: controller),
      ),
    );
  }
}
