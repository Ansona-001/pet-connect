import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/presentation/screens/login_screen.dart';
import '../../features/onboarding/presentation/screens/onboarding_screen.dart';
import '../../features/onboarding/presentation/screens/profile_setup_screen.dart';
import '../../features/social/presentation/screens/chat_screen.dart';
import '../../features/social/presentation/screens/inbox_screen.dart';
import '../../features/social/presentation/screens/social_shell_screen.dart';
import '../../features/splash/splash_screen.dart';
import 'routes.dart';

class AppRouter {
  AppRouter._();

  static final GoRouter router = GoRouter(
    initialLocation: AppRoutes.splash,
    routes: [
      GoRoute(
        path: AppRoutes.splash,
        pageBuilder: (context, state) {
          return _fadePage(state: state, child: const SplashScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.onboarding,
        pageBuilder: (context, state) {
          return _slideFadePage(state: state, child: const OnboardingScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.login,
        pageBuilder: (context, state) {
          return _slideFadePage(state: state, child: const LoginScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.setupProfile,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: const ProfileSetupScreen(),
          );
        },
      ),
      GoRoute(
        path: AppRoutes.home,
        pageBuilder: (context, state) {
          return _fadePage(state: state, child: const SocialShellScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.inbox,
        pageBuilder: (context, state) {
          return _slideFadePage(state: state, child: const InboxScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.chatPath,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: ChatScreen(
              chatId: state.pathParameters['chatId'] ?? 'chat-coco',
            ),
          );
        },
      ),
    ],
  );

  static CustomTransitionPage<void> _fadePage({
    required GoRouterState state,
    required Widget child,
  }) {
    return CustomTransitionPage(
      key: state.pageKey,
      child: child,
      transitionsBuilder: (context, animation, _, child) {
        return FadeTransition(opacity: animation, child: child);
      },
    );
  }

  static CustomTransitionPage<void> _slideFadePage({
    required GoRouterState state,
    required Widget child,
  }) {
    return CustomTransitionPage(
      key: state.pageKey,
      child: child,
      transitionsBuilder: (context, animation, _, child) {
        final curvedAnimation = CurvedAnimation(
          parent: animation,
          curve: Curves.easeOutCubic,
        );

        return FadeTransition(
          opacity: curvedAnimation,
          child: SlideTransition(
            position: Tween<Offset>(
              begin: const Offset(0, 0.04),
              end: Offset.zero,
            ).animate(curvedAnimation),
            child: child,
          ),
        );
      },
    );
  }
}
