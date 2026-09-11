import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/routes/app_router.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_spacing.dart';
import '../../core/theme/app_text_styles.dart';
import '../auth/application/auth_controller.dart';

class SplashScreen extends ConsumerStatefulWidget {
  const SplashScreen({super.key});

  @override
  ConsumerState<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends ConsumerState<SplashScreen>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  late final Animation<double> _fade;
  late final Animation<double> _scale;

  @override
  void initState() {
    super.initState();

    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    );

    _fade = CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic);

    _scale = Tween<double>(
      begin: 0.92,
      end: 1,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic));

    _controller.forward();

    // Deferred to a microtask: now that the router is built inside a
    // Provider (goRouterProvider) and evaluates its initial redirect while
    // the app's widget tree is first building, calling restore() directly
    // here would mutate authControllerProvider's state synchronously from
    // within that same build phase, which Riverpod forbids ("Tried to
    // modify a provider while the widget tree was building").
    Future.microtask(_restoreSession);
  }

  Future<void> _restoreSession() async {
    await Future.wait([
      Future<void>.delayed(const Duration(milliseconds: 1200)),
      ref.read(authControllerProvider.notifier).restore(),
    ]);
    if (!mounted) return;
    final auth = ref.read(authControllerProvider);

    // Prefer restoring whatever the user originally tried to reach (a cold
    // deep link, captured by the router's redirect before it forced this
    // screen to mount) over the generic default — but only once auth
    // actually allows landing there; someone who still needs to complete
    // onboarding goes through setup-profile regardless of what they first
    // asked for.
    final pendingNotifier = ref.read(pendingRedirectProvider.notifier);
    final pending = pendingNotifier.state;
    final canRestorePending =
        pending != null &&
        auth.isAuthenticated &&
        auth.user!.onboardingCompleted;
    if (canRestorePending) {
      pendingNotifier.state = null;
      context.go(pending);
      return;
    }

    context.go(defaultDestinationFor(auth));
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      body: DecoratedBox(
        decoration: const BoxDecoration(gradient: AppColors.darkGradient),
        child: Center(
          child: FadeTransition(
            opacity: _fade,
            child: ScaleTransition(
              scale: _scale,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: 104,
                    height: 104,
                    decoration: BoxDecoration(
                      color: AppColors.primary,
                      shape: BoxShape.circle,
                      boxShadow: [
                        BoxShadow(
                          color: AppColors.primary.withValues(alpha: 0.32),
                          blurRadius: 42,
                          spreadRadius: 4,
                        ),
                      ],
                    ),
                    child: const Icon(
                      Icons.pets_rounded,
                      size: 52,
                      color: AppColors.white,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.xl),
                  Text('PetConnect', style: AppTextStyles.headingLarge),
                  const SizedBox(height: AppSpacing.sm),
                  Text(
                    'Find friends. Share moments.',
                    style: AppTextStyles.bodySmall,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
