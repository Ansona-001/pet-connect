import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../application/social_controller.dart';
import 'discover_screen.dart';
import 'home_screen.dart';
import 'match_screen.dart';
import 'profile_screen.dart';
import 'reels_screen.dart';

class SocialShellScreen extends ConsumerStatefulWidget {
  const SocialShellScreen({super.key});

  @override
  ConsumerState<SocialShellScreen> createState() => _SocialShellScreenState();
}

class _SocialShellScreenState extends ConsumerState<SocialShellScreen> {
  int _index = 0;
  late final List<Widget?> _screens;

  @override
  void initState() {
    super.initState();
    _screens = List<Widget?>.filled(5, null)..[0] = const HomeScreen();
    Future<void>.microtask(
      () => ref.read(socialControllerProvider.notifier).initialize(),
    );
  }

  Widget _buildScreen(int index) => switch (index) {
    0 => const HomeScreen(),
    1 => const DiscoverScreen(),
    2 => const MatchScreen(),
    3 => const ReelsScreen(),
    4 => const ProfileScreen(),
    _ => const SizedBox.shrink(),
  };

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: IndexedStack(
            index: _index,
            children: List<Widget>.generate(
              _screens.length,
              (index) => _screens[index] ?? const SizedBox.shrink(),
              growable: false,
            ),
          ),
        ),
      ),
      bottomNavigationBar: SafeArea(
        top: false,
        child: Center(
          heightFactor: 1,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(12, 4, 12, 8),
              child: ClipRRect(
                borderRadius: AppRadius.xl,
                child: NavigationBar(
                  selectedIndex: _index,
                  height: 68,
                  backgroundColor: AppColors.surface,
                  indicatorColor: AppColors.primary.withValues(alpha: 0.20),
                  labelBehavior: NavigationDestinationLabelBehavior.alwaysShow,
                  onDestinationSelected: (value) {
                    setState(() {
                      _index = value;
                      _screens[value] ??= _buildScreen(value);
                    });
                  },
                  destinations: const [
                    NavigationDestination(
                      icon: Icon(Icons.home_outlined),
                      selectedIcon: Icon(Icons.home_rounded),
                      label: 'Home',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.travel_explore_outlined),
                      selectedIcon: Icon(Icons.travel_explore_rounded),
                      label: 'Discover',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.favorite_border_rounded),
                      selectedIcon: Icon(Icons.favorite_rounded),
                      label: 'Match',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.play_circle_outline_rounded),
                      selectedIcon: Icon(Icons.play_circle_fill_rounded),
                      label: 'Reels',
                    ),
                    NavigationDestination(
                      icon: Icon(Icons.person_outline_rounded),
                      selectedIcon: Icon(Icons.person_rounded),
                      label: 'Profile',
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
