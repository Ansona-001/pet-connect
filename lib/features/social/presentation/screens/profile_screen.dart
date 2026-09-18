import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_shadows.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../../auth/application/auth_controller.dart';
import '../../application/social_controller.dart';
import '../../data/mock_social_data.dart';

class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen> {
  int _selectedMediaTab = 0;

  static const _mediaAssets = <String>[
    MockSocialData.golden,
    MockSocialData.playdate,
    MockSocialData.sheltie,
    MockSocialData.golden,
    MockSocialData.cat,
    MockSocialData.playdate,
    MockSocialData.golden,
    MockSocialData.sheltie,
    MockSocialData.playdate,
  ];

  void _showFeedback(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  void _showPremium(String petName) {
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      isScrollControlled: true,
      builder: (sheetContext) => _PremiumSheet(
        petName: petName,
        onContinue: () {
          Navigator.of(sheetContext).pop();
          _showFeedback('Premium checkout will be available soon.');
        },
      ),
    );
  }

  Future<void> _shareProfile(String petName) async {
    final slug = petName.trim().toLowerCase().replaceAll(' ', '-');
    await Clipboard.setData(
      ClipboardData(text: 'https://petconnect.app/pet/$slug'),
    );
    if (mounted) _showFeedback('Profile link copied to your clipboard.');
  }

  Future<void> _showSettings() async {
    final action = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      builder: (sheetContext) => SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.devices_rounded),
              title: const Text('Manage sessions'),
              subtitle: const Text('See and sign out other signed-in devices.'),
              onTap: () => Navigator.of(sheetContext).pop('sessions'),
            ),
            ListTile(
              leading: const Icon(Icons.logout_rounded, color: AppColors.error),
              title: const Text('Sign out'),
              subtitle: const Text('Remove this session from this device.'),
              onTap: () => Navigator.of(sheetContext).pop('sign_out'),
            ),
          ],
        ),
      ),
    );
    if (!mounted || action == null) return;

    if (action == 'sessions') {
      context.push(AppRoutes.sessions);
      return;
    }

    await ref.read(authControllerProvider.notifier).logout();
    ref.invalidate(socialControllerProvider);
    if (mounted) context.go(AppRoutes.login);
  }

  @override
  Widget build(BuildContext context) {
    final socialState = ref.watch(socialControllerProvider);
    final setup = socialState.userProfile;
    // SocialState.userProfile.ownerName/city are a one-time snapshot taken
    // when SocialController was constructed (see socialControllerProvider)
    // and never refreshed afterward. AuthUser is the source of truth kept
    // live by EditProfileScreen, so it takes priority here — falling back
    // to the snapshot only if, for some reason, there's no signed-in user.
    final authUser = ref.watch(authControllerProvider).user;
    final ownerName = authUser?.name.isNotEmpty == true
        ? authUser!.name
        : setup.ownerName;
    final city = authUser?.city.isNotEmpty == true
        ? authUser!.city
        : setup.city;
    final selectedPet = socialState.pets.isEmpty
        ? null
        : socialState.pets[socialState.activePetIndex < socialState.pets.length
              ? socialState.activePetIndex
              : 0];
    final isCompanion = selectedPet == null && socialState.activePetIndex == 1;
    final companionIsLuna = setup.petName.toLowerCase() == 'simba';
    final petName =
        selectedPet?.name ??
        (isCompanion ? (companionIsLuna ? 'Luna' : 'Simba') : setup.petName);
    final petType = selectedPet == null
        ? isCompanion
              ? (companionIsLuna ? 'Golden Retriever' : 'Persian Cat')
              : setup.petType
        : (selectedPet.breed.isEmpty ? selectedPet.petType : selectedPet.breed);
    final imageAsset =
        selectedPet?.imageAsset ??
        (isCompanion
            ? (companionIsLuna ? MockSocialData.golden : MockSocialData.cat)
            : setup.petType.toLowerCase() == 'cat'
            ? MockSocialData.cat
            : MockSocialData.golden);

    return AppScaffold(
      child: LayoutBuilder(
        builder: (context, constraints) => SingleChildScrollView(
          physics: const BouncingScrollPhysics(),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 720),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(
                  AppSpacing.lg,
                  AppSpacing.sm,
                  AppSpacing.lg,
                  AppSpacing.xxxl,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _ProfileTopBar(
                      onNotificationTap: () =>
                          _showFeedback('You’re all caught up!'),
                      onSettingsTap: _showSettings,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    _ProfileHero(city: city, imageAsset: imageAsset),
                    const SizedBox(height: AppSpacing.lg),
                    _PetIdentity(
                      petName: petName,
                      petType: petType,
                      ownerName: ownerName,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    _ProfileButtons(
                      onEdit: () => context.push(AppRoutes.editProfile),
                      onShare: () => _shareProfile(petName),
                    ),
                    const SizedBox(height: AppSpacing.xxl),
                    const _ProfileStats(),
                    const SizedBox(height: AppSpacing.xxl),
                    _AboutSection(petName: petName, interests: setup.interests),
                    const SizedBox(height: AppSpacing.xxl),
                    _OwnerCard(
                      ownerName: ownerName,
                      petName: petName,
                      petType: petType,
                      onTap: () => context.push(AppRoutes.pets),
                    ),
                    const SizedBox(height: AppSpacing.xxl),
                    _PremiumBanner(
                      petName: petName,
                      onTap: () => _showPremium(petName),
                    ),
                    const SizedBox(height: AppSpacing.xxxl),
                    _MediaTabs(
                      selectedIndex: _selectedMediaTab,
                      onSelected: (index) {
                        setState(() => _selectedMediaTab = index);
                      },
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    _MediaGrid(
                      assets: _mediaAssets,
                      selectedTab: _selectedMediaTab,
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ProfileTopBar extends StatelessWidget {
  const _ProfileTopBar({
    required this.onNotificationTap,
    required this.onSettingsTap,
  });

  final VoidCallback onNotificationTap;
  final VoidCallback onSettingsTap;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(child: Text('Profile', style: AppTextStyles.headingLarge)),
        IconButton(
          tooltip: 'Notifications',
          onPressed: onNotificationTap,
          icon: const Badge(
            smallSize: AppSpacing.sm,
            backgroundColor: AppColors.accent,
            child: Icon(Icons.notifications_none_rounded),
          ),
        ),
        IconButton(
          tooltip: 'Settings',
          onPressed: onSettingsTap,
          icon: const Icon(Icons.settings_outlined),
        ),
      ],
    );
  }
}

class _ProfileHero extends StatelessWidget {
  const _ProfileHero({required this.city, required this.imageAsset});

  final String city;
  final String imageAsset;

  @override
  Widget build(BuildContext context) {
    return AspectRatio(
      aspectRatio: 1.45,
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.card,
          borderRadius: AppRadius.xxl,
          boxShadow: AppShadows.card,
        ),
        child: ClipRRect(
          borderRadius: AppRadius.xxl,
          child: Stack(
            fit: StackFit.expand,
            children: [
              AppImage(
                imageAsset,
                fit: BoxFit.cover,
                alignment: Alignment.topCenter,
                cacheWidth: 1200,
                errorBuilder: (_, _, _) => const ColoredBox(
                  color: AppColors.elevatedSurface,
                  child: Icon(
                    Icons.pets_rounded,
                    size: 72,
                    color: AppColors.textDisabled,
                  ),
                ),
              ),
              const DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      AppColors.transparent,
                      AppColors.transparent,
                      Color(0xB3000000),
                    ],
                  ),
                ),
              ),
              Positioned(
                left: AppSpacing.lg,
                bottom: AppSpacing.lg,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppSpacing.md,
                    vertical: AppSpacing.sm,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.overlay,
                    borderRadius: AppRadius.pill,
                    border: Border.all(color: AppColors.glassBorder),
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: AppSpacing.sm,
                        height: AppSpacing.sm,
                        decoration: const BoxDecoration(
                          color: AppColors.success,
                          shape: BoxShape.circle,
                        ),
                      ),
                      const SizedBox(width: AppSpacing.sm),
                      Text(
                        'Active now',
                        style: AppTextStyles.labelSmall.copyWith(
                          color: AppColors.white,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
              Positioned(
                right: AppSpacing.lg,
                top: AppSpacing.lg,
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppSpacing.md,
                    vertical: AppSpacing.sm,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.overlay,
                    borderRadius: AppRadius.pill,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Icon(
                        Icons.location_on_rounded,
                        size: 16,
                        color: AppColors.white,
                      ),
                      const SizedBox(width: AppSpacing.xs),
                      Text(
                        city,
                        style: AppTextStyles.labelSmall.copyWith(
                          color: AppColors.white,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _PetIdentity extends StatelessWidget {
  const _PetIdentity({
    required this.petName,
    required this.petType,
    required this.ownerName,
  });

  final String petName;
  final String petType;
  final String ownerName;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Flexible(child: Text(petName, style: AppTextStyles.displaySmall)),
            const SizedBox(width: AppSpacing.sm),
            const Icon(Icons.verified_rounded, size: 22, color: AppColors.info),
          ],
        ),
        const SizedBox(height: AppSpacing.xs),
        Text(
          '$petType  •  PetConnect member',
          style: AppTextStyles.bodyMedium.copyWith(
            color: AppColors.textSecondary,
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        Row(
          children: [
            const CircleAvatar(
              radius: 12,
              backgroundColor: AppColors.primary,
              child: Icon(
                Icons.person_rounded,
                size: 15,
                color: AppColors.white,
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Text('Pet parent: ', style: AppTextStyles.bodySmall),
            Text(
              ownerName,
              style: AppTextStyles.labelMedium.copyWith(
                color: AppColors.primaryLight,
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _ProfileButtons extends StatelessWidget {
  const _ProfileButtons({required this.onEdit, required this.onShare});

  final VoidCallback onEdit;
  final VoidCallback onShare;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: FilledButton.icon(
            onPressed: onEdit,
            icon: const Icon(Icons.edit_outlined, size: 19),
            label: const Text('Edit profile'),
          ),
        ),
        const SizedBox(width: AppSpacing.md),
        SizedBox.square(
          dimension: AppSpacing.massive,
          child: IconButton.outlined(
            tooltip: 'Share profile',
            onPressed: onShare,
            icon: const Icon(Icons.ios_share_rounded, size: 20),
          ),
        ),
      ],
    );
  }
}

class _ProfileStats extends StatelessWidget {
  const _ProfileStats();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.sm,
        vertical: AppSpacing.lg,
      ),
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: AppRadius.lg,
        border: Border.all(color: AppColors.border),
      ),
      child: const Row(
        children: [
          Expanded(
            child: _Stat(value: '128', label: 'Posts'),
          ),
          _StatDivider(),
          Expanded(
            child: _Stat(value: '12.8K', label: 'Followers'),
          ),
          _StatDivider(),
          Expanded(
            child: _Stat(value: '486', label: 'Following'),
          ),
          _StatDivider(),
          Expanded(
            child: _Stat(value: '2', label: 'Pets'),
          ),
        ],
      ),
    );
  }
}

class _Stat extends StatelessWidget {
  const _Stat({required this.value, required this.label});

  final String value;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(value, maxLines: 1, style: AppTextStyles.labelLarge),
        const SizedBox(height: AppSpacing.xs),
        Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.fade,
          style: AppTextStyles.caption,
        ),
      ],
    );
  }
}

class _StatDivider extends StatelessWidget {
  const _StatDivider();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 1,
      height: AppSpacing.xxxl,
      color: AppColors.divider,
    );
  }
}

class _AboutSection extends StatelessWidget {
  const _AboutSection({required this.petName, required this.interests});

  final String petName;
  final List<String> interests;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('About $petName', style: AppTextStyles.headingSmall),
        const SizedBox(height: AppSpacing.sm),
        Text(
          '$petName is here to make new friends, discover welcoming places, '
          'and share the little moments that make pet life special. 🐾',
          style: AppTextStyles.bodyMedium.copyWith(
            color: AppColors.textSecondary,
          ),
        ),
        const SizedBox(height: AppSpacing.lg),
        Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children:
              (interests.isEmpty
                      ? const ['Playdates', 'Parks', 'Training']
                      : interests)
                  .take(4)
                  .map(
                    (interest) => _TraitChip(
                      icon: Icons.favorite_outline_rounded,
                      label: interest,
                    ),
                  )
                  .toList(growable: false),
        ),
      ],
    );
  }
}

class _TraitChip extends StatelessWidget {
  const _TraitChip({required this.icon, required this.label});

  final IconData icon;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.md,
        vertical: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: AppColors.primary.withValues(alpha: 0.12),
        borderRadius: AppRadius.pill,
        border: Border.all(color: AppColors.primary.withValues(alpha: 0.35)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: AppColors.primaryLight),
          const SizedBox(width: AppSpacing.sm),
          Text(label, style: AppTextStyles.labelSmall),
        ],
      ),
    );
  }
}

class _OwnerCard extends StatelessWidget {
  const _OwnerCard({
    required this.ownerName,
    required this.petName,
    required this.petType,
    required this.onTap,
  });

  final String ownerName;
  final String petName;
  final String petType;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: Ink(
          padding: const EdgeInsets.all(AppSpacing.lg),
          decoration: BoxDecoration(
            color: AppColors.surface,
            borderRadius: AppRadius.lg,
            border: Border.all(color: AppColors.border),
          ),
          child: Row(
            children: [
              Container(
                width: AppSpacing.massive,
                height: AppSpacing.massive,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.16),
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.person_rounded,
                  color: AppColors.primaryLight,
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '$ownerName’s pet family',
                      style: AppTextStyles.labelLarge,
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      '$petName • $petType • manage pets',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTextStyles.bodySmall,
                    ),
                  ],
                ),
              ),
              const Icon(
                Icons.chevron_right_rounded,
                color: AppColors.textSecondary,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _PremiumBanner extends StatelessWidget {
  const _PremiumBanner({required this.petName, required this.onTap});

  final String petName;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.xl,
        child: Ink(
          padding: const EdgeInsets.all(AppSpacing.xl),
          decoration: const BoxDecoration(
            gradient: AppColors.primaryGradient,
            borderRadius: AppRadius.xl,
          ),
          child: Row(
            children: [
              Container(
                width: AppSpacing.giant,
                height: AppSpacing.giant,
                decoration: BoxDecoration(
                  color: AppColors.white.withValues(alpha: 0.18),
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.auto_awesome_rounded,
                  color: AppColors.white,
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'PetConnect Premium',
                      style: AppTextStyles.labelLarge.copyWith(
                        color: AppColors.white,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      'Boost $petName and see who liked them.',
                      style: AppTextStyles.bodySmall.copyWith(
                        color: AppColors.white.withValues(alpha: 0.86),
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.arrow_forward_rounded, color: AppColors.white),
            ],
          ),
        ),
      ),
    );
  }
}

class _MediaTabs extends StatelessWidget {
  const _MediaTabs({required this.selectedIndex, required this.onSelected});

  final int selectedIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    const icons = [
      Icons.grid_on_rounded,
      Icons.movie_outlined,
      Icons.person_pin_outlined,
    ];
    const labels = ['Posts', 'Reels', 'Tagged'];

    return Row(
      children: List.generate(
        icons.length,
        (index) => Expanded(
          child: Semantics(
            selected: selectedIndex == index,
            button: true,
            label: labels[index],
            child: InkWell(
              onTap: () => onSelected(index),
              borderRadius: AppRadius.sm,
              child: AnimatedContainer(
                duration: const Duration(milliseconds: 180),
                padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
                decoration: BoxDecoration(
                  border: Border(
                    bottom: BorderSide(
                      width: 2,
                      color: selectedIndex == index
                          ? AppColors.primary
                          : AppColors.transparent,
                    ),
                  ),
                ),
                child: Icon(
                  icons[index],
                  color: selectedIndex == index
                      ? AppColors.primaryLight
                      : AppColors.textDisabled,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _MediaGrid extends StatelessWidget {
  const _MediaGrid({required this.assets, required this.selectedTab});

  final List<String> assets;
  final int selectedTab;

  @override
  Widget build(BuildContext context) {
    final visibleAssets = selectedTab == 0
        ? assets
        : selectedTab == 1
        ? assets.take(6).toList(growable: false)
        : assets.reversed.take(6).toList(growable: false);

    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      itemCount: visibleAssets.length,
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: AppSpacing.xs,
        mainAxisSpacing: AppSpacing.xs,
      ),
      itemBuilder: (context, index) => _MediaTile(
        imageAsset: visibleAssets[index],
        isReel: selectedTab == 1 || index == 1 || index == 5,
        isTagged: selectedTab == 2,
      ),
    );
  }
}

class _MediaTile extends StatelessWidget {
  const _MediaTile({
    required this.imageAsset,
    required this.isReel,
    required this.isTagged,
  });

  final String imageAsset;
  final bool isReel;
  final bool isTagged;

  @override
  Widget build(BuildContext context) {
    return ClipRRect(
      borderRadius: AppRadius.xs,
      child: Stack(
        fit: StackFit.expand,
        children: [
          AppImage(
            imageAsset,
            fit: BoxFit.cover,
            cacheWidth: 800,
            errorBuilder: (_, _, _) => const ColoredBox(
              color: AppColors.elevatedSurface,
              child: Icon(Icons.image_not_supported_outlined),
            ),
          ),
          if (isReel)
            const Positioned(
              top: AppSpacing.sm,
              right: AppSpacing.sm,
              child: Icon(
                Icons.play_circle_fill_rounded,
                color: AppColors.white,
                size: 20,
              ),
            ),
          if (isTagged)
            const Positioned(
              bottom: AppSpacing.sm,
              left: AppSpacing.sm,
              child: Icon(
                Icons.person_pin_rounded,
                color: AppColors.white,
                size: 20,
              ),
            ),
        ],
      ),
    );
  }
}

class _PremiumSheet extends StatelessWidget {
  const _PremiumSheet({required this.petName, required this.onContinue});

  final String petName;
  final VoidCallback onContinue;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.xxl,
          AppSpacing.sm,
          AppSpacing.xxl,
          AppSpacing.xxl,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: AppSpacing.section,
              height: AppSpacing.section,
              decoration: const BoxDecoration(
                gradient: AppColors.primaryGradient,
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.workspace_premium_rounded,
                size: 32,
                color: AppColors.white,
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(
              'Make every connection count',
              textAlign: TextAlign.center,
              style: AppTextStyles.headingMedium,
            ),
            const SizedBox(height: AppSpacing.sm),
            Text(
              'Get unlimited likes, advanced filters, profile boosts, and '
              'see everyone who wants to meet $petName.',
              textAlign: TextAlign.center,
              style: AppTextStyles.bodyMedium.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
            const SizedBox(height: AppSpacing.xxl),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: onContinue,
                child: const Text('Explore Premium'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
