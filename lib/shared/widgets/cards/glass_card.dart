import 'dart:ui';

import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/theme/app_shadows.dart';

/// Frosted glass-style card.
///
/// Use this for overlays, onboarding panels, auth panels,
/// profile info blocks, and modal content.
class GlassCard extends StatelessWidget {
  const GlassCard({
    super.key,
    required this.child,
    this.padding = const EdgeInsets.all(20),
    this.borderRadius,
    this.blur = 18,
    this.backgroundOpacity = 0.08,
    this.borderOpacity = 0.12,
    this.showShadow = true,
  });

  final Widget child;
  final EdgeInsets padding;
  final BorderRadius? borderRadius;
  final double blur;
  final double backgroundOpacity;
  final double borderOpacity;
  final bool showShadow;

  @override
  Widget build(BuildContext context) {
    final radius = borderRadius ?? AppRadius.xl;

    return ClipRRect(
      borderRadius: radius,
      child: BackdropFilter(
        filter: ImageFilter.blur(sigmaX: blur, sigmaY: blur),
        child: Container(
          padding: padding,
          decoration: BoxDecoration(
            color: AppColors.white.withValues(alpha: backgroundOpacity),
            borderRadius: radius,
            border: Border.all(
              color: AppColors.white.withValues(alpha: borderOpacity),
            ),
            boxShadow: showShadow ? AppShadows.glass : AppShadows.none,
          ),
          child: child,
        ),
      ),
    );
  }
}
