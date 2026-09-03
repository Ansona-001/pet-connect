import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_shadows.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/cards/glass_card.dart';
import '../../../../shared/widgets/inputs/app_text_field.dart';
import '../../application/auth_controller.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _emailController = TextEditingController(text: 'demo@petconnect.local');
  final _passwordController = TextEditingController(text: 'PetConnect123!');
  final _nameController = TextEditingController();
  bool _createAccount = false;

  bool get _canContinue {
    final value = _emailController.text.trim();
    return value.contains('@') &&
        value.contains('.') &&
        _passwordController.text.length >= 8 &&
        (!_createAccount || _nameController.text.trim().length >= 2);
  }

  @override
  void initState() {
    super.initState();
    _emailController.addListener(() => setState(() {}));
    _passwordController.addListener(() => setState(() {}));
    _nameController.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _emailController.dispose();
    _passwordController.dispose();
    _nameController.dispose();
    super.dispose();
  }

  Future<void> _continue() async {
    final controller = ref.read(authControllerProvider.notifier);
    final user = _createAccount
        ? await controller.register(
            email: _emailController.text.trim(),
            password: _passwordController.text,
            name: _nameController.text.trim(),
          )
        : await controller.login(
            email: _emailController.text.trim(),
            password: _passwordController.text,
          );
    if (!mounted || user == null) return;
    context.go(
      user.onboardingCompleted ? AppRoutes.home : AppRoutes.setupProfile,
    );
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authControllerProvider);
    return Scaffold(
      backgroundColor: AppColors.background,
      body: Stack(
        children: [
          const _LoginBackground(),

          SafeArea(
            child: LayoutBuilder(
              builder: (context, constraints) {
                return SingleChildScrollView(
                  keyboardDismissBehavior:
                      ScrollViewKeyboardDismissBehavior.onDrag,
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: ConstrainedBox(
                    constraints: BoxConstraints(
                      minHeight: constraints.maxHeight - (AppSpacing.xl * 2),
                    ),
                    child: IntrinsicHeight(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const _BrandHeader()
                              .animate()
                              .fadeIn(duration: 420.ms)
                              .slideY(begin: -0.12, end: 0),

                          const Spacer(),

                          Text(
                                'Start your\npet social life.',
                                style: AppTextStyles.displayMedium,
                              )
                              .animate()
                              .fadeIn(duration: 420.ms)
                              .slideY(begin: 0.12, end: 0),

                          const SizedBox(height: AppSpacing.lg),

                          Text(
                                'Join pet lovers nearby, discover companions, and share your favorite moments.',
                                style: AppTextStyles.bodyMedium.copyWith(
                                  color: AppColors.textSecondary,
                                ),
                              )
                              .animate(delay: 80.ms)
                              .fadeIn(duration: 420.ms)
                              .slideY(begin: 0.12, end: 0),

                          const SizedBox(height: AppSpacing.xxxl),

                          GlassCard(
                                borderRadius: AppRadius.xxl,
                                backgroundOpacity: 0.10,
                                borderOpacity: 0.16,
                                padding: const EdgeInsets.all(AppSpacing.xl),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      _createAccount
                                          ? 'Create your account'
                                          : 'Sign in with email',
                                      style: AppTextStyles.headingSmall,
                                    ),
                                    const SizedBox(height: AppSpacing.sm),
                                    Text(
                                      'Your session is securely refreshed so you stay signed in.',
                                      style: AppTextStyles.bodySmall,
                                    ),
                                    const SizedBox(height: AppSpacing.xl),
                                    if (_createAccount) ...[
                                      AppTextField(
                                        controller: _nameController,
                                        label: 'Your name',
                                        textInputAction: TextInputAction.next,
                                        prefix: const Icon(
                                          Icons.person_outline_rounded,
                                          color: AppColors.textSecondary,
                                        ),
                                      ),
                                      const SizedBox(height: AppSpacing.lg),
                                    ],
                                    AppTextField(
                                      controller: _emailController,
                                      label: 'Email address',
                                      hint: 'you@example.com',
                                      keyboardType: TextInputType.emailAddress,
                                      textInputAction: TextInputAction.next,
                                      prefix: const Icon(
                                        Icons.alternate_email_rounded,
                                        color: AppColors.textSecondary,
                                      ),
                                    ),
                                    const SizedBox(height: AppSpacing.lg),
                                    AppTextField(
                                      controller: _passwordController,
                                      label: 'Password',
                                      isPassword: true,
                                      textInputAction: TextInputAction.done,
                                      onSubmitted: (_) {
                                        if (_canContinue &&
                                            !authState.isLoading) {
                                          _continue();
                                        }
                                      },
                                      prefix: const Icon(
                                        Icons.lock_outline_rounded,
                                        color: AppColors.textSecondary,
                                      ),
                                    ),
                                    if (authState.error != null) ...[
                                      const SizedBox(height: AppSpacing.md),
                                      Text(
                                        authState.error!,
                                        style: AppTextStyles.bodySmall.copyWith(
                                          color: AppColors.error,
                                        ),
                                      ),
                                    ],
                                    const SizedBox(height: AppSpacing.xl),
                                    PrimaryButton(
                                      text: authState.isLoading
                                          ? 'Connecting...'
                                          : _createAccount
                                          ? 'Create account'
                                          : 'Sign in',
                                      icon: Icons.arrow_forward_rounded,
                                      enabled:
                                          _canContinue && !authState.isLoading,
                                      onPressed:
                                          _canContinue && !authState.isLoading
                                          ? _continue
                                          : null,
                                    ),
                                    const SizedBox(height: AppSpacing.sm),
                                    Center(
                                      child: TextButton(
                                        onPressed: authState.isLoading
                                            ? null
                                            : () => setState(
                                                () => _createAccount =
                                                    !_createAccount,
                                              ),
                                        child: Text(
                                          _createAccount
                                              ? 'Already have an account? Sign in'
                                              : 'New here? Create an account',
                                        ),
                                      ),
                                    ),
                                  ],
                                ),
                              )
                              .animate(delay: 160.ms)
                              .fadeIn(duration: 460.ms)
                              .slideY(begin: 0.16, end: 0),

                          const SizedBox(height: AppSpacing.xl),

                          Center(
                            child: Text(
                              'Google and Apple sign-in become available after provider credentials are configured.',
                              textAlign: TextAlign.center,
                              style: AppTextStyles.caption.copyWith(
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ).animate(delay: 200.ms).fadeIn(duration: 420.ms),

                          const SizedBox(height: AppSpacing.lg),

                          Center(
                            child: Text(
                              'By continuing, you agree to our Terms & Privacy Policy.',
                              textAlign: TextAlign.center,
                              style: AppTextStyles.caption.copyWith(
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ).animate(delay: 220.ms).fadeIn(duration: 420.ms),

                          const SizedBox(height: AppSpacing.lg),
                        ],
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _LoginBackground extends StatelessWidget {
  const _LoginBackground();

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        const Positioned.fill(
          child: DecoratedBox(
            decoration: BoxDecoration(gradient: AppColors.darkGradient),
          ),
        ),

        Positioned(
          top: -120,
          right: -120,
          child: _GlowOrb(
            size: 280,
            color: AppColors.primary.withValues(alpha: 0.30),
          ),
        ),

        Positioned(
          bottom: 180,
          left: -140,
          child: _GlowOrb(
            size: 260,
            color: AppColors.accent.withValues(alpha: 0.22),
          ),
        ),

        Positioned(
          top: 170,
          left: 28,
          right: 28,
          child:
              Container(
                    height: 190,
                    decoration: BoxDecoration(
                      borderRadius: AppRadius.xxl,
                      color: AppColors.card,
                      boxShadow: AppShadows.card,
                    ),
                    child: Stack(
                      children: [
                        Positioned.fill(
                          child: DecoratedBox(
                            decoration: BoxDecoration(
                              borderRadius: AppRadius.xxl,
                              gradient: LinearGradient(
                                begin: Alignment.topLeft,
                                end: Alignment.bottomRight,
                                colors: [
                                  AppColors.primary.withValues(alpha: 0.22),
                                  AppColors.accent.withValues(alpha: 0.18),
                                  AppColors.card,
                                ],
                              ),
                            ),
                          ),
                        ),
                        Center(
                          child: Icon(
                            Icons.pets_rounded,
                            size: 92,
                            color: AppColors.white.withValues(alpha: 0.92),
                          ),
                        ),
                      ],
                    ),
                  )
                  .animate()
                  .fadeIn(duration: 500.ms)
                  .scale(
                    begin: const Offset(0.94, 0.94),
                    end: const Offset(1, 1),
                    duration: 500.ms,
                    curve: Curves.easeOutCubic,
                  ),
        ),
      ],
    );
  }
}

class _GlowOrb extends StatelessWidget {
  const _GlowOrb({required this.size, required this.color});

  final double size;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: size,
      height: size,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        color: color,
        boxShadow: [BoxShadow(color: color, blurRadius: 80, spreadRadius: 40)],
      ),
    );
  }
}

class _BrandHeader extends StatelessWidget {
  const _BrandHeader();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(
          height: 42,
          width: 42,
          decoration: BoxDecoration(
            color: AppColors.primary,
            shape: BoxShape.circle,
            boxShadow: AppShadows.colored(
              AppColors.primary,
              opacity: 0.28,
              blur: 20,
              offsetY: 8,
            ),
          ),
          child: const Icon(
            Icons.pets_rounded,
            color: AppColors.white,
            size: 22,
          ),
        ),
        const SizedBox(width: AppSpacing.md),
        Text('PetConnect', style: AppTextStyles.headingSmall),
      ],
    );
  }
}
