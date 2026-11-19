import 'dart:ui';
import 'package:flutter/material.dart';

class PosColors {
  static const espresso = Color(0xFF0F172A);
  static const terracotta = Color(0xFFEA580C);
  static const emerald = Color(0xFF10B981);
  static const amber = Color(0xFFF59E0B);
  static const rose = Color(0xFFE11D48);

  // Daylight surfaces
  static const lightCanvas = Color(0xFFF8FAFC);
  static const lightCard = Color(0xFFFFFFFF);
  static const lightBorder = Color(0xFFE2E8F0);
  static const lightMuted = Color(0xFF64748B);

  // High-contrast dark surfaces
  static const darkCanvas = Color(0xFF09090B);
  static const darkCard = Color(0xFF121215);
  static const darkBorder = Color(0xFF27272A);
  static const darkMuted = Color(0xFFA1A1AA);
}

class PosTheme {
  static const TextStyle tabularPrice = TextStyle(
    fontFeatures: [FontFeature.tabularFigures()],
    fontWeight: FontWeight.w700,
  );

  static ThemeData light() {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.light,
      scaffoldBackgroundColor: PosColors.lightCanvas,
      colorScheme: const ColorScheme.light(
        primary: PosColors.terracotta,
        secondary: PosColors.espresso,
        surface: PosColors.lightCard,
        error: PosColors.rose,
      ),
      cardTheme: CardThemeData(
        color: PosColors.lightCard,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: const BorderSide(color: PosColors.lightBorder),
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          minimumSize: const Size(88, 52),
          backgroundColor: PosColors.terracotta,
          foregroundColor: Colors.white,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          textStyle: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(88, 52),
          side: const BorderSide(color: PosColors.lightBorder),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        ),
      ),
    );
  }

  static ThemeData dark() {
    return ThemeData(
      useMaterial3: true,
      brightness: Brightness.dark,
      scaffoldBackgroundColor: PosColors.darkCanvas,
      colorScheme: const ColorScheme.dark(
        primary: PosColors.terracotta,
        secondary: Colors.white,
        surface: PosColors.darkCard,
        error: PosColors.rose,
      ),
      cardTheme: CardThemeData(
        color: PosColors.darkCard,
        elevation: 0,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
          side: const BorderSide(color: PosColors.darkBorder),
        ),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          minimumSize: const Size(88, 52),
          backgroundColor: PosColors.terracotta,
          foregroundColor: Colors.white,
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
          textStyle: const TextStyle(fontWeight: FontWeight.w600, fontSize: 14),
        ),
      ),
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size(88, 52),
          side: const BorderSide(color: PosColors.darkBorder),
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
        ),
      ),
    );
  }
}
