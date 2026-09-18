import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../features/auth/application/auth_controller.dart';
import '../../features/auth/presentation/screens/forgot_password_screen.dart';
import '../../features/auth/presentation/screens/login_screen.dart';
import '../../features/auth/presentation/screens/reset_password_screen.dart';
import '../../features/auth/presentation/screens/sessions_screen.dart';
import '../../features/auth/presentation/screens/verify_email_screen.dart';
import '../../features/onboarding/presentation/screens/onboarding_screen.dart';
import '../../features/onboarding/presentation/screens/profile_setup_screen.dart';
import '../../features/pets/presentation/screens/my_pets_screen.dart';
import '../../features/pets/presentation/screens/pet_form_screen.dart';
import '../../features/social/presentation/screens/chat_screen.dart';
import '../../features/social/presentation/screens/edit_profile_screen.dart';
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
    (previous, next) {
      // Only re-run `_redirect` when something it actually reads has
      // changed. Notifying on every AuthState change (e.g. a profile edit
      // updating unrelated fields like bio/city) makes GoRouter reprocess
      // its current route via `redirect`, and doing that while sitting on
      // a `push()`-ed route (not top-level navigation) was found to
      // duplicate that route's page — a second instance mounts right as
      // the first is popped, so e.g. EditProfileScreen's own `pop()` after
      // a successful save appeared to silently do nothing. Caught by
      // instrumenting the widget's instance-level lifecycle live, not by
      // code review — the duplicate page's `initState` fired between the
      // original's `pop()` call and its `dispose()`.
      if (previous == null || _affectsRedirect(previous, next)) {
        refreshNotifier.notify();
      }
    },
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
        path: AppRoutes.verifyEmail,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: VerifyEmailScreen(
              email: state.uri.queryParameters['email'],
              token: state.uri.queryParameters['token'],
            ),
          );
        },
      ),
      GoRoute(
        path: AppRoutes.forgotPassword,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: ForgotPasswordScreen(
              initialEmail: state.uri.queryParameters['email'],
            ),
          );
        },
      ),
      GoRoute(
        path: AppRoutes.resetPassword,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: ResetPasswordScreen(
              email: state.uri.queryParameters['email'],
              token: state.uri.queryParameters['token'],
            ),
          );
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
        path: AppRoutes.sessions,
        pageBuilder: (context, state) {
          return _slideFadePage(state: state, child: const SessionsScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.editProfile,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: const EditProfileScreen(),
          );
        },
      ),
      GoRoute(
        path: AppRoutes.pets,
        pageBuilder: (context, state) {
          return _slideFadePage(state: state, child: const MyPetsScreen());
        },
      ),
      GoRoute(
        path: AppRoutes.petForm,
        pageBuilder: (context, state) {
          return _slideFadePage(
            state: state,
            child: PetFormScreen(petId: state.uri.queryParameters['petId']),
          );
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

/// Whether a change from `previous` to `next` could change what
/// `_redirect` decides — i.e. whether it touches `initialized`,
/// `isAuthenticated` (user null-ness), `sessionExpired`, or
/// `onboardingCompleted`. Anything else (name/bio/city/isPrivate edits,
/// `isLoading` toggling, `error` messages) is irrelevant to routing and
/// must not trigger a redirect reprocessing pass — see the caller.
bool _affectsRedirect(AuthState previous, AuthState next) {
  return previous.initialized != next.initialized ||
      previous.isAuthenticated != next.isAuthenticated ||
      previous.sessionExpired != next.sessionExpired ||
      previous.user?.onboardingCompleted != next.user?.onboardingCompleted;
}

/// The location a redirect bounced the user away from while unauthenticated,
/// so it can be restored after they sign back in — see [_redirect] and
/// `LoginScreen._continue`. `null` means there's nothing to restore (the
/// default post-login destination applies).
final pendingRedirectProvider = StateProvider<String?>((ref) => null);

/// Locations that carry no useful "destination" to return to — capturing
/// one of these as a pending redirect would just bounce the user right back
/// into the auth flow after they log in.
const _authFlowLocations = {
  AppRoutes.splash,
  AppRoutes.onboarding,
  AppRoutes.login,
  AppRoutes.verifyEmail,
  AppRoutes.forgotPassword,
  AppRoutes.resetPassword,
  AppRoutes.setupProfile,
};

/// The single source of truth for "given this auth state, and no specific
/// location in mind, where should the app be?" — shared by [_redirect]
/// below and by [SplashScreen] once `restore()` resolves. This used to be
/// duplicated (splash had its own hardcoded `if (!isAuthenticated) go to
/// onboarding` independent of `_redirect`'s rules), and the two silently
/// disagreed the moment `sessionExpired` was added: splash's copy didn't
/// know about it, so a dead session was sent to the onboarding pitch
/// screens instead of `/login` every time splash — not `_redirect` — was
/// the one deciding, i.e. on every cold start and reload. Route any future
/// destination logic through this one function instead of re-deciding it
/// at each call site.
String defaultDestinationFor(AuthState auth) {
  if (!auth.isAuthenticated) {
    return auth.sessionExpired ? AppRoutes.login : AppRoutes.onboarding;
  }
  if (!auth.user!.onboardingCompleted) return AppRoutes.setupProfile;
  return AppRoutes.home;
}

void _capturePendingLocation(Ref ref, String location) {
  if (_authFlowLocations.contains(location)) return;
  ref.read(pendingRedirectProvider.notifier).state = location;
}

/// Decides whether [location] may be shown given the current [AuthState],
/// returning the location to redirect to instead, or `null` to allow it.
///
/// Rules (checked in order):
/// 1. Session restore hasn't finished yet (`!auth.initialized`) — capture
///    [location] as the pending redirect (so a cold deep link isn't lost)
///    and force everything to `/splash`, which is what actually triggers
///    `restore()`.
/// 2. Already on `/splash` — never force it away. [SplashScreen] owns its
///    own minimum display duration and navigates itself once `restore()`
///    resolves (via [defaultDestinationFor]); redirecting here too would
///    race it and cut its animation short.
/// 3. Not authenticated — only onboarding/login are reachable; anything
///    else (including a protected route reached via deep link, back
///    button, or manual URL entry) is captured as the pending redirect and
///    bounces to whatever [defaultDestinationFor] says. New protected
///    routes are safe by default: they're blocked unless added to
///    `publicLocations`, not the other way around.
/// 4. Authenticated but onboarding isn't complete — setup-profile is
///    reachable, and so is verify-email (registration routes there first;
///    see `LoginScreen._continue`), since checking email shouldn't require
///    onboarding to be done first.
/// 5. Fully authenticated and onboarded — auth-only screens (splash is
///    handled above, onboarding/login/verify-email/setup-profile here)
///    redirect home; everything else is allowed. Verification isn't a
///    permanent gate: nothing on the client currently tracks whether the
///    email was ever confirmed, so once onboarding is done this screen is
///    just another auth-flow screen to redirect away from, not something
///    to keep nagging about.
String? _redirect(Ref ref, String location) {
  final auth = ref.read(authControllerProvider);

  if (!auth.initialized) {
    _capturePendingLocation(ref, location);
    return location == AppRoutes.splash ? null : AppRoutes.splash;
  }

  if (location == AppRoutes.splash) return null;

  // verify-email, forgot-password, and reset-password are all public
  // rather than gated behind auth: a real verification/reset link is often
  // opened on a different device/browser than the one that registered or
  // requested it — possibly while fully logged out there — and the token
  // itself is the authorization, not the session. The API side already
  // works this way; verify-email's client gap (this screen used to require
  // being authenticated) was caught by testing a fresh, unauthenticated
  // deep link live rather than only ever visiting it already logged in.
  const publicLocations = {
    AppRoutes.onboarding,
    AppRoutes.login,
    AppRoutes.verifyEmail,
    AppRoutes.forgotPassword,
    AppRoutes.resetPassword,
  };
  if (!auth.isAuthenticated) {
    if (publicLocations.contains(location)) return null;
    _capturePendingLocation(ref, location);
    return defaultDestinationFor(auth);
  }

  final needsOnboarding = !auth.user!.onboardingCompleted;
  if (needsOnboarding) {
    const allowedWhilePendingOnboarding = {
      AppRoutes.setupProfile,
      AppRoutes.verifyEmail,
    };
    return allowedWhilePendingOnboarding.contains(location)
        ? null
        : AppRoutes.setupProfile;
  }

  const authOnlyLocations = {
    AppRoutes.onboarding,
    AppRoutes.login,
    AppRoutes.verifyEmail,
    AppRoutes.forgotPassword,
    AppRoutes.resetPassword,
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
