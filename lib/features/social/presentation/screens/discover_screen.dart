import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_shadows.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../data/mock_social_data.dart';
import '../../domain/social_models.dart';

class DiscoverScreen extends StatefulWidget {
  const DiscoverScreen({super.key});

  @override
  State<DiscoverScreen> createState() => _DiscoverScreenState();
}

class _DiscoverScreenState extends State<DiscoverScreen> {
  static const _filters = <String>[
    'For you',
    'Dogs',
    'Cats',
    'Under 3 km',
    'Female',
  ];

  static const _adoptionPets = <_AdoptionPet>[
    _AdoptionPet(
      id: 'adopt-olive',
      name: 'Olive',
      breed: 'Domestic shorthair',
      age: '11 months',
      location: 'Al Quoz · 4.2 km',
      imageAsset: MockSocialData.cat,
    ),
    _AdoptionPet(
      id: 'adopt-teddy',
      name: 'Teddy',
      breed: 'Golden retriever mix',
      age: '2 years',
      location: 'JVC · 6.8 km',
      imageAsset: MockSocialData.golden,
    ),
  ];

  final TextEditingController _searchController = TextEditingController();
  final Set<String> _favoritePetIds = <String>{};
  final Set<String> _joinedCommunities = <String>{};
  final Set<String> _rsvpedEvents = <String>{};
  final Set<String> _savedAdoptions = <String>{};

  String _activeFilter = _filters.first;
  String _query = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  bool _containsQuery(Iterable<String> values) {
    final query = _query.trim().toLowerCase();
    if (query.isEmpty) return true;
    return values.any((value) => value.toLowerCase().contains(query));
  }

  List<PetProfile> get _visiblePets {
    return MockSocialData.matchCandidates
        .where((pet) {
          final breed = pet.breed.toLowerCase();
          final isCat =
              breed.contains('cat') ||
              breed.contains('shorthair') ||
              breed.contains('persian');

          final passesFilter = switch (_activeFilter) {
            'Dogs' => !isCat,
            'Cats' => isCat,
            'Under 3 km' => pet.distanceKm < 3,
            'Female' => pet.gender.toLowerCase() == 'female',
            _ => true,
          };

          return passesFilter &&
              _containsQuery([
                pet.name,
                pet.ownerName,
                pet.breed,
                pet.gender,
                ...pet.personality,
              ]);
        })
        .toList(growable: false);
  }

  List<CommunityPreview> get _visibleCommunities {
    return MockSocialData.communities
        .where(
          (community) => _containsQuery([community.name, community.members]),
        )
        .toList(growable: false);
  }

  List<EventPreview> get _visibleEvents {
    return MockSocialData.events
        .where(
          (event) =>
              _containsQuery([event.title, event.location, event.dateLabel]),
        )
        .toList(growable: false);
  }

  List<_AdoptionPet> get _visibleAdoptions {
    return _adoptionPets
        .where(
          (pet) => _containsQuery([
            pet.name,
            pet.breed,
            pet.age,
            pet.location,
            'adoption',
          ]),
        )
        .toList(growable: false);
  }

  void _showFeedback(String message, {IconData icon = Icons.check_circle}) {
    final messenger = ScaffoldMessenger.of(context);
    messenger
      ..hideCurrentSnackBar()
      ..showSnackBar(
        SnackBar(
          content: Row(
            children: [
              Icon(icon, color: AppColors.white, size: AppSpacing.xl),
              const SizedBox(width: AppSpacing.md),
              Expanded(child: Text(message)),
            ],
          ),
        ),
      );
  }

  void _toggleFavorite(PetProfile pet) {
    setState(() {
      if (!_favoritePetIds.add(pet.id)) {
        _favoritePetIds.remove(pet.id);
      }
    });
    final isFavorite = _favoritePetIds.contains(pet.id);
    _showFeedback(
      isFavorite
          ? '${pet.name} was added to your favorites.'
          : '${pet.name} was removed from favorites.',
      icon: isFavorite ? Icons.favorite : Icons.favorite_border,
    );
  }

  void _toggleCommunity(CommunityPreview community) {
    setState(() {
      if (!_joinedCommunities.add(community.name)) {
        _joinedCommunities.remove(community.name);
      }
    });
    final joined = _joinedCommunities.contains(community.name);
    _showFeedback(
      joined ? 'Welcome to ${community.name}!' : 'You left ${community.name}.',
      icon: joined ? Icons.groups_rounded : Icons.logout_rounded,
    );
  }

  void _toggleRsvp(EventPreview event) {
    setState(() {
      if (!_rsvpedEvents.add(event.title)) {
        _rsvpedEvents.remove(event.title);
      }
    });
    final isGoing = _rsvpedEvents.contains(event.title);
    _showFeedback(
      isGoing
          ? "You're going to ${event.title}."
          : 'RSVP removed for ${event.title}.',
      icon: isGoing ? Icons.event_available : Icons.event_busy,
    );
  }

  void _toggleAdoption(_AdoptionPet pet) {
    setState(() {
      if (!_savedAdoptions.add(pet.id)) {
        _savedAdoptions.remove(pet.id);
      }
    });
    final isSaved = _savedAdoptions.contains(pet.id);
    _showFeedback(
      isSaved
          ? '${pet.name} was added to your adoption shortlist.'
          : '${pet.name} was removed from your shortlist.',
      icon: isSaved ? Icons.bookmark : Icons.bookmark_border,
    );
  }

  @override
  Widget build(BuildContext context) {
    final pets = _visiblePets;
    final communities = _visibleCommunities;
    final events = _visibleEvents;
    final adoptions = _visibleAdoptions;

    return Scaffold(
      backgroundColor: AppColors.background,
      body: SafeArea(
        bottom: false,
        child: LayoutBuilder(
          builder: (context, constraints) {
            final constrainedWidth = constraints.maxWidth > 980
                ? 980.0
                : constraints.maxWidth;
            final contentWidth = constrainedWidth - (AppSpacing.lg * 2);
            return Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 980),
                child: CustomScrollView(
                  keyboardDismissBehavior:
                      ScrollViewKeyboardDismissBehavior.onDrag,
                  slivers: [
                    SliverPadding(
                      padding: const EdgeInsets.fromLTRB(
                        AppSpacing.lg,
                        AppSpacing.lg,
                        AppSpacing.lg,
                        AppSpacing.section,
                      ),
                      sliver: SliverList.list(
                        children: [
                          _buildHeader(),
                          const SizedBox(height: AppSpacing.xxl),
                          _buildSearchField(),
                          const SizedBox(height: AppSpacing.lg),
                          _buildFilters(),
                          const SizedBox(height: AppSpacing.xxl),
                          _buildNearbyBanner(),
                          const SizedBox(height: AppSpacing.xxxl),
                          _SectionHeader(
                            title: 'Pets near you',
                            subtitle: pets.isEmpty
                                ? 'Try another filter'
                                : '${pets.length} great matches nearby',
                            actionLabel: 'Map',
                            onAction: () => _showFeedback(
                              'Map preview centered on Downtown Dubai.',
                              icon: Icons.map_rounded,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          _buildPetList(pets, contentWidth),
                          const SizedBox(height: AppSpacing.xxxl),
                          const _SectionHeader(
                            title: 'Communities',
                            subtitle: 'Find your people — and their pets',
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          _buildCommunityList(communities, contentWidth),
                          const SizedBox(height: AppSpacing.xxxl),
                          const _SectionHeader(
                            title: 'Happening nearby',
                            subtitle: 'Make this week one to remember',
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          _buildEvents(events, contentWidth),
                          const SizedBox(height: AppSpacing.xxxl),
                          _SectionHeader(
                            title: 'Looking for a home',
                            subtitle: 'Your new best friend may be right here',
                            actionLabel: 'Adoption guide',
                            onAction: () => _showFeedback(
                              'Adoption guide saved for later.',
                              icon: Icons.volunteer_activism_rounded,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.lg),
                          _buildAdoptions(adoptions, contentWidth),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _buildHeader() {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Discover', style: AppTextStyles.headingLarge),
              const SizedBox(height: AppSpacing.xs),
              Text(
                'New friends are closer than you think.',
                style: AppTextStyles.bodySmall,
              ),
            ],
          ),
        ),
        Material(
          color: AppColors.surface,
          shape: const StadiumBorder(side: BorderSide(color: AppColors.border)),
          child: InkWell(
            customBorder: const StadiumBorder(),
            onTap: () => _showFeedback(
              'Discovering around Downtown Dubai.',
              icon: Icons.near_me_rounded,
            ),
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: AppSpacing.md,
                vertical: AppSpacing.sm,
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Icon(
                    Icons.location_on_rounded,
                    color: AppColors.primaryLight,
                    size: AppSpacing.xl,
                  ),
                  const SizedBox(width: AppSpacing.xs),
                  Text('Dubai', style: AppTextStyles.labelMedium),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildSearchField() {
    return TextField(
      controller: _searchController,
      textInputAction: TextInputAction.search,
      onChanged: (value) => setState(() => _query = value),
      decoration: InputDecoration(
        hintText: 'Search pets, groups, events…',
        prefixIcon: const Icon(
          Icons.search_rounded,
          color: AppColors.textSecondary,
        ),
        suffixIcon: _query.isEmpty
            ? null
            : IconButton(
                tooltip: 'Clear search',
                onPressed: () {
                  _searchController.clear();
                  setState(() => _query = '');
                },
                icon: const Icon(
                  Icons.close_rounded,
                  color: AppColors.textSecondary,
                ),
              ),
      ),
    );
  }

  Widget _buildFilters() {
    return SizedBox(
      height: AppSpacing.huge,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: _filters.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final filter = _filters[index];
          final isSelected = filter == _activeFilter;
          return ChoiceChip(
            label: Text(filter),
            selected: isSelected,
            showCheckmark: false,
            onSelected: (_) => setState(() => _activeFilter = filter),
            backgroundColor: AppColors.surface,
            selectedColor: AppColors.primary,
            side: BorderSide(
              color: isSelected ? AppColors.primary : AppColors.border,
            ),
            shape: const StadiumBorder(),
            labelStyle: AppTextStyles.labelMedium.copyWith(
              color: isSelected ? AppColors.white : AppColors.textSecondary,
            ),
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm),
          );
        },
      ),
    );
  }

  Widget _buildNearbyBanner() {
    return DecoratedBox(
      decoration: BoxDecoration(
        gradient: AppColors.primaryGradient,
        borderRadius: AppRadius.xl,
        boxShadow: AppShadows.colored(AppColors.primary),
      ),
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Ready for a playdate?',
                    style: AppTextStyles.headingSmall,
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text(
                    '12 friendly pets are active around you.',
                    style: AppTextStyles.bodySmall.copyWith(
                      color: AppColors.white.withValues(alpha: 0.84),
                    ),
                  ),
                  const SizedBox(height: AppSpacing.lg),
                  FilledButton.tonalIcon(
                    onPressed: () => _showFeedback(
                      'Showing pets available for a playdate.',
                      icon: Icons.pets_rounded,
                    ),
                    style: FilledButton.styleFrom(
                      backgroundColor: AppColors.white,
                      foregroundColor: AppColors.primaryDark,
                      textStyle: AppTextStyles.buttonMedium,
                    ),
                    icon: const Icon(Icons.near_me_rounded),
                    label: const Text('Explore now'),
                  ),
                ],
              ),
            ),
            const SizedBox(width: AppSpacing.lg),
            Container(
              width: AppSpacing.massive,
              height: AppSpacing.massive,
              decoration: BoxDecoration(
                color: AppColors.white.withValues(alpha: 0.16),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.pets_rounded,
                color: AppColors.white,
                size: AppSpacing.xxxl,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPetList(List<PetProfile> pets, double availableWidth) {
    if (pets.isEmpty) {
      return const _EmptyResult(
        icon: Icons.pets_outlined,
        message: 'No pets match this search yet.',
      );
    }

    final cardWidth = (availableWidth * (availableWidth >= 720 ? 0.31 : 0.76))
        .clamp(236.0, 304.0)
        .toDouble();

    return SizedBox(
      height: 286,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: pets.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.md),
        itemBuilder: (context, index) {
          final pet = pets[index];
          final isFavorite = _favoritePetIds.contains(pet.id);
          return SizedBox(
            width: cardWidth,
            child: Material(
              color: AppColors.card,
              borderRadius: AppRadius.lg,
              clipBehavior: Clip.antiAlias,
              child: InkWell(
                onTap: () => _showFeedback(
                  '${pet.name} is ${pet.distanceKm.toStringAsFixed(1)} km away.',
                  icon: Icons.pets_rounded,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Expanded(
                      child: Stack(
                        fit: StackFit.expand,
                        children: [
                          AppImage(
                            pet.imageAsset,
                            fit: BoxFit.cover,
                            cacheWidth: 900,
                            errorBuilder: (_, _, _) => const ColoredBox(
                              color: AppColors.elevatedSurface,
                              child: Icon(
                                Icons.pets_rounded,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                          DecoratedBox(
                            decoration: BoxDecoration(
                              gradient: LinearGradient(
                                begin: Alignment.topCenter,
                                end: Alignment.bottomCenter,
                                colors: [
                                  AppColors.overlayLight,
                                  AppColors.transparent,
                                ],
                              ),
                            ),
                          ),
                          Positioned(
                            top: AppSpacing.md,
                            left: AppSpacing.md,
                            child: _DistanceBadge(distance: pet.distanceKm),
                          ),
                          Positioned(
                            top: AppSpacing.sm,
                            right: AppSpacing.sm,
                            child: IconButton.filledTonal(
                              tooltip: isFavorite
                                  ? 'Remove from favorites'
                                  : 'Add to favorites',
                              onPressed: () => _toggleFavorite(pet),
                              style: IconButton.styleFrom(
                                backgroundColor: AppColors.overlay,
                                foregroundColor: isFavorite
                                    ? AppColors.accent
                                    : AppColors.white,
                              ),
                              icon: Icon(
                                isFavorite
                                    ? Icons.favorite_rounded
                                    : Icons.favorite_border_rounded,
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                    Padding(
                      padding: const EdgeInsets.all(AppSpacing.md),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Flexible(
                                child: Text(
                                  pet.name,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTextStyles.headingSmall,
                                ),
                              ),
                              if (pet.isVerified) ...[
                                const SizedBox(width: AppSpacing.xs),
                                const Icon(
                                  Icons.verified_rounded,
                                  color: AppColors.info,
                                  size: AppSpacing.lg,
                                ),
                              ],
                              if (pet.isOnline) ...[
                                const Spacer(),
                                Container(
                                  width: AppSpacing.sm,
                                  height: AppSpacing.sm,
                                  decoration: const BoxDecoration(
                                    color: AppColors.success,
                                    shape: BoxShape.circle,
                                  ),
                                ),
                              ],
                            ],
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            '${pet.breed} · ${pet.age}',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTextStyles.bodySmall,
                          ),
                          const SizedBox(height: AppSpacing.sm),
                          Text(
                            pet.personality.take(2).join('  •  '),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTextStyles.labelSmall.copyWith(
                              color: AppColors.primaryLight,
                            ),
                          ),
                        ],
                      ),
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

  Widget _buildCommunityList(
    List<CommunityPreview> communities,
    double availableWidth,
  ) {
    if (communities.isEmpty) {
      return const _EmptyResult(
        icon: Icons.groups_outlined,
        message: 'No communities found for this search.',
      );
    }

    final cardWidth = (availableWidth * (availableWidth >= 720 ? 0.31 : 0.76))
        .clamp(236.0, 304.0)
        .toDouble();

    return SizedBox(
      height: 180,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: communities.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.md),
        itemBuilder: (context, index) {
          final community = communities[index];
          final isJoined = _joinedCommunities.contains(community.name);
          final color = Color(community.colorValue);
          return Container(
            width: cardWidth,
            padding: const EdgeInsets.all(AppSpacing.lg),
            decoration: BoxDecoration(
              color: AppColors.card,
              borderRadius: AppRadius.lg,
              border: Border.all(color: AppColors.border),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      width: AppSpacing.giant,
                      height: AppSpacing.giant,
                      alignment: Alignment.center,
                      decoration: BoxDecoration(
                        color: color.withValues(alpha: 0.16),
                        borderRadius: AppRadius.md,
                      ),
                      child: Text(
                        community.icon,
                        style: AppTextStyles.headingMedium,
                      ),
                    ),
                    const Spacer(),
                    SizedBox(
                      height: AppSpacing.huge,
                      child: isJoined
                          ? OutlinedButton(
                              onPressed: () => _toggleCommunity(community),
                              style: OutlinedButton.styleFrom(
                                minimumSize: Size.zero,
                                padding: const EdgeInsets.symmetric(
                                  horizontal: AppSpacing.md,
                                ),
                              ),
                              child: const Text('Joined'),
                            )
                          : FilledButton(
                              onPressed: () => _toggleCommunity(community),
                              style: FilledButton.styleFrom(
                                minimumSize: Size.zero,
                                padding: const EdgeInsets.symmetric(
                                  horizontal: AppSpacing.lg,
                                ),
                                backgroundColor: color,
                              ),
                              child: const Text('Join'),
                            ),
                    ),
                  ],
                ),
                const Spacer(),
                Text(
                  community.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTextStyles.labelLarge,
                ),
                const SizedBox(height: AppSpacing.xs),
                Text(community.members, style: AppTextStyles.caption),
              ],
            ),
          );
        },
      ),
    );
  }

  Widget _buildEvents(List<EventPreview> events, double availableWidth) {
    if (events.isEmpty) {
      return const _EmptyResult(
        icon: Icons.event_busy_outlined,
        message: 'No nearby events match your search.',
      );
    }

    final wide = availableWidth >= 720;
    final itemWidth = wide
        ? (availableWidth - AppSpacing.md) / 2
        : availableWidth;
    return Wrap(
      spacing: AppSpacing.md,
      runSpacing: AppSpacing.md,
      children: events
          .map((event) {
            final isGoing = _rsvpedEvents.contains(event.title);
            final attendees = event.attendees + (isGoing ? 1 : 0);
            return SizedBox(
              width: itemWidth,
              child: Container(
                padding: const EdgeInsets.all(AppSpacing.lg),
                decoration: BoxDecoration(
                  color: AppColors.card,
                  borderRadius: AppRadius.lg,
                  border: Border.all(
                    color: isGoing ? AppColors.primary : AppColors.border,
                  ),
                ),
                child: Row(
                  children: [
                    Container(
                      width: AppSpacing.massive,
                      height: AppSpacing.massive,
                      decoration: BoxDecoration(
                        color: AppColors.primary.withValues(alpha: 0.14),
                        borderRadius: AppRadius.md,
                      ),
                      child: const Icon(
                        Icons.calendar_month_rounded,
                        color: AppColors.primaryLight,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            event.dateLabel,
                            style: AppTextStyles.overline.copyWith(
                              color: AppColors.accent,
                            ),
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            event.title,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTextStyles.labelLarge,
                          ),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            '${event.location} · $attendees going',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTextStyles.caption,
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    IconButton.filledTonal(
                      tooltip: isGoing ? 'Remove RSVP' : 'RSVP',
                      onPressed: () => _toggleRsvp(event),
                      style: IconButton.styleFrom(
                        backgroundColor: isGoing
                            ? AppColors.primary
                            : AppColors.elevatedSurface,
                        foregroundColor: AppColors.white,
                      ),
                      icon: Icon(
                        isGoing ? Icons.check_rounded : Icons.add_rounded,
                      ),
                    ),
                  ],
                ),
              ),
            );
          })
          .toList(growable: false),
    );
  }

  Widget _buildAdoptions(List<_AdoptionPet> adoptions, double availableWidth) {
    if (adoptions.isEmpty) {
      return const _EmptyResult(
        icon: Icons.home_outlined,
        message: 'No adoption listings match your search.',
      );
    }

    final wide = availableWidth >= 720;
    final itemWidth = wide
        ? (availableWidth - AppSpacing.md) / 2
        : availableWidth;
    return Wrap(
      spacing: AppSpacing.md,
      runSpacing: AppSpacing.md,
      children: adoptions
          .map((pet) {
            final isSaved = _savedAdoptions.contains(pet.id);
            return SizedBox(
              width: itemWidth,
              child: Material(
                color: AppColors.card,
                borderRadius: AppRadius.lg,
                clipBehavior: Clip.antiAlias,
                child: InkWell(
                  onTap: () => _toggleAdoption(pet),
                  child: SizedBox(
                    height: 154,
                    child: Row(
                      children: [
                        AspectRatio(
                          aspectRatio: 0.86,
                          child: AppImage(
                            pet.imageAsset,
                            fit: BoxFit.cover,
                            cacheWidth: 800,
                            errorBuilder: (_, _, _) => const ColoredBox(
                              color: AppColors.elevatedSurface,
                              child: Icon(
                                Icons.home_rounded,
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                        ),
                        Expanded(
                          child: Padding(
                            padding: const EdgeInsets.all(AppSpacing.md),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    Expanded(
                                      child: Text(
                                        pet.name,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: AppTextStyles.headingSmall,
                                      ),
                                    ),
                                    IconButton(
                                      visualDensity: VisualDensity.compact,
                                      tooltip: isSaved
                                          ? 'Remove from shortlist'
                                          : 'Add to shortlist',
                                      onPressed: () => _toggleAdoption(pet),
                                      icon: Icon(
                                        isSaved
                                            ? Icons.bookmark_rounded
                                            : Icons.bookmark_border_rounded,
                                        color: isSaved
                                            ? AppColors.accent
                                            : AppColors.textSecondary,
                                      ),
                                    ),
                                  ],
                                ),
                                Text(
                                  '${pet.breed} · ${pet.age}',
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTextStyles.bodySmall,
                                ),
                                const Spacer(),
                                Row(
                                  children: [
                                    const Icon(
                                      Icons.location_on_outlined,
                                      size: AppSpacing.lg,
                                      color: AppColors.textDisabled,
                                    ),
                                    const SizedBox(width: AppSpacing.xs),
                                    Expanded(
                                      child: Text(
                                        pet.location,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: AppTextStyles.caption,
                                      ),
                                    ),
                                  ],
                                ),
                              ],
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            );
          })
          .toList(growable: false),
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({
    required this.title,
    required this.subtitle,
    this.actionLabel,
    this.onAction,
  });

  final String title;
  final String subtitle;
  final String? actionLabel;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(title, style: AppTextStyles.headingMedium),
              const SizedBox(height: AppSpacing.xs),
              Text(subtitle, style: AppTextStyles.bodySmall),
            ],
          ),
        ),
        if (actionLabel != null && onAction != null)
          TextButton(
            onPressed: onAction,
            child: Text(
              actionLabel!,
              style: AppTextStyles.labelMedium.copyWith(
                color: AppColors.primaryLight,
              ),
            ),
          ),
      ],
    );
  }
}

class _DistanceBadge extends StatelessWidget {
  const _DistanceBadge({required this.distance});

  final double distance;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.overlay,
        borderRadius: AppRadius.pill,
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: AppSpacing.sm,
          vertical: AppSpacing.xs,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(
              Icons.near_me_rounded,
              color: AppColors.white,
              size: AppSpacing.md,
            ),
            const SizedBox(width: AppSpacing.xs),
            Text(
              '${distance.toStringAsFixed(1)} km',
              style: AppTextStyles.labelSmall.copyWith(color: AppColors.white),
            ),
          ],
        ),
      ),
    );
  }
}

class _EmptyResult extends StatelessWidget {
  const _EmptyResult({required this.icon, required this.message});

  final IconData icon;
  final String message;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(AppSpacing.xxl),
      decoration: BoxDecoration(
        color: AppColors.surface,
        borderRadius: AppRadius.lg,
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        children: [
          Icon(icon, color: AppColors.textDisabled),
          const SizedBox(height: AppSpacing.sm),
          Text(
            message,
            textAlign: TextAlign.center,
            style: AppTextStyles.bodySmall,
          ),
        ],
      ),
    );
  }
}

class _AdoptionPet {
  const _AdoptionPet({
    required this.id,
    required this.name,
    required this.breed,
    required this.age,
    required this.location,
    required this.imageAsset,
  });

  final String id;
  final String name;
  final String breed;
  final String age;
  final String location;
  final String imageAsset;
}
