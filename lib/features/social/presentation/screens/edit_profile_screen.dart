import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/inputs/app_text_field.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../../auth/application/auth_controller.dart';

/// Lets the owner edit their own name/bio/city and set the "private
/// profile" preference — brief Milestone 1: "Owner profile editing (bio,
/// city, privacy prefs)". Reached from Profile's "Edit profile" button.
class EditProfileScreen extends ConsumerStatefulWidget {
  const EditProfileScreen({super.key});

  @override
  ConsumerState<EditProfileScreen> createState() => _EditProfileScreenState();
}

class _EditProfileScreenState extends ConsumerState<EditProfileScreen> {
  late final TextEditingController _nameController;
  late final TextEditingController _bioController;
  late final TextEditingController _cityController;
  late bool _isPrivate;
  late bool _isDiscoverable;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    final user = ref.read(authControllerProvider).user;
    _nameController = TextEditingController(text: user?.name ?? '');
    _bioController = TextEditingController(text: user?.bio ?? '');
    _cityController = TextEditingController(text: user?.city ?? '');
    _isPrivate = user?.isPrivate ?? false;
    _isDiscoverable = user?.isDiscoverable ?? true;
    _nameController.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _nameController.dispose();
    _bioController.dispose();
    _cityController.dispose();
    super.dispose();
  }

  bool get _canSave => _nameController.text.trim().isNotEmpty;

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    final success = await ref
        .read(authControllerProvider.notifier)
        .updateProfile(
          name: _nameController.text,
          bio: _bioController.text,
          city: _cityController.text,
          isPrivate: _isPrivate,
          isDiscoverable: _isDiscoverable,
        );
    if (!mounted) return;
    if (success) {
      context.pop();
      return;
    }
    setState(() {
      _saving = false;
      _error =
          ref.read(authControllerProvider).error ??
          'The profile could not be saved.';
    });
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
                Row(
                  children: [
                    IconButton(
                      tooltip: 'Back',
                      onPressed: _back,
                      icon: const Icon(Icons.arrow_back_rounded),
                    ),
                    const SizedBox(width: AppSpacing.xs),
                    Text('Edit profile', style: AppTextStyles.headingLarge),
                  ],
                ),
                const SizedBox(height: AppSpacing.xl),
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
                AppTextField(
                  controller: _cityController,
                  label: 'City',
                  textInputAction: TextInputAction.next,
                  prefix: const Icon(
                    Icons.location_on_outlined,
                    color: AppColors.textSecondary,
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                AppTextField(
                  controller: _bioController,
                  label: 'Bio',
                  maxLines: 4,
                  minLines: 3,
                  textInputAction: TextInputAction.done,
                  prefix: const Icon(
                    Icons.notes_rounded,
                    color: AppColors.textSecondary,
                  ),
                ),
                const SizedBox(height: AppSpacing.xl),
                Container(
                  decoration: BoxDecoration(
                    color: AppColors.surface,
                    borderRadius: AppRadius.lg,
                    border: Border.all(color: AppColors.border),
                  ),
                  child: SwitchListTile.adaptive(
                    value: _isPrivate,
                    onChanged: (value) => setState(() => _isPrivate = value),
                    title: const Text('Private profile'),
                    subtitle: const Text(
                      'Only people you follow or match with can see your '
                      'full profile and pet details.',
                    ),
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppSpacing.lg,
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                Container(
                  decoration: BoxDecoration(
                    color: AppColors.surface,
                    borderRadius: AppRadius.lg,
                    border: Border.all(color: AppColors.border),
                  ),
                  child: SwitchListTile.adaptive(
                    value: _isDiscoverable,
                    onChanged: (value) =>
                        setState(() => _isDiscoverable = value),
                    title: const Text('Show me in discovery'),
                    subtitle: const Text(
                      'Lets nearby pet parents find you through location-based '
                      'discovery and matching. Turning this off hides your '
                      'pets from both, even for people you already follow.',
                    ),
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppSpacing.lg,
                    ),
                  ),
                ),
                if (_error != null) ...[
                  const SizedBox(height: AppSpacing.lg),
                  Text(
                    _error!,
                    style: AppTextStyles.bodySmall.copyWith(
                      color: AppColors.error,
                    ),
                  ),
                ],
                const SizedBox(height: AppSpacing.xl),
                PrimaryButton(
                  text: _saving ? 'Saving...' : 'Save changes',
                  icon: Icons.check_rounded,
                  isLoading: _saving,
                  enabled: _canSave && !_saving,
                  onPressed: _canSave && !_saving ? _save : null,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
