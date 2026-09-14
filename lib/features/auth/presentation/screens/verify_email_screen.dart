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
import '../../application/auth_controller.dart';
import '../../data/auth_repository.dart';

/// Shown right after registration (see `LoginScreen._continue`) and also
/// reachable as a deep link with a `token` query parameter — the shape a
/// real verification email would use once a transactional email provider
/// is configured (brief §18; until then, `mailer.LogSender` on the backend
/// just logs the token for local testing).
///
/// This screen does not block the rest of the app: nothing here is a hard
/// gate, since a hard gate would strand every user while no real email
/// provider can guarantee delivery. It offers a real, working verify/resend
/// flow and lets the user continue regardless.
class VerifyEmailScreen extends ConsumerStatefulWidget {
  const VerifyEmailScreen({super.key, this.email, this.token});

  final String? email;
  final String? token;

  @override
  ConsumerState<VerifyEmailScreen> createState() => _VerifyEmailScreenState();
}

enum _Status { idle, working, verified, error }

class _VerifyEmailScreenState extends ConsumerState<VerifyEmailScreen> {
  final _tokenController = TextEditingController();
  _Status _status = _Status.idle;
  String? _message;
  bool _resending = false;
  String? _resendMessage;

  @override
  void initState() {
    super.initState();
    _tokenController.addListener(() => setState(() {}));
    final deepLinkToken = widget.token?.trim();
    if (deepLinkToken != null && deepLinkToken.isNotEmpty) {
      _tokenController.text = deepLinkToken;
      WidgetsBinding.instance.addPostFrameCallback((_) => _verify());
    }
  }

  @override
  void dispose() {
    _tokenController.dispose();
    super.dispose();
  }

  String? get _email =>
      widget.email ?? ref.read(authControllerProvider).user?.email;

  Future<void> _verify() async {
    final token = _tokenController.text.trim();
    if (token.isEmpty) return;
    setState(() {
      _status = _Status.working;
      _message = null;
    });
    try {
      await ref.read(authRepositoryProvider).verifyEmail(token);
      if (!mounted) return;
      setState(() => _status = _Status.verified);
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _status = _Status.error;
        _message = ApiException.from(error).message;
      });
    }
  }

  Future<void> _resend() async {
    final email = _email;
    if (email == null || email.isEmpty) return;
    setState(() {
      _resending = true;
      _resendMessage = null;
    });
    try {
      final message = await ref
          .read(authRepositoryProvider)
          .resendVerification(email);
      if (!mounted) return;
      setState(() => _resendMessage = message);
    } catch (error) {
      if (!mounted) return;
      setState(() => _resendMessage = ApiException.from(error).message);
    } finally {
      if (mounted) setState(() => _resending = false);
    }
  }

  void _continue() {
    final auth = ref.read(authControllerProvider);
    // A verification link can be opened on a device/browser that never
    // logged in here at all (see app_router.dart's `_redirect` — this
    // screen is public for exactly that reason). Route by what's actually
    // true instead of assuming a session exists.
    if (!auth.isAuthenticated) {
      context.go(AppRoutes.login);
      return;
    }
    context.go(
      auth.user!.onboardingCompleted ? AppRoutes.home : AppRoutes.setupProfile,
    );
  }

  @override
  Widget build(BuildContext context) {
    final email = _email;
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
                        color: _status == _Status.verified
                            ? AppColors.success
                            : AppColors.primary,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        _status == _Status.verified
                            ? Icons.check_rounded
                            : Icons.mark_email_read_outlined,
                        color: AppColors.white,
                        size: 34,
                      ),
                    )
                    .animate()
                    .fadeIn(duration: 380.ms)
                    .scale(begin: const Offset(0.9, 0.9)),
                const SizedBox(height: AppSpacing.xl),
                Text(
                  _status == _Status.verified
                      ? 'Email verified'
                      : 'Check your email',
                  style: AppTextStyles.displayMedium,
                ).animate().fadeIn(duration: 380.ms),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  _status == _Status.verified
                      ? 'Your email address is confirmed.'
                      : email != null
                      ? "We sent a verification link to $email. Open it on this device, or paste the link's token below."
                      : 'We sent a verification link to your email. Open it on this device, or paste the link\'s token below.',
                  style: AppTextStyles.bodyMedium.copyWith(
                    color: AppColors.textSecondary,
                  ),
                ).animate(delay: 80.ms).fadeIn(duration: 380.ms),
                const SizedBox(height: AppSpacing.xxxl),
                if (_status != _Status.verified) ...[
                  GlassCard(
                    borderRadius: AppRadius.xxl,
                    backgroundOpacity: 0.10,
                    borderOpacity: 0.16,
                    padding: const EdgeInsets.all(AppSpacing.xl),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Have a verification token?',
                          style: AppTextStyles.headingSmall,
                        ),
                        const SizedBox(height: AppSpacing.lg),
                        AppTextField(
                          controller: _tokenController,
                          label: 'Verification token',
                          hint: 'Paste the token from your email link',
                          prefix: const Icon(
                            Icons.key_outlined,
                            color: AppColors.textSecondary,
                          ),
                          onSubmitted: (_) => _verify(),
                        ),
                        if (_status == _Status.error && _message != null) ...[
                          const SizedBox(height: AppSpacing.md),
                          Text(
                            _message!,
                            style: AppTextStyles.bodySmall.copyWith(
                              color: AppColors.error,
                            ),
                          ),
                        ],
                        const SizedBox(height: AppSpacing.xl),
                        PrimaryButton(
                          text: _status == _Status.working
                              ? 'Verifying...'
                              : 'Verify',
                          icon: Icons.arrow_forward_rounded,
                          isLoading: _status == _Status.working,
                          onPressed: _tokenController.text.trim().isEmpty
                              ? null
                              : _verify,
                        ),
                      ],
                    ),
                  ).animate(delay: 140.ms).fadeIn(duration: 420.ms),
                  const SizedBox(height: AppSpacing.xl),
                  Center(
                    child: TextButton(
                      onPressed: _resending ? null : _resend,
                      child: Text(
                        _resending
                            ? 'Sending...'
                            : "Didn't get it? Resend email",
                      ),
                    ),
                  ),
                  if (_resendMessage != null)
                    Center(
                      child: Padding(
                        padding: const EdgeInsets.only(top: AppSpacing.sm),
                        child: Text(
                          _resendMessage!,
                          textAlign: TextAlign.center,
                          style: AppTextStyles.bodySmall.copyWith(
                            color: AppColors.textSecondary,
                          ),
                        ),
                      ),
                    ),
                  const SizedBox(height: AppSpacing.xl),
                ],
                PrimaryButton(
                  text: _status == _Status.verified
                      ? 'Continue'
                      : "I'll do this later",
                  icon: Icons.arrow_forward_rounded,
                  onPressed: _continue,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
