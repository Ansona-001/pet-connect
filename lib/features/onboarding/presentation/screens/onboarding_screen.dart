import 'package:flutter/material.dart';

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
      title: 'Meet amazing pets near you.',
      subtitle:
          'Discover playful companions, local pet lovers, and new friends for your furry family.',
      icon: Icons.pets_rounded,
      gradientColors: [Color(0xFF7C3AED), Color(0xFFFF4D8D)],
    ),
    OnboardingItem(
      title: 'Swipe. Match. Connect.',
      subtitle:
          'Find compatible pets by personality, lifestyle, interests, and location.',
      icon: Icons.favorite_rounded,
      gradientColors: [Color(0xFFFF4D8D), Color(0xFFFB7185)],
    ),
    OnboardingItem(
      title: 'Share moments that matter.',
      subtitle:
          'Post stories, reels, photos, and memories with a community that loves pets.',
      icon: Icons.camera_alt_rounded,
      gradientColors: [Color(0xFF0EA5E9), Color(0xFF7C3AED)],
    ),
  ];

  bool get _isLastPage => _currentIndex == _items.length - 1;

  void _goNext() {
    if (_isLastPage) {
      // TODO: Navigate to login once auth screen is ready.
      debugPrint('Onboarding completed');
      return;
    }

    _pageController.nextPage(
      duration: const Duration(milliseconds: 320),
      curve: Curves.easeOutCubic,
    );
  }

  void _skip() {
    _pageController.animateToPage(
      _items.length - 1,
      duration: const Duration(milliseconds: 320),
      curve: Curves.easeOutCubic,
    );
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
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
                  width: 88,
                  height: 42,
                  onPressed: _skip,
                ),
              ),
            ),
          ),

          Positioned(
            left: AppSpacing.xxl,
            right: AppSpacing.xxl,
            bottom: AppSpacing.xxl,
            child: Column(
              children: [
                PageIndicator(
                  length: _items.length,
                  currentIndex: _currentIndex,
                ),
                const SizedBox(height: AppSpacing.xxl),
                PrimaryButton(
                  text: _isLastPage ? 'Get Started' : 'Continue',
                  onPressed: _goNext,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
