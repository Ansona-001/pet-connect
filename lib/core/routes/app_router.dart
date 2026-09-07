import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/application/auth_controller.dart';
import '../../features/auth/presentation/screens/login_screen.dart';
import '../../features/onboarding/presentation/screens/onboarding_screen.dart';
import '../../features/onboarding/presentation/screens/profile_setup_screen.dart';
import '../../features/social/presentation/screens/chat_screen.dart';
import '../../features/social/presentation/screens/inbox_screen.dart';
import '../../features/social/presentation/screens/social_shell_screen.dart';
import '../../features/splash/splash_screen.dart';
import 'routes.dart';

/// Builds the app's [GoRouter] with an auth-aware [GoRouter.redirect], so a
/// protected route can never be opened just by navigating/deep-linking to
/// it while unauthenticated — previously nothing enforced this centrally;
/// gating only happened inside individual screens' own navigation calls.
final goRouterProvider = Provider<GoRouter>((ref) {
  final refreshNotifier = _RouterRefreshNotifier();
  ref.listen<AuthState>(
    authControllerProvider,
    (previous, next) => refreshNotifier.notify(),
  );
  ref.onDispose(refreshNotifier.dispose);

  return GoRouter(
    initialLocation: AppRoutes.splash,
    refreshListenable: refreshNotifier,
    redirect: (context, state) => _redirect(ref, state.matchedLocation),
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
});

/// Decides whether [location] may be shown given the current [AuthState],
/// returning the location to redirect to instead, or `null` to allow it.
///
/// Rules (checked in order):
/// 1. Session restore hasn't finished yet (`!auth.initialized`) — force
///    everything to `/splash`, which is what actually triggers `restore()`.
/// 2. Already on `/splash` — never force it away. [SplashScreen] owns its
///    own minimum display duration and navigates itself once `restore()`
///    resolves; redirecting here too would race it and cut its animation
///    short.
/// 3. Not authenticated — only onboarding/login are reachable; anything
///    else (including a protected route reached via deep link, back
///    button, or manual URL entry) bounces to onboarding. New protected
///    routes are safe by default: they're blocked unless added to
///    `publicLocations`, not the other way around.
/// 4. Authenticated but onboarding isn't complete — only setup-profile is
///    reachable.
/// 5. Fully authenticated and onboarded — auth-only screens (splash is
///    handled above, onboarding/login/setup-profile here) redirect home;
///    everything else is allowed.
String? _redirect(Ref ref, String location) {
  final auth = ref.read(authControllerProvider);

  if (!auth.initialized) {
    return location == AppRoutes.splash ? null : AppRoutes.splash;
  }

  if (location == AppRoutes.splash) return null;

  const publicLocations = {AppRoutes.onboarding, AppRoutes.login};
  if (!auth.isAuthenticated) {
    return publicLocations.contains(location) ? null : AppRoutes.onboarding;
  }

  final needsOnboarding = !auth.user!.onboardingCompleted;
  if (needsOnboarding) {
    return location == AppRoutes.setupProfile ? null : AppRoutes.setupProfile;
  }

  const authOnlyLocations = {
    AppRoutes.onboarding,
    AppRoutes.login,
    AppRoutes.setupProfile,
  };
  return authOnlyLocations.contains(location) ? AppRoutes.home : null;
}

/// Bridges [authControllerProvider] state changes to GoRouter's
/// `refreshListenable`, so `redirect` re-runs on login/logout/restore
/// instead of only on explicit navigation.
class _RouterRefreshNotifier extends ChangeNotifier {
  void notify() => notifyListeners();
}

CustomTransitionPage<void> _fadePage({
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

CustomTransitionPage<void> _slideFadePage({
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
