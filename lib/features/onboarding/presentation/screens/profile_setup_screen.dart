import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/inputs/app_text_field.dart';
import '../../../auth/application/auth_controller.dart';
import '../../../social/application/social_controller.dart';

class ProfileSetupScreen extends ConsumerStatefulWidget {
  const ProfileSetupScreen({super.key});

  @override
  ConsumerState<ProfileSetupScreen> createState() => _ProfileSetupScreenState();
}

class _ProfileSetupScreenState extends ConsumerState<ProfileSetupScreen> {
  final _pageController = PageController();
  final _nameController = TextEditingController(text: 'Alex');
  final _cityController = TextEditingController(text: 'Dubai');
  final _petNameController = TextEditingController(text: 'Luna');

  int _step = 0;
  String _petType = 'Dog';
  final Set<String> _interests = {'Playdates', 'Parks', 'Training'};

  static const _petTypes = ['Dog', 'Cat', 'Bird', 'Rabbit', 'Other'];
  static const _interestOptions = [
    'Playdates',
    'Parks',
    'Training',
    'Adoption',
    'Pet-friendly cafés',
    'Health & wellness',
  ];

  @override
  void dispose() {
    _pageController.dispose();
    _nameController.dispose();
    _cityController.dispose();
    _petNameController.dispose();
    super.dispose();
  }

  Future<void> _next() async {
    if (_step == 2) {
      final completed = await ref
          .read(authControllerProvider.notifier)
          .completeOnboarding(
            ownerName: _nameController.text,
            city: _cityController.text,
            petName: _petNameController.text,
            petType: _petType,
            interests: _interests.toList(growable: false),
          );
      if (!mounted || !completed) return;
      ref
          .read(socialControllerProvider.notifier)
          .completeSetup(
            ownerName: _nameController.text,
            city: _cityController.text,
            petName: _petNameController.text,
            petType: _petType,
            interests: _interests.toList(growable: false),
          );
      context.go(AppRoutes.home);
      return;
    }
    _pageController.nextPage(
      duration: const Duration(milliseconds: 320),
      curve: Curves.easeOutCubic,
    );
  }

  void _back() {
    if (_step == 0) {
      context.go(AppRoutes.login);
      return;
    }
    _pageController.previousPage(
      duration: const Duration(milliseconds: 320),
      curve: Curves.easeOutCubic,
    );
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    return Scaffold(
      body: SafeArea(
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 560),
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(
                    AppSpacing.sm,
                    AppSpacing.sm,
                    AppSpacing.xl,
                    0,
                  ),
                  child: Row(
                    children: [
                      IconButton(
                        tooltip: 'Back',
                        onPressed: _back,
                        icon: const Icon(Icons.arrow_back_rounded),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: ClipRRect(
                          borderRadius: AppRadius.pill,
                          child: LinearProgressIndicator(
                            minHeight: 7,
                            value: (_step + 1) / 3,
                            backgroundColor: AppColors.card,
                          ),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Text('${_step + 1}/3', style: AppTextStyles.labelMedium),
                    ],
                  ),
                ),
                Expanded(
                  child: PageView(
                    controller: _pageController,
                    physics: const NeverScrollableScrollPhysics(),
                    onPageChanged: (value) => setState(() => _step = value),
                    children: [
                      _SetupPage(
                        eyebrow: 'WELCOME TO PETCONNECT',
                        title: 'Let’s start with you.',
                        subtitle:
                            'This helps nearby pet parents know who they’re connecting with.',
                        child: Column(
                          children: [
                            AppTextField(
                              controller: _nameController,
                              label: 'Your name',
                              prefix: const Icon(
                                Icons.person_outline_rounded,
                                color: AppColors.textSecondary,
                              ),
                            ),
                            const SizedBox(height: AppSpacing.lg),
                            AppTextField(
                              controller: _cityController,
                              label: 'City',
                              prefix: const Icon(
                                Icons.location_on_outlined,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ],
                        ),
                      ),
                      _SetupPage(
                        eyebrow: 'YOUR PET',
                        title: 'Who’s joining the pack?',
                        subtitle:
                            'Every pet gets their own social identity. You can add more later.',
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            AppTextField(
                              controller: _petNameController,
                              label: 'Pet name',
                              prefix: const Icon(
                                Icons.pets_outlined,
                                color: AppColors.textSecondary,
                              ),
                            ),
                            const SizedBox(height: AppSpacing.xxl),
                            Text('Pet type', style: AppTextStyles.labelLarge),
                            const SizedBox(height: AppSpacing.md),
                            Wrap(
                              spacing: AppSpacing.sm,
                              runSpacing: AppSpacing.sm,
                              children: _petTypes
                                  .map((type) {
                                    return _ChoiceChip(
                                      label: type,
                                      selected: _petType == type,
                                      onSelected: () =>
                                          setState(() => _petType = type),
                                    );
                                  })
                                  .toList(growable: false),
                            ),
                          ],
                        ),
                      ),
                      _SetupPage(
                        eyebrow: 'PERSONALIZE YOUR WORLD',
                        title: 'What are you here for?',
                        subtitle:
                            'Choose a few interests so your feed and discoveries feel relevant.',
                        child: Wrap(
                          spacing: AppSpacing.sm,
                          runSpacing: AppSpacing.sm,
                          children: _interestOptions
                              .map((interest) {
                                return _ChoiceChip(
                                  label: interest,
                                  selected: _interests.contains(interest),
                                  onSelected: () {
                                    setState(() {
                                      if (!_interests.add(interest)) {
                                        _interests.remove(interest);
                                      }
                                    });
                                  },
                                );
                              })
                              .toList(growable: false),
                        ),
                      ),
                    ],
                  ),
                ),
                Padding(
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: Column(
                    children: [
                      if (authState.error != null) ...[
                        Text(
                          authState.error!,
                          textAlign: TextAlign.center,
                          style: AppTextStyles.bodySmall.copyWith(
                            color: AppColors.error,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                      ],
                      PrimaryButton(
                        text: authState.isLoading
                            ? 'Saving...'
                            : _step == 2
                            ? 'Enter PetConnect'
                            : 'Continue',
                        icon: _step == 2
                            ? Icons.pets_rounded
                            : Icons.arrow_forward_rounded,
                        enabled:
                            !authState.isLoading &&
                            (_step != 2 || _interests.isNotEmpty),
                        onPressed:
                            !authState.isLoading &&
                                (_step != 2 || _interests.isNotEmpty)
                            ? _next
                            : null,
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _SetupPage extends StatelessWidget {
  const _SetupPage({
    required this.eyebrow,
    required this.title,
    required this.subtitle,
    required this.child,
  });

  final String eyebrow;
  final String title;
  final String subtitle;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.xl,
        AppSpacing.huge,
        AppSpacing.xl,
        AppSpacing.xxl,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 64,
            height: 64,
            decoration: const BoxDecoration(
              color: AppColors.primary,
              borderRadius: AppRadius.lg,
            ),
            child: const Icon(Icons.pets_rounded, size: 32),
          ),
          const SizedBox(height: AppSpacing.xxl),
          Text(
            eyebrow,
            style: AppTextStyles.overline.copyWith(
              color: AppColors.primaryLight,
            ),
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(title, style: AppTextStyles.displaySmall),
          const SizedBox(height: AppSpacing.md),
          Text(subtitle, style: AppTextStyles.bodyMedium),
          const SizedBox(height: AppSpacing.xxxl),
          child,
        ],
      ),
    );
  }
}

class _ChoiceChip extends StatelessWidget {
  const _ChoiceChip({
    required this.label,
    required this.selected,
    required this.onSelected,
  });

  final String label;
  final bool selected;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      borderRadius: AppRadius.pill,
      onTap: onSelected,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.lg,
          vertical: AppSpacing.md,
        ),
        decoration: BoxDecoration(
          color: selected
              ? AppColors.primary.withValues(alpha: 0.18)
              : AppColors.surface,
          borderRadius: AppRadius.pill,
          border: Border.all(
            color: selected ? AppColors.primary : AppColors.border,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (selected) ...[
              const Icon(
                Icons.check_rounded,
                size: 16,
                color: AppColors.primaryLight,
              ),
              const SizedBox(width: AppSpacing.sm),
            ],
            Text(label, style: AppTextStyles.labelMedium),
          ],
        ),
      ),
    );
  }
}
