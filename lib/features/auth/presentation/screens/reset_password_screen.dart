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

/// Consumes a password reset token. Reachable as a deep link
/// (`/reset-password?token=...`, the shape a real reset email would use)
/// and public regardless of auth state, for the same reason
/// `VerifyEmailScreen` is: the token is the authorization, not the
/// session, and the link is routinely opened logged out or on another
/// device. Unlike email verification, a token alone can't auto-complete
/// this screen — a new password is always required too.
class ResetPasswordScreen extends ConsumerStatefulWidget {
  const ResetPasswordScreen({super.key, this.email, this.token});

  final String? email;
  final String? token;

  @override
  ConsumerState<ResetPasswordScreen> createState() =>
      _ResetPasswordScreenState();
}

class _ResetPasswordScreenState extends ConsumerState<ResetPasswordScreen> {
  late final _tokenController = TextEditingController(text: widget.token ?? '');
  final _passwordController = TextEditingController();
  final _confirmController = TextEditingController();
  bool _submitting = false;
  String? _error;
  bool _done = false;

  @override
  void initState() {
    super.initState();
    _tokenController.addListener(() => setState(() {}));
    _passwordController.addListener(() => setState(() {}));
    _confirmController.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _tokenController.dispose();
    _passwordController.dispose();
    _confirmController.dispose();
    super.dispose();
  }

  String? get _validationError {
    if (_passwordController.text.length < 8) {
      return 'Password must be at least 8 characters.';
    }
    if (_passwordController.text != _confirmController.text) {
      return "Passwords don't match.";
    }
    return null;
  }

  bool get _canSubmit =>
      _tokenController.text.trim().isNotEmpty && _validationError == null;

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      await ref
          .read(authRepositoryProvider)
          .resetPassword(
            token: _tokenController.text.trim(),
            password: _passwordController.text,
          );
      if (!mounted) return;
      setState(() => _done = true);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = ApiException.from(error).message);
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
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
                      decoration: BoxDecoration(
                        color: _done ? AppColors.success : AppColors.primary,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        _done ? Icons.check_rounded : Icons.password_rounded,
                        color: AppColors.white,
                        size: 34,
                      ),
                    )
                    .animate()
                    .fadeIn(duration: 380.ms)
                    .scale(begin: const Offset(0.9, 0.9)),
                const SizedBox(height: AppSpacing.xl),
                Text(
                  _done ? 'Password updated' : 'Set a new password',
                  style: AppTextStyles.displayMedium,
                ).animate().fadeIn(duration: 380.ms),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  _done
                      ? 'Sign in with your new password.'
                      : 'Paste the token from your reset link and choose a new password.',
                  style: AppTextStyles.bodyMedium.copyWith(
                    color: AppColors.textSecondary,
                  ),
                ).animate(delay: 80.ms).fadeIn(duration: 380.ms),
                const SizedBox(height: AppSpacing.xxxl),
                if (!_done) ...[
                  GlassCard(
                    borderRadius: AppRadius.xxl,
                    backgroundOpacity: 0.10,
                    borderOpacity: 0.16,
                    padding: const EdgeInsets.all(AppSpacing.xl),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        AppTextField(
                          controller: _tokenController,
                          label: 'Reset token',
                          hint: 'Paste the token from your email link',
                          prefix: const Icon(
                            Icons.key_outlined,
                            color: AppColors.textSecondary,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.lg),
                        AppTextField(
                          controller: _passwordController,
                          label: 'New password',
                          isPassword: true,
                          prefix: const Icon(
                            Icons.lock_outline_rounded,
                            color: AppColors.textSecondary,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.lg),
                        AppTextField(
                          controller: _confirmController,
                          label: 'Confirm new password',
                          isPassword: true,
                          textInputAction: TextInputAction.done,
                          onSubmitted: (_) =>
                              _canSubmit && !_submitting ? _submit() : null,
                          prefix: const Icon(
                            Icons.lock_outline_rounded,
                            color: AppColors.textSecondary,
                          ),
                        ),
                        if (_error != null) ...[
                          const SizedBox(height: AppSpacing.md),
                          Text(
                            _error!,
                            style: AppTextStyles.bodySmall.copyWith(
                              color: AppColors.error,
                            ),
                          ),
                        ] else if (_passwordController.text.isNotEmpty ||
                            _confirmController.text.isNotEmpty) ...[
                          if (_validationError != null) ...[
                            const SizedBox(height: AppSpacing.md),
                            Text(
                              _validationError!,
                              style: AppTextStyles.bodySmall.copyWith(
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ],
                        ],
                        const SizedBox(height: AppSpacing.xl),
                        PrimaryButton(
                          text: _submitting ? 'Updating...' : 'Reset password',
                          icon: Icons.arrow_forward_rounded,
                          isLoading: _submitting,
                          enabled: _canSubmit,
                          onPressed: _canSubmit ? _submit : null,
                        ),
                      ],
                    ),
                  ).animate(delay: 140.ms).fadeIn(duration: 420.ms),
                  const SizedBox(height: AppSpacing.xl),
                  Center(
                    child: TextButton(
                      onPressed: () => context.go(
                        '${AppRoutes.forgotPassword}${widget.email != null ? '?email=${Uri.encodeQueryComponent(widget.email!)}' : ''}',
                      ),
                      child: const Text('Need a new link? Request one'),
                    ),
                  ),
                ] else
                  PrimaryButton(
                    text: 'Continue to sign in',
                    icon: Icons.arrow_forward_rounded,
                    onPressed: () => context.go(AppRoutes.login),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
