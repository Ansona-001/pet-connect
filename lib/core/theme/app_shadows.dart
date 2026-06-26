import 'package:flutter/material.dart';

/// Centralized shadow definitions.
///
/// Never write BoxShadow directly inside widgets.
/// Always use one of the predefined shadows from this class.
///
/// Example:
///
/// BoxDecoration(
///   color: AppColors.card,
///   borderRadius: AppRadius.xl,
///   boxShadow: AppShadows.card,
/// )
class AppShadows {
  AppShadows._();

  /// No shadow
  static const List<BoxShadow> none = [];

  /// Small shadow
  static const List<BoxShadow> sm = [
    BoxShadow(color: Color(0x14000000), blurRadius: 6, offset: Offset(0, 2)),
  ];

  /// Medium shadow
  static const List<BoxShadow> md = [
    BoxShadow(color: Color(0x1A000000), blurRadius: 12, offset: Offset(0, 6)),
  ];

  /// Large shadow
  static const List<BoxShadow> lg = [
    BoxShadow(color: Color(0x26000000), blurRadius: 24, offset: Offset(0, 12)),
  ];

  /// Card shadow
  static const List<BoxShadow> card = [
    BoxShadow(
      color: Color(0x22000000),
      blurRadius: 18,
      spreadRadius: 0,
      offset: Offset(0, 8),
    ),
  ];

  /// Floating button shadow
  static const List<BoxShadow> floating = [
    BoxShadow(
      color: Color(0x33000000),
      blurRadius: 30,
      spreadRadius: 0,
      offset: Offset(0, 12),
    ),
  ];

  /// Primary glow (used for CTA buttons)
  static const List<BoxShadow> primaryGlow = [
    BoxShadow(
      color: Color(0x667C3AED),
      blurRadius: 24,
      spreadRadius: 2,
      offset: Offset(0, 0),
    ),
  ];

  /// Accent glow (used for likes, matches, etc.)
  static const List<BoxShadow> accentGlow = [
    BoxShadow(
      color: Color(0x66FF4D8D),
      blurRadius: 24,
      spreadRadius: 2,
      offset: Offset(0, 0),
    ),
  ];

  /// Glassmorphism-style shadow
  static const List<BoxShadow> glass = [
    BoxShadow(color: Color(0x1AFFFFFF), blurRadius: 8, offset: Offset(0, 1)),
    BoxShadow(color: Color(0x33000000), blurRadius: 24, offset: Offset(0, 12)),
  ];

  /// Creates a colored glow dynamically.
  static List<BoxShadow> glow(
    Color color, {
    double blur = 24,
    double spread = 2,
  }) {
    return [
      BoxShadow(
        color: color.withValues(alpha: 0.45),
        blurRadius: blur,
        spreadRadius: spread,
        offset: const Offset(0, 0),
      ),
    ];
  }

  /// Creates a subtle colored elevation.
  static List<BoxShadow> colored(
    Color color, {
    double opacity = 0.20,
    double blur = 20,
    double offsetY = 8,
  }) {
    return [
      BoxShadow(
        color: color.withValues(alpha: opacity),
        blurRadius: blur,
        offset: Offset(0, offsetY),
      ),
    ];
  }
}
