import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/network/api_exception.dart';
import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../application/auth_controller.dart';
import '../../data/auth_repository.dart';
import '../../domain/session_info.dart';

/// Lists the account's active sessions and lets the owner revoke one, or
/// sign out of every device at once — brief Milestone 1: "session/device
/// list, revoke session(s), logout-all". Reached from Profile's settings
/// sheet, alongside the existing single-device "Sign out."
class SessionsScreen extends ConsumerStatefulWidget {
  const SessionsScreen({super.key});

  @override
  ConsumerState<SessionsScreen> createState() => _SessionsScreenState();
}

class _SessionsScreenState extends ConsumerState<SessionsScreen> {
  List<SessionInfo>? _sessions;
  String? _error;
  bool _loading = true;
  String? _revokingId;
  bool _loggingOutAll = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final sessions = await ref.read(authRepositoryProvider).listSessions();
      if (!mounted) return;
      setState(() {
        _sessions = sessions;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = ApiException.from(error).message;
        _loading = false;
      });
    }
  }

  Future<void> _revoke(SessionInfo session) async {
    setState(() => _revokingId = session.id);
    try {
      await ref.read(authRepositoryProvider).revokeSession(session.id);
      if (!mounted) return;
      setState(() {
        _sessions = _sessions?.where((s) => s.id != session.id).toList();
      });
    } catch (error) {
      if (!mounted) return;
      _showSnack(ApiException.from(error).message);
    } finally {
      if (mounted) setState(() => _revokingId = null);
    }
  }

  Future<void> _logoutEverywhere() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: AppColors.surface,
        title: const Text('Log out of all devices?'),
        content: const Text(
          'This signs you out everywhere, including this device. '
          "You'll need to sign in again.",
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('Log out everywhere'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    setState(() => _loggingOutAll = true);
    await ref.read(authControllerProvider.notifier).logoutAll();
    if (!mounted) return;
    context.go(AppRoutes.login);
  }

  void _showSnack(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  void _back() {
    if (context.canPop()) {
      context.pop();
    } else {
      context.go(AppRoutes.home);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AppScaffold(
      child: LayoutBuilder(
        builder: (context, constraints) => Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: ListView(
              physics: const BouncingScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.sm,
                AppSpacing.lg,
                AppSpacing.xxxl,
              ),
              children: [
                _SessionsHeader(onBack: _back, onRefresh: _load),
                const SizedBox(height: AppSpacing.xl),
                ..._buildBody(),
              ],
            ),
          ),
        ),
      ),
    );
  }

  List<Widget> _buildBody() {
    if (_loading) {
      return const [
        SizedBox(height: AppSpacing.xxxl),
        Center(child: CircularProgressIndicator(color: AppColors.primary)),
      ];
    }
    if (_error != null) {
      return [
        Text(
          _error!,
          style: AppTextStyles.bodyMedium.copyWith(color: AppColors.error),
        ),
        const SizedBox(height: AppSpacing.lg),
        PrimaryButton(text: 'Retry', onPressed: _load),
      ];
    }

    final sessions = _sessions ?? const [];
    if (sessions.isEmpty) {
      return [
        Text(
          'No active sessions found.',
          style: AppTextStyles.bodyMedium.copyWith(
            color: AppColors.textSecondary,
          ),
        ),
      ];
    }

    return [
      for (final session in sessions) ...[
        _SessionTile(
          session: session,
          revoking: _revokingId == session.id,
          onRevoke: session.isCurrent ? null : () => _revoke(session),
        ),
        const SizedBox(height: AppSpacing.md),
      ],
      const SizedBox(height: AppSpacing.lg),
      PrimaryButton(
        text: _loggingOutAll ? 'Logging out...' : 'Log out of all devices',
        icon: Icons.logout_rounded,
        isLoading: _loggingOutAll,
        onPressed: _loggingOutAll ? null : _logoutEverywhere,
      ),
      const SizedBox(height: AppSpacing.sm),
      Center(
        child: Text(
          'This signs every device — including this one — out immediately.',
          textAlign: TextAlign.center,
          style: AppTextStyles.caption.copyWith(color: AppColors.textSecondary),
        ),
      ),
    ];
  }
}

class _SessionsHeader extends StatelessWidget {
  const _SessionsHeader({required this.onBack, required this.onRefresh});

  final VoidCallback onBack;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        IconButton(
          tooltip: 'Back',
          onPressed: onBack,
          icon: const Icon(Icons.arrow_back_rounded),
        ),
        const SizedBox(width: AppSpacing.xs),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Sessions', style: AppTextStyles.headingLarge),
              const SizedBox(height: AppSpacing.xs),
              Text(
                'Devices signed in to your account',
                style: AppTextStyles.bodySmall,
              ),
            ],
          ),
        ),
        IconButton(
          tooltip: 'Refresh',
          onPressed: onRefresh,
          icon: const Icon(Icons.refresh_rounded),
        ),
      ],
    );
  }
}

class _SessionTile extends StatelessWidget {
  const _SessionTile({
    required this.session,
    required this.revoking,
    required this.onRevoke,
  });

  final SessionInfo session;
  final bool revoking;
  final VoidCallback? onRevoke;

  IconData get _platformIcon {
    switch (session.platform) {
      case 'ios':
      case 'android':
        return Icons.smartphone_rounded;
      case 'windows':
      case 'macos':
      case 'linux':
        return Icons.laptop_mac_rounded;
      default:
        return Icons.public_rounded;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.surface,
        borderRadius: AppRadius.lg,
        border: Border.all(color: AppColors.border),
      ),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: AppColors.card,
              shape: BoxShape.circle,
            ),
            child: Icon(
              _platformIcon,
              color: AppColors.textSecondary,
              size: 22,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        session.deviceName,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTextStyles.bodyMedium,
                      ),
                    ),
                    if (session.isCurrent) ...[
                      const SizedBox(width: AppSpacing.sm),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.sm,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: AppColors.primary.withValues(alpha: 0.16),
                          borderRadius: AppRadius.pill,
                        ),
                        child: Text(
                          'This device',
                          style: AppTextStyles.caption.copyWith(
                            color: AppColors.primary,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  'Active ${_relativeTime(session.lastUsedAt)}',
                  style: AppTextStyles.bodySmall.copyWith(
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
          if (onRevoke != null)
            SizedBox(
              width: 40,
              height: 40,
              child: revoking
                  ? const Padding(
                      padding: EdgeInsets.all(10),
                      child: CircularProgressIndicator(
                        strokeWidth: 2,
                        color: AppColors.error,
                      ),
                    )
                  : IconButton(
                      tooltip: 'Revoke session',
                      onPressed: onRevoke,
                      icon: const Icon(
                        Icons.logout_rounded,
                        color: AppColors.error,
                        size: 20,
                      ),
                    ),
            ),
        ],
      ),
    );
  }
}

/// Deliberately simple duration-bucketed formatting rather than pulling in
/// a date-formatting dependency for one string.
String _relativeTime(DateTime value) {
  final diff = DateTime.now().difference(value);
  if (diff.inMinutes < 1) return 'just now';
  if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
  if (diff.inHours < 24) return '${diff.inHours}h ago';
  if (diff.inDays < 7) return '${diff.inDays}d ago';
  const months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];
  return 'on ${months[value.month - 1]} ${value.day}';
}
