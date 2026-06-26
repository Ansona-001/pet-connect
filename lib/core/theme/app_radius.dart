import 'package:flutter/material.dart';

/// Centralized border radius values.
///
/// Never hardcode:
///
///   BorderRadius.circular(16)
///
/// Instead use:
///
///   AppRadius.lg
///
class AppRadius {
  AppRadius._();

  /// 8px
  static const BorderRadius xs = BorderRadius.all(Radius.circular(8));

  /// 12px
  static const BorderRadius sm = BorderRadius.all(Radius.circular(12));

  /// 16px
  static const BorderRadius md = BorderRadius.all(Radius.circular(16));

  /// 20px
  static const BorderRadius lg = BorderRadius.all(Radius.circular(20));

  /// 24px
  static const BorderRadius xl = BorderRadius.all(Radius.circular(24));

  /// 28px
  static const BorderRadius xxl = BorderRadius.all(Radius.circular(28));

  /// 32px
  static const BorderRadius pill = BorderRadius.all(Radius.circular(32));

  /// Circular widgets (avatars, profile images, etc.)
  static const BorderRadius circular = BorderRadius.all(Radius.circular(999));
}
