import 'package:flutter/material.dart';
import 'package:flutter_animate/flutter_animate.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/network/api_exception.dart';
import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/cards/glass_card.dart';
import '../../../../shared/widgets/inputs/app_text_field.dart';
import '../../data/auth_repository.dart';

/// Requests a password reset link. Reachable from `LoginScreen`'s "Forgot
/// password?" link, and public regardless of auth state — the same
/// reasoning as `VerifyEmailScreen`: this is exactly the flow for someone
/// who's currently logged out.
class ForgotPasswordScreen extends ConsumerStatefulWidget {
  const ForgotPasswordScreen({super.key, this.initialEmail});

  final String? initialEmail;

  @override
  ConsumerState<ForgotPasswordScreen> createState() =>
      _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends ConsumerState<ForgotPasswordScreen> {
  late final _emailController = TextEditingController(
    text: widget.initialEmail ?? '',
  );
  bool _sending = false;
  String? _message;
  bool _isError = false;
  bool _sent = false;

  @override
  void initState() {
    super.initState();
    _emailController.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _emailController.dispose();
    super.dispose();
  }

  bool get _canSend {
    final value = _emailController.text.trim();
    return value.contains('@') && value.contains('.');
  }

  Future<void> _send() async {
    setState(() {
      _sending = true;
      _message = null;
      _isError = false;
    });
    try {
      final message = await ref
          .read(authRepositoryProvider)
          .forgotPassword(_emailController.text.trim());
      if (!mounted) return;
      setState(() {
        _message = message;
        _sent = true;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _message = ApiException.from(error).message;
        _isError = true;
      });
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  void _enterToken() {
    final email = _emailController.text.trim();
    context.go(
      '${AppRoutes.resetPassword}?email=${Uri.encodeQueryComponent(email)}',
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.background,
      body: DecoratedBox(
        decoration: const BoxDecoration(gradient: AppColors.darkGradient),
        child: SafeArea(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: AppSpacing.xxxl),
                Container(
                      width: 72,
                      height: 72,
                      decoration: const BoxDecoration(
                        color: AppColors.primary,
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(
                        Icons.lock_reset_rounded,
                        color: AppColors.white,
                        size: 34,
                      ),
                    )
                    .animate()
                    .fadeIn(duration: 380.ms)
                    .scale(begin: const Offset(0.9, 0.9)),
                const SizedBox(height: AppSpacing.xl),
                Text(
                  'Reset your password',
                  style: AppTextStyles.displayMedium,
                ).animate().fadeIn(duration: 380.ms),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  "Enter your account email and we'll send a link to reset your password.",
                  style: AppTextStyles.bodyMedium.copyWith(
                    color: AppColors.textSecondary,
                  ),
                ).animate(delay: 80.ms).fadeIn(duration: 380.ms),
                const SizedBox(height: AppSpacing.xxxl),
                GlassCard(
                  borderRadius: AppRadius.xxl,
                  backgroundOpacity: 0.10,
                  borderOpacity: 0.16,
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      AppTextField(
                        controller: _emailController,
                        label: 'Email address',
                        hint: 'you@example.com',
                        keyboardType: TextInputType.emailAddress,
                        textInputAction: TextInputAction.done,
                        enabled: !_sent,
                        onSubmitted: (_) =>
                            _canSend && !_sending ? _send() : null,
                        prefix: const Icon(
                          Icons.alternate_email_rounded,
                          color: AppColors.textSecondary,
                        ),
                      ),
                      if (_message != null) ...[
                        const SizedBox(height: AppSpacing.md),
                        Text(
                          _message!,
                          style: AppTextStyles.bodySmall.copyWith(
                            color: _isError
                                ? AppColors.error
                                : AppColors.textSecondary,
                          ),
                        ),
                      ],
                      const SizedBox(height: AppSpacing.xl),
                      if (!_sent)
                        PrimaryButton(
                          text: _sending ? 'Sending...' : 'Send reset link',
                          icon: Icons.arrow_forward_rounded,
                          isLoading: _sending,
                          enabled: _canSend,
                          onPressed: _canSend ? _send : null,
                        )
                      else
                        PrimaryButton(
                          text: 'Have a reset token? Enter it',
                          icon: Icons.key_outlined,
                          onPressed: _enterToken,
                        ),
                    ],
                  ),
                ).animate(delay: 140.ms).fadeIn(duration: 420.ms),
                const SizedBox(height: AppSpacing.xl),
                Center(
                  child: TextButton(
                    onPressed: () => context.go(AppRoutes.login),
                    child: const Text('Back to sign in'),
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
