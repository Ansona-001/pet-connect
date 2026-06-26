import 'package:flutter/material.dart';

/// Centralized application color palette.
///
/// Never use Colors.blue, Colors.red, etc. directly in the UI.
/// Always reference colors from this class.
class AppColors {
  AppColors._();

  // ---------------------------------------------------------------------------
  // Brand Colors
  // ---------------------------------------------------------------------------

  static const Color primary = Color(0xFF7C3AED);
  static const Color secondary = Color(0xFFA855F7);
  static const Color accent = Color(0xFFFF4D8D);

  // ---------------------------------------------------------------------------
  // Background
  // ---------------------------------------------------------------------------

  static const Color background = Color(0xFF0B0B0F);
  static const Color surface = Color(0xFF18181C);
  static const Color card = Color(0xFF222228);

  // ---------------------------------------------------------------------------
  // Text
  // ---------------------------------------------------------------------------

  static const Color textPrimary = Color(0xFFFFFFFF);

  static const Color textSecondary = Color(0xFFB8B8C2);

  static const Color textHint = Color(0xFF6D6D78);

  // ---------------------------------------------------------------------------
  // Status
  // ---------------------------------------------------------------------------

  static const Color success = Color(0xFF00D68F);

  static const Color warning = Color(0xFFF59E0B);

  static const Color error = Color(0xFFFF5A5F);

  static const Color info = Color(0xFF38BDF8);

  // ---------------------------------------------------------------------------
  // Border
  // ---------------------------------------------------------------------------

  static const Color border = Color(0xFF2D2D33);

  static const Color divider = Color(0xFF2D2D33);

  // ---------------------------------------------------------------------------
  // Utility
  // ---------------------------------------------------------------------------

  static const Color white = Colors.white;

  static const Color black = Colors.black;

  static const Color transparent = Colors.transparent;

  // ---------------------------------------------------------------------------
  // Gradients
  // ---------------------------------------------------------------------------

  static const LinearGradient primaryGradient = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [primary, secondary],
  );

  static const LinearGradient accentGradient = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [accent, primary],
  );

  static const LinearGradient surfaceGradient = LinearGradient(
    begin: Alignment.topCenter,
    end: Alignment.bottomCenter,
    colors: [Color(0xFF24242A), Color(0xFF18181C)],
  );
}
