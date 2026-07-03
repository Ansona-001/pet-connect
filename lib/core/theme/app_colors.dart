import 'package:flutter/material.dart';

/// Central color palette for PetConnect.
///
/// Do not use hardcoded colors anywhere else in the app.
/// Always reference colors from this class.
class AppColors {
  AppColors._();

  // ---------------------------------------------------------------------------
  // Brand
  // ---------------------------------------------------------------------------

  static const Color primary = Color(0xFF5B4BFF);
  static const Color primaryLight = Color(0xFF7A6DFF);
  static const Color primaryDark = Color(0xFF4533E6);

  static const Color accent = Color(0xFFFF7A59);
  static const Color accentLight = Color(0xFFFF977D);
  static const Color accentDark = Color(0xFFE55C37);

  // ---------------------------------------------------------------------------
  // Backgrounds
  // ---------------------------------------------------------------------------

  static const Color background = Color(0xFF0F1115);

  static const Color surface = Color(0xFF171A20);

  static const Color card = Color(0xFF1E222B);

  static const Color elevatedSurface = Color(0xFF242933);

  // ---------------------------------------------------------------------------
  // Text
  // ---------------------------------------------------------------------------

  static const Color textPrimary = Color(0xFFF5F7FA);

  static const Color textSecondary = Color(0xFFA7B0BE);

  static const Color textDisabled = Color(0xFF6E7684);

  // ---------------------------------------------------------------------------
  // Borders
  // ---------------------------------------------------------------------------

  static const Color border = Color(0xFF2B303A);

  static const Color divider = Color(0xFF323844);

  // ---------------------------------------------------------------------------
  // Status
  // ---------------------------------------------------------------------------

  static const Color success = Color(0xFF2ECC71);

  static const Color warning = Color(0xFFF4B740);

  static const Color error = Color(0xFFEF4444);

  static const Color info = Color(0xFF3BA4F6);

  // ---------------------------------------------------------------------------
  // Common
  // ---------------------------------------------------------------------------

  static const Color white = Colors.white;

  static const Color black = Colors.black;

  static const Color transparent = Colors.transparent;

  // ---------------------------------------------------------------------------
  // Glass Effect
  // ---------------------------------------------------------------------------

  static final Color glass = Colors.white.withValues(alpha: 0.06);

  static final Color glassBorder = Colors.white.withValues(alpha: 0.10);

  // ---------------------------------------------------------------------------
  // Overlays
  // ---------------------------------------------------------------------------

  static final Color overlay = Colors.black.withValues(alpha: 0.60);

  static final Color overlayLight = Colors.black.withValues(alpha: 0.30);

  static final Color overlayHeavy = Colors.black.withValues(alpha: 0.82);

  // ---------------------------------------------------------------------------
  // Gradients
  // ---------------------------------------------------------------------------

  static const LinearGradient primaryGradient = LinearGradient(
    begin: Alignment.topLeft,
    end: Alignment.bottomRight,
    colors: [primary, accent],
  );

  static const LinearGradient darkGradient = LinearGradient(
    begin: Alignment.topCenter,
    end: Alignment.bottomCenter,
    colors: [Color(0xFF171A20), Color(0xFF0F1115)],
  );

  static const LinearGradient onboardingOverlay = LinearGradient(
    begin: Alignment.topCenter,
    end: Alignment.bottomCenter,
    colors: [Colors.transparent, Color(0x66000000), Color(0xCC000000)],
  );
}
