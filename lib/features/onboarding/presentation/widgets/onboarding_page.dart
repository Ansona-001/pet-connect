import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../models/onboarding_item.dart';

class OnboardingPage extends StatelessWidget {
  const OnboardingPage({super.key, required this.item});

  final OnboardingItem item;

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        Positioned.fill(
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: item.gradientColors,
              ),
            ),
          ),
        ),

        Positioned.fill(
          child: DecoratedBox(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [
                  AppColors.black.withValues(alpha: 0.05),
                  AppColors.black.withValues(alpha: 0.45),
                  AppColors.black.withValues(alpha: 0.92),
                ],
              ),
            ),
          ),
        ),

        Center(
          child:
              Icon(
                    item.icon,
                    size: 140,
                    color: AppColors.white.withValues(alpha: 0.92),
                  )
                  .animate()
                  .fadeIn(duration: 500.ms)
                  .scale(
                    begin: const Offset(0.85, 0.85),
                    end: const Offset(1, 1),
                    duration: 500.ms,
                    curve: Curves.easeOutCubic,
                  ),
        ),

        Positioned(
          left: AppSpacing.xxl,
          right: AppSpacing.xxl,
          bottom: 150,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                item.title,
                style: AppTextStyles.displayMedium,
              ).animate().fadeIn(duration: 400.ms).slideY(begin: 0.12, end: 0),

              const SizedBox(height: AppSpacing.lg),

              Text(
                    item.subtitle,
                    style: AppTextStyles.bodyMedium.copyWith(
                      color: AppColors.textSecondary,
                    ),
                  )
                  .animate(delay: 120.ms)
                  .fadeIn(duration: 400.ms)
                  .slideY(begin: 0.12, end: 0),
            ],
          ),
        ),
      ],
    );
  }
}
