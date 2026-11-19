import 'dart:ui';
import 'package:flutter/material.dart';

class StaffColors {
  static const espresso = Color(0xFF0F172A);
  static const terracotta = Color(0xFFEA580C);
  static const emerald = Color(0xFF10B981);
  static const amber = Color(0xFFF59E0B);
  static const rose = Color(0xFFE11D48);

  static const lightCanvas = Color(0xFFF8FAFC);
  static const lightCard = Color(0xFFFFFFFF);
  static const lightBorder = Color(0xFFE2E8F0);
  static const lightMuted = Color(0xFF64748B);

  static const darkCanvas = Color(0xFF09090B);
  static const darkCard = Color(0xFF121215);
  static const darkBorder = Color(0xFF27272A);
  static const darkMuted = Color(0xFFA1A1AA);
}

class StaffTheme {
  static const TextStyle tabularPrice = TextStyle(
    fontFeatures: [FontFeature.tabularFigures()],
    fontWeight: FontWeight.w700,
  );

  static ThemeData light() {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      scaffoldBackgroundColor: StaffColors.lightCanvas,
      colorScheme: const ColorScheme.light(
        primary: StaffColors.terracotta,
        secondary: StaffColors.espresso,
        surface: StaffColors.lightCard,
        error: StaffColors.rose,
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          minimumSize: const Size(88, 54),
          backgroundColor: StaffColors.terracotta,
          foregroundColor: Colors.white,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(88, 52),
          side: const BorderSide(color: StaffColors.lightBorder),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        ),
      ),
    );
  }

  static ThemeData dark() {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: StaffColors.darkCanvas,
      colorScheme: const ColorScheme.dark(
        primary: StaffColors.terracotta,
        secondary: Colors.white,
        surface: StaffColors.darkCard,
        error: StaffColors.rose,
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          minimumSize: const Size(88, 54),
          backgroundColor: StaffColors.terracotta,
          foregroundColor: Colors.white,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
          textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(88, 52),
          side: const BorderSide(color: StaffColors.darkBorder),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
        ),
      ),
    );
  }
}
