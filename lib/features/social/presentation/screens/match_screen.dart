import 'dart:math' as math;

import 'package:flutter/material.dart';
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
import '../../application/social_controller.dart';
import '../../data/mock_social_data.dart';
import '../../domain/social_models.dart';

class MatchScreen extends ConsumerStatefulWidget {
  const MatchScreen({super.key});

  @override
  ConsumerState<MatchScreen> createState() => _MatchScreenState();
}

class _MatchScreenState extends ConsumerState<MatchScreen> {
  static const double _swipeThreshold = 88;
  static const Duration _swipeDuration = Duration(milliseconds: 220);

  Offset _dragOffset = Offset.zero;
  bool _isAnimating = false;
  bool _isShowingMatch = false;

  Future<void> _react({
    required bool interested,
    bool superLike = false,
  }) async {
    if (_isAnimating) return;

    final width = MediaQuery.sizeOf(context).width;
    final targetX = interested ? width * 1.35 : -width * 1.35;

    setState(() {
      _isAnimating = true;
      _dragOffset = Offset(targetX, _dragOffset.dy);
    });

    await Future<void>.delayed(_swipeDuration);
    if (!mounted) return;

    ref
        .read(socialControllerProvider.notifier)
        .reactToCandidate(interested: interested, superLike: superLike);

    setState(() {
      _isAnimating = false;
      _dragOffset = Offset.zero;
    });
  }

  void _onPanUpdate(DragUpdateDetails details) {
    if (_isAnimating) return;
    setState(() => _dragOffset += details.delta);
  }

  void _onPanEnd(DragEndDetails details) {
    if (_isAnimating) return;
    if (_dragOffset.dx.abs() >= _swipeThreshold) {
      _react(interested: _dragOffset.dx > 0);
      return;
    }
    setState(() => _dragOffset = Offset.zero);
  }

  Future<void> _showMatch(PetProfile pet) async {
    if (_isShowingMatch || !mounted) return;
    _isShowingMatch = true;

    final state = ref.read(socialControllerProvider);
    final matchedChatId = state.lastMatchChatId;
    final useCompanion = state.activePetIndex == 1;
    final companionIsLuna = state.userProfile.petName.toLowerCase() == 'simba';
    final activePetName = useCompanion
        ? (companionIsLuna ? 'Luna' : 'Simba')
        : state.userProfile.petName;
    final activePetImage = state.pets.isNotEmpty
        ? state
              .pets[state.activePetIndex < state.pets.length
                  ? state.activePetIndex
                  : 0]
              .imageAsset
        : useCompanion
        ? (companionIsLuna ? MockSocialData.golden : MockSocialData.cat)
        : state.userProfile.petType.toLowerCase() == 'cat'
        ? MockSocialData.cat
        : MockSocialData.golden;

    final openChat = await showDialog<bool>(
      context: context,
      barrierColor: AppColors.overlayHeavy,
      builder: (dialogContext) => _MatchCelebrationDialog(
        pet: pet,
        activePetName: activePetName,
        activePetImage: activePetImage,
      ),
    );

    if (!mounted) return;
    ref.read(socialControllerProvider.notifier).dismissMatch();
    _isShowingMatch = false;

    if (openChat ?? false) {
      context.push(AppRoutes.chat(matchedChatId ?? 'chat-${pet.id}'));
    }
  }

  void _showFilters() {
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (sheetContext) => const _DiscoveryFiltersSheet(),
    );
  }

  Future<void> _showSuperLike() async {
    final confirmed = await showModalBottomSheet<bool>(
      context: context,
      showDragHandle: true,
      builder: (sheetContext) => SafeArea(
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
              const Icon(Icons.star_rounded, size: 48, color: AppColors.info),
              const SizedBox(height: AppSpacing.md),
              Text(
                'Stand out with a Super Like',
                style: AppTextStyles.headingSmall,
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                'Super Likes are a Premium feature. Use the demo credit to show this pet you’re especially interested.',
                textAlign: TextAlign.center,
                style: AppTextStyles.bodyMedium.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
              const SizedBox(height: AppSpacing.xl),
              SizedBox(
                width: double.infinity,
                child: FilledButton.icon(
                  onPressed: () => Navigator.of(sheetContext).pop(true),
                  icon: const Icon(Icons.star_rounded),
                  label: const Text('Use demo Super Like'),
                ),
              ),
            ],
          ),
        ),
      ),
    );
    if ((confirmed ?? false) && mounted) {
      await _react(interested: true, superLike: true);
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen<PetProfile?>(
      socialControllerProvider.select((state) => state.lastMatch),
      (previous, next) {
        if (next == null || next.id == previous?.id) return;
        WidgetsBinding.instance.addPostFrameCallback((_) => _showMatch(next));
      },
    );

    final socialState = ref.watch(socialControllerProvider);
    final candidates = socialState.candidates;
    if (candidates.isEmpty || socialState.candidateIndex >= candidates.length) {
      return AppScaffold(
        child: _DeckComplete(
          onRefresh: () =>
              ref.read(socialControllerProvider.notifier).resetMatchDeck(),
        ),
      );
    }

    final current = candidates[socialState.candidateIndex];
    final next = socialState.candidateIndex + 1 < candidates.length
        ? candidates[socialState.candidateIndex + 1]
        : null;

    return AppScaffold(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final cardHeight = math.min(
            constraints.maxHeight * 0.68,
            constraints.maxWidth * 1.3,
          );

          return Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 560),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(
                  AppSpacing.lg,
                  AppSpacing.sm,
                  AppSpacing.lg,
                  AppSpacing.md,
                ),
                child: Column(
                  children: [
                    _MatchHeader(
                      city: socialState.userProfile.city,
                      onFilterTap: _showFilters,
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    Expanded(
                      child: Center(
                        child: SizedBox(
                          height: cardHeight,
                          child: Stack(
                            alignment: Alignment.center,
                            children: [
                              if (next != null)
                                Positioned.fill(
                                  top: AppSpacing.md,
                                  left: AppSpacing.md,
                                  right: AppSpacing.md,
                                  child: _PetMatchCard(
                                    pet: next,
                                    isPreview: true,
                                  ),
                                ),
                              Positioned.fill(
                                child: GestureDetector(
                                  key: ValueKey('match-card-${current.id}'),
                                  onPanUpdate: _onPanUpdate,
                                  onPanEnd: _onPanEnd,
                                  child: AnimatedContainer(
                                    duration: _isAnimating
                                        ? _swipeDuration
                                        : const Duration(milliseconds: 180),
                                    curve: Curves.easeOutCubic,
                                    transformAlignment: Alignment.center,
                                    transform: Matrix4.identity()
                                      ..translateByDouble(
                                        _dragOffset.dx,
                                        _dragOffset.dy * 0.24,
                                        0,
                                        1,
                                      )
                                      ..rotateZ(_dragOffset.dx / 950),
                                    child: _PetMatchCard(
                                      pet: current,
                                      swipeOffset: _dragOffset.dx,
                                    ),
                                  ),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.lg),
                    _MatchActions(
                      enabled: !_isAnimating,
                      onSkip: () => _react(interested: false),
                      onSuperLike: _showSuperLike,
                      onLike: () => _react(interested: true),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Text(
                      'Swipe right to connect',
                      style: AppTextStyles.caption,
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

class _DeckComplete extends StatelessWidget {
  const _DeckComplete({required this.onRefresh});

  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(AppSpacing.xxl),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 420),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                width: 88,
                height: 88,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.14),
                  shape: BoxShape.circle,
                ),
                child: const Icon(
                  Icons.explore_rounded,
                  size: 42,
                  color: AppColors.primaryLight,
                ),
              ),
              const SizedBox(height: AppSpacing.xxl),
              Text(
                'You’ve met everyone nearby',
                textAlign: TextAlign.center,
                style: AppTextStyles.headingLarge,
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                'Check back later for new pets, or refresh the demo deck to keep exploring.',
                textAlign: TextAlign.center,
                style: AppTextStyles.bodyMedium.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
              const SizedBox(height: AppSpacing.xxl),
              FilledButton.icon(
                onPressed: onRefresh,
                icon: const Icon(Icons.refresh_rounded),
                label: const Text('Refresh suggestions'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MatchHeader extends StatelessWidget {
  const _MatchHeader({required this.city, required this.onFilterTap});

  final String city;
  final VoidCallback onFilterTap;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Find a friend', style: AppTextStyles.headingLarge),
              const SizedBox(height: AppSpacing.xs),
              Row(
                children: [
                  const Icon(
                    Icons.location_on_rounded,
                    size: 16,
                    color: AppColors.primaryLight,
                  ),
                  const SizedBox(width: AppSpacing.xs),
                  Expanded(
                    child: Text(
                      '$city  •  within 10 km',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTextStyles.bodySmall,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
        IconButton.filledTonal(
          tooltip: 'Discovery filters',
          onPressed: onFilterTap,
          style: IconButton.styleFrom(
            backgroundColor: AppColors.card,
            foregroundColor: AppColors.textPrimary,
            side: const BorderSide(color: AppColors.border),
          ),
          icon: const Icon(Icons.tune_rounded),
        ),
      ],
    );
  }
}

class _PetMatchCard extends StatelessWidget {
  const _PetMatchCard({
    required this.pet,
    this.swipeOffset = 0,
    this.isPreview = false,
  });

  final PetProfile pet;
  final double swipeOffset;
  final bool isPreview;

  @override
  Widget build(BuildContext context) {
    final likeOpacity = (swipeOffset / 100).clamp(0.0, 1.0);
    final skipOpacity = (-swipeOffset / 100).clamp(0.0, 1.0);

    return Semantics(
      label: '${pet.name}, ${pet.breed}, ${pet.age}',
      image: true,
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.card,
          borderRadius: AppRadius.xxl,
          border: Border.all(color: AppColors.glassBorder),
          boxShadow: isPreview ? AppShadows.md : AppShadows.floating,
        ),
        foregroundDecoration: isPreview
            ? BoxDecoration(
                color: AppColors.background.withValues(alpha: 0.42),
                borderRadius: AppRadius.xxl,
              )
            : null,
        child: ClipRRect(
          borderRadius: AppRadius.xxl,
          child: Stack(
            fit: StackFit.expand,
            children: [
              AppImage(
                pet.imageAsset,
                fit: BoxFit.cover,
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
                    stops: [0, 0.48, 1],
                    colors: [
                      AppColors.transparent,
                      AppColors.transparent,
                      Color(0xE6000000),
                    ],
                  ),
                ),
              ),
              Positioned(
                top: AppSpacing.lg,
                left: AppSpacing.lg,
                child: Opacity(
                  opacity: likeOpacity,
                  child: const _SwipeStamp(
                    label: 'LET\'S PLAY',
                    color: AppColors.success,
                    angle: -0.12,
                  ),
                ),
              ),
              Positioned(
                top: AppSpacing.lg,
                right: AppSpacing.lg,
                child: Opacity(
                  opacity: skipOpacity,
                  child: const _SwipeStamp(
                    label: 'MAYBE LATER',
                    color: AppColors.accent,
                    angle: 0.12,
                  ),
                ),
              ),
              Positioned(
                top: AppSpacing.lg,
                right: AppSpacing.lg,
                child: Opacity(
                  opacity: 1 - math.max(likeOpacity, skipOpacity),
                  child: _DistanceBadge(pet: pet),
                ),
              ),
              Positioned(
                left: AppSpacing.xl,
                right: AppSpacing.xl,
                bottom: AppSpacing.xl,
                child: _PetCardDetails(pet: pet),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _DistanceBadge extends StatelessWidget {
  const _DistanceBadge({required this.pet});

  final PetProfile pet;

  @override
  Widget build(BuildContext context) {
    return Container(
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
          if (pet.isOnline) ...[
            Container(
              width: AppSpacing.sm,
              height: AppSpacing.sm,
              decoration: const BoxDecoration(
                color: AppColors.success,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
          ],
          Text(
            '${pet.distanceKm.toStringAsFixed(1)} km away',
            style: AppTextStyles.labelSmall.copyWith(color: AppColors.white),
          ),
        ],
      ),
    );
  }
}

class _PetCardDetails extends StatelessWidget {
  const _PetCardDetails({required this.pet});

  final PetProfile pet;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Flexible(
              child: Text(
                '${pet.name}, ${pet.age.split(' ').first}',
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.displaySmall,
              ),
            ),
            if (pet.isVerified) ...[
              const SizedBox(width: AppSpacing.sm),
              const Icon(
                Icons.verified_rounded,
                color: AppColors.info,
                size: 22,
              ),
            ],
          ],
        ),
        const SizedBox(height: AppSpacing.xs),
        Text(
          '${pet.breed}  •  ${pet.gender}  •  with ${pet.ownerName}',
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: AppTextStyles.bodySmall.copyWith(color: AppColors.white),
        ),
        const SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children: pet.personality
              .map(
                (trait) => Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: AppSpacing.md,
                    vertical: AppSpacing.xs,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.white.withValues(alpha: 0.14),
                    borderRadius: AppRadius.pill,
                    border: Border.all(color: AppColors.glassBorder),
                  ),
                  child: Text(
                    trait,
                    style: AppTextStyles.labelSmall.copyWith(
                      color: AppColors.white,
                    ),
                  ),
                ),
              )
              .toList(growable: false),
        ),
      ],
    );
  }
}

class _SwipeStamp extends StatelessWidget {
  const _SwipeStamp({
    required this.label,
    required this.color,
    required this.angle,
  });

  final String label;
  final Color color;
  final double angle;

  @override
  Widget build(BuildContext context) {
    return Transform.rotate(
      angle: angle,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.md,
          vertical: AppSpacing.sm,
        ),
        decoration: BoxDecoration(
          border: Border.all(color: color, width: 3),
          borderRadius: AppRadius.sm,
        ),
        child: Text(
          label,
          style: AppTextStyles.labelLarge.copyWith(color: color),
        ),
      ),
    );
  }
}

class _MatchActions extends StatelessWidget {
  const _MatchActions({
    required this.enabled,
    required this.onSkip,
    required this.onSuperLike,
    required this.onLike,
  });

  final bool enabled;
  final VoidCallback onSkip;
  final VoidCallback onSuperLike;
  final VoidCallback onLike;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        _RoundActionButton(
          tooltip: 'Skip',
          icon: Icons.close_rounded,
          color: AppColors.accent,
          onPressed: enabled ? onSkip : null,
        ),
        const SizedBox(width: AppSpacing.xl),
        _RoundActionButton(
          tooltip: 'Super like',
          icon: Icons.star_rounded,
          color: AppColors.info,
          compact: true,
          onPressed: enabled ? onSuperLike : null,
        ),
        const SizedBox(width: AppSpacing.xl),
        _RoundActionButton(
          tooltip: 'Like',
          icon: Icons.favorite_rounded,
          color: AppColors.success,
          onPressed: enabled ? onLike : null,
        ),
      ],
    );
  }
}

class _RoundActionButton extends StatelessWidget {
  const _RoundActionButton({
    required this.tooltip,
    required this.icon,
    required this.color,
    required this.onPressed,
    this.compact = false,
  });

  final String tooltip;
  final IconData icon;
  final Color color;
  final VoidCallback? onPressed;
  final bool compact;

  @override
  Widget build(BuildContext context) {
    final dimension = compact ? AppSpacing.giant : AppSpacing.massive;
    return Tooltip(
      message: tooltip,
      child: DecoratedBox(
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          boxShadow: AppShadows.glow(color, blur: 18, spread: 0),
        ),
        child: SizedBox.square(
          dimension: dimension,
          child: IconButton(
            onPressed: onPressed,
            style: IconButton.styleFrom(
              backgroundColor: AppColors.card,
              foregroundColor: color,
              disabledForegroundColor: AppColors.textDisabled,
              side: BorderSide(color: color.withValues(alpha: 0.55)),
            ),
            icon: Icon(icon, size: compact ? 24 : 28),
          ),
        ),
      ),
    );
  }
}

class _MatchCelebrationDialog extends StatelessWidget {
  const _MatchCelebrationDialog({
    required this.pet,
    required this.activePetName,
    required this.activePetImage,
  });

  final PetProfile pet;
  final String activePetName;
  final String activePetImage;

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: AppColors.transparent,
      insetPadding: const EdgeInsets.all(AppSpacing.xl),
      child: Container(
        constraints: const BoxConstraints(maxWidth: 420),
        padding: const EdgeInsets.all(AppSpacing.xxl),
        decoration: BoxDecoration(
          color: AppColors.card,
          borderRadius: AppRadius.xxl,
          border: Border.all(color: AppColors.glassBorder),
          boxShadow: AppShadows.primaryGlow,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('🐾', style: AppTextStyles.displayMedium),
            const SizedBox(height: AppSpacing.sm),
            Text(
              'It’s a match!',
              textAlign: TextAlign.center,
              style: AppTextStyles.displaySmall.copyWith(
                color: AppColors.primaryLight,
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
            Text(
              '$activePetName and ${pet.name} both want to be friends.',
              textAlign: TextAlign.center,
              style: AppTextStyles.bodyMedium.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
            const SizedBox(height: AppSpacing.xxl),
            SizedBox(
              height: 104,
              child: Stack(
                alignment: Alignment.center,
                children: [
                  Transform.translate(
                    offset: const Offset(-42, 0),
                    child: _MatchAvatar(
                      imageAsset: activePetImage,
                      label: activePetName,
                    ),
                  ),
                  Transform.translate(
                    offset: const Offset(42, 0),
                    child: _MatchAvatar(
                      imageAsset: pet.imageAsset,
                      label: pet.name,
                    ),
                  ),
                  Container(
                    width: AppSpacing.giant,
                    height: AppSpacing.giant,
                    decoration: const BoxDecoration(
                      color: AppColors.primary,
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(
                      Icons.favorite_rounded,
                      color: AppColors.white,
                      size: 24,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.xxl),
            SizedBox(
              width: double.infinity,
              child: FilledButton.icon(
                onPressed: () => Navigator.of(context).pop(true),
                icon: const Icon(Icons.chat_bubble_rounded),
                label: const Text('Say hello'),
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
            TextButton(
              onPressed: () => Navigator.of(context).pop(false),
              child: const Text('Keep discovering'),
            ),
          ],
        ),
      ),
    );
  }
}

class _MatchAvatar extends StatelessWidget {
  const _MatchAvatar({required this.imageAsset, required this.label});

  final String imageAsset;
  final String label;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: label,
      image: true,
      child: Container(
        width: 96,
        height: 96,
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          border: Border.all(color: AppColors.primaryLight, width: 3),
          image: DecorationImage(
            image: ResizeImage(AssetImage(imageAsset), width: 180),
            fit: BoxFit.cover,
          ),
        ),
      ),
    );
  }
}

class _DiscoveryFiltersSheet extends StatefulWidget {
  const _DiscoveryFiltersSheet();

  @override
  State<_DiscoveryFiltersSheet> createState() => _DiscoveryFiltersSheetState();
}

class _DiscoveryFiltersSheetState extends State<_DiscoveryFiltersSheet> {
  double _distance = 10;
  String _petType = 'Dogs';

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
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('Discovery preferences', style: AppTextStyles.headingMedium),
            const SizedBox(height: AppSpacing.xxl),
            Text('Pet type', style: AppTextStyles.labelMedium),
            const SizedBox(height: AppSpacing.sm),
            Wrap(
              spacing: AppSpacing.sm,
              children: ['Dogs', 'Cats', 'All pets']
                  .map(
                    (type) => ChoiceChip(
                      label: Text(type),
                      selected: _petType == type,
                      onSelected: (_) => setState(() => _petType = type),
                    ),
                  )
                  .toList(growable: false),
            ),
            const SizedBox(height: AppSpacing.xxl),
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Maximum distance',
                    style: AppTextStyles.labelMedium,
                  ),
                ),
                Text(
                  '${_distance.round()} km',
                  style: AppTextStyles.labelMedium.copyWith(
                    color: AppColors.primaryLight,
                  ),
                ),
              ],
            ),
            Slider(
              value: _distance,
              min: 2,
              max: 50,
              divisions: 24,
              label: '${_distance.round()} km',
              onChanged: (value) => setState(() => _distance = value),
            ),
            const SizedBox(height: AppSpacing.md),
            SizedBox(
              width: double.infinity,
              child: FilledButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Apply filters'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
