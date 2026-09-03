import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/cards/glass_card.dart';
import '../models/onboarding_item.dart';

class OnboardingPage extends StatelessWidget {
  const OnboardingPage({super.key, required this.item});

  final OnboardingItem item;

  @override
  Widget build(BuildContext context) {
    final compactHeight = MediaQuery.sizeOf(context).height < 700;

    return Stack(
      children: [
        Positioned.fill(
          child: Image.asset(
            item.imageAsset,
            fit: BoxFit.cover,
            alignment: Alignment.topCenter,
            cacheWidth: 1440,
            errorBuilder: (context, error, stackTrace) => ColoredBox(
              color: AppColors.surface,
              child: Center(
                child: Icon(item.icon, size: 120, color: item.accentColor),
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
                  AppColors.black.withValues(alpha: 0.08),
                  AppColors.background.withValues(alpha: 0.30),
                  AppColors.background.withValues(alpha: 0.98),
                ],
                stops: const [0, 0.45, 0.84],
              ),
            ),
          ),
        ),

        Positioned(
          left: AppSpacing.xl,
          right: AppSpacing.xl,
          bottom: compactHeight ? 140 : 180,
          child: GlassCard(
            borderRadius: AppRadius.xxl,
            padding: EdgeInsets.all(
              compactHeight ? AppSpacing.lg : AppSpacing.xxl,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                CircleAvatar(
                  radius: 25,
                  backgroundColor: item.accentColor,
                  child: Icon(item.icon, color: AppColors.white, size: 26),
                ),
                SizedBox(
                  height: compactHeight ? AppSpacing.md : AppSpacing.xxl,
                ),
                RichText(
                  text: TextSpan(
                    style: AppTextStyles.displaySmall,
                    children: [
                      TextSpan(text: item.title),
                      TextSpan(
                        text: item.highlight,
                        style: AppTextStyles.displaySmall.copyWith(
                          color: item.accentColor,
                        ),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                Text(
                  item.subtitle,
                  style: AppTextStyles.bodyMedium.copyWith(
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ).animate().fadeIn(duration: 420.ms).slideY(begin: 0.12, end: 0),
        ),
      ],
    );
  }
}
