import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../../../core/routes/routes.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/buttons/secondary_button.dart';
import '../models/onboarding_item.dart';
import '../widgets/onboarding_page.dart';
import '../widgets/page_indicator.dart';

class OnboardingScreen extends StatefulWidget {
  const OnboardingScreen({super.key});

  @override
  State<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends State<OnboardingScreen> {
  final PageController _pageController = PageController();

  int _currentIndex = 0;

  static const List<OnboardingItem> _items = [
    OnboardingItem(
      title: 'A better world for\n ',
      highlight: 'pets and their people.',
      subtitle: 'Discover, connect, and share life with pet lovers near you.',
      icon: Icons.pets_rounded,
      accentColor: AppColors.primary,
      imageAsset: 'assets/images/onboarding/onboarding_1.jpg',
    ),
    OnboardingItem(
      title: 'Discover amazing\n',
      highlight: 'pet lovers nearby.',
      subtitle:
          'Find new friends, connect with pet parents, and grow your pet community.',
      icon: Icons.groups_rounded,
      accentColor: AppColors.success,
      imageAsset: 'assets/images/content/pet_playdate.png',
    ),
    OnboardingItem(
      title: 'Share moments\n',
      highlight: 'they’ll never forget.',
      subtitle:
          'Post updates, photos, and stories that celebrate your pet’s everyday adventures.',
      icon: Icons.star_rounded,
      accentColor: AppColors.accent,
      imageAsset: 'assets/images/onboarding/onboarding_3.jpg',
    ),
  ];

  bool get _isLastPage => _currentIndex == _items.length - 1;

  void _next() {
    if (_isLastPage) {
      context.go(AppRoutes.login);
      return;
    }

    _pageController.nextPage(
      duration: const Duration(milliseconds: 320),
      curve: Curves.easeOutCubic,
    );
  }

  void _skip() {
    context.go(AppRoutes.login);
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final activeColor = _items[_currentIndex].accentColor;

    return Scaffold(
      backgroundColor: AppColors.background,
      body: Stack(
        children: [
          PageView.builder(
            controller: _pageController,
            itemCount: _items.length,
            onPageChanged: (index) {
              setState(() => _currentIndex = index);
            },
            itemBuilder: (context, index) {
              return OnboardingPage(item: _items[index]);
            },
          ),

          SafeArea(
            child: Align(
              alignment: Alignment.topRight,
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: SecondaryButton(
                  text: 'Skip',
                  width: 86,
                  height: 42,
                  onPressed: _skip,
                ),
              ),
            ),
          ),

          Positioned(
            left: AppSpacing.xl,
            right: AppSpacing.xl,
            bottom: 0,
            child: SafeArea(
              top: false,
              minimum: const EdgeInsets.only(bottom: AppSpacing.xxl),
              child: Column(
                children: [
                  PageIndicator(
                    length: _items.length,
                    currentIndex: _currentIndex,
                    activeColor: activeColor,
                  ),
                  const SizedBox(height: AppSpacing.xxl),
                  PrimaryButton(
                    text: _isLastPage ? 'Get Started' : 'Next',
                    icon: Icons.arrow_forward_rounded,
                    onPressed: _next,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
