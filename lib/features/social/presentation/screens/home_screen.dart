import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_shadows.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../application/social_controller.dart';
import '../../data/mock_social_data.dart';
import '../../domain/social_models.dart';

class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  @override
  Widget build(BuildContext context) {
    final socialState = ref.watch(socialControllerProvider);
    final posts = socialState.posts;
    final pets = _petsFor(socialState);
    final activePetIndex = socialState.activePetIndex < pets.length
        ? socialState.activePetIndex
        : 0;
    final activePet = pets[activePetIndex];

    return Scaffold(
      backgroundColor: AppColors.background,
      body: SafeArea(
        bottom: false,
        child: LayoutBuilder(
          builder: (context, constraints) {
            final horizontalPadding = constraints.maxWidth >= 720
                ? AppSpacing.xxl
                : AppSpacing.lg;

            return Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 760),
                child: CustomScrollView(
                  physics: const BouncingScrollPhysics(),
                  slivers: [
                    SliverToBoxAdapter(
                      child: _HomeHeader(
                        activePet: activePet,
                        horizontalPadding: horizontalPadding,
                        onSelectPet: () => _selectPet(pets, activePetIndex),
                        onCreatePost: () => _showCreateSheet(activePet),
                        onMessages: () => context.push<void>(AppRoutes.inbox),
                      ),
                    ),
                    SliverToBoxAdapter(
                      child: _StoriesSection(
                        stories: socialState.stories,
                        activePet: activePet,
                        horizontalPadding: horizontalPadding,
                        onStoryTap: (story) => _openStory(story, activePet),
                        onWatchAll: () => _showMessage(
                          'You are all caught up with your pack.',
                        ),
                      ),
                    ),
                    SliverPadding(
                      padding: EdgeInsets.fromLTRB(
                        horizontalPadding,
                        AppSpacing.xl,
                        horizontalPadding,
                        AppSpacing.md,
                      ),
                      sliver: const SliverToBoxAdapter(child: _FeedHeading()),
                    ),
                    SliverPadding(
                      padding: EdgeInsets.fromLTRB(
                        horizontalPadding,
                        0,
                        horizontalPadding,
                        AppSpacing.section + AppSpacing.massive,
                      ),
                      sliver: SliverList(
                        delegate: SliverChildBuilderDelegate(
                          (context, index) {
                            if (index.isOdd) {
                              return const SizedBox(height: AppSpacing.xl);
                            }

                            final post = posts[index ~/ 2];
                            return _FeedPostCard(
                              key: ValueKey(post.id),
                              post: post,
                              onLike: () => ref
                                  .read(socialControllerProvider.notifier)
                                  .toggleLike(post.id),
                              onSave: () => ref
                                  .read(socialControllerProvider.notifier)
                                  .toggleSave(post.id),
                              onComment: () => _showComments(post),
                              onShare: () => _showShareSheet(post),
                              onMore: () => _showMessage(
                                'Post options opened for ${post.petName}.',
                              ),
                            );
                          },
                          childCount: posts.isEmpty ? 0 : posts.length * 2 - 1,
                        ),
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

  List<_ActivePet> _petsFor(SocialState state) {
    if (state.pets.isNotEmpty) {
      return state.pets
          .map(
            (pet) => _ActivePet(
              name: pet.name,
              detail: pet.breed.isEmpty ? pet.petType : pet.breed,
              imageAsset: pet.imageAsset.isEmpty
                  ? MockSocialData.golden
                  : pet.imageAsset,
            ),
          )
          .toList(growable: false);
    }
    final profile = state.userProfile;
    final isCat = profile.petType.toLowerCase() == 'cat';
    final primaryPet = _ActivePet(
      name: profile.petName,
      detail: profile.petType,
      imageAsset: isCat ? MockSocialData.cat : MockSocialData.golden,
    );
    return [primaryPet];
  }

  Future<void> _selectPet(List<_ActivePet> pets, int currentIndex) async {
    final selectedIndex = await showModalBottomSheet<int>(
      context: context,
      useSafeArea: true,
      builder: (context) =>
          _PetPickerSheet(pets: pets, selectedIndex: currentIndex),
    );

    if (!mounted || selectedIndex == null) return;

    if (selectedIndex == -1) {
      _showMessage('Pet profile setup is coming next.');
      return;
    }

    ref.read(socialControllerProvider.notifier).selectActivePet(selectedIndex);
  }

  void _openStory(PetStory story, _ActivePet activePet) {
    if (story.isOwn) {
      _showMessage('Add a photo or video to ${activePet.name}\'s story.');
      return;
    }

    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: AppColors.black,
      builder: (sheetContext) => _StoryViewer(
        story: story,
        onReact: () {
          Navigator.of(sheetContext).pop();
          _showMessage('Reaction sent to ${story.petName}.');
        },
      ),
    );
  }

  void _showComments(FeedPost post) {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (context) => _CommentsSheet(post: post),
    );
  }

  Future<void> _showCreateSheet(_ActivePet activePet) async {
    final result = await showModalBottomSheet<String>(
      context: context,
      useSafeArea: true,
      builder: (context) => _CreateSheet(pet: activePet),
    );

    if (!mounted || result == null) return;
    if (result == 'create_post') {
      await _createImagePost();
    } else {
      _showMessage(result);
    }
  }

  Future<void> _createImagePost() async {
    final image = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      imageQuality: 88,
      maxWidth: 1800,
    );
    if (!mounted || image == null) return;
    final captionController = TextEditingController();
    final caption = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Share a new moment'),
        content: TextField(
          controller: captionController,
          autofocus: true,
          maxLines: 4,
          maxLength: 2200,
          decoration: const InputDecoration(
            hintText: 'Write a caption for your pack...',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () =>
                Navigator.of(dialogContext).pop(captionController.text),
            child: const Text('Post'),
          ),
        ],
      ),
    );
    captionController.dispose();
    if (!mounted || caption == null) return;
    _showMessage('Uploading your post...');
    final created = await ref
        .read(socialControllerProvider.notifier)
        .createImagePost(
          caption: caption,
          fileName: image.name,
          bytes: await image.readAsBytes(),
        );
    if (!mounted) return;
    final error = ref.read(socialControllerProvider).error;
    _showMessage(
      created
          ? 'Your post is live.'
          : error ?? 'The post could not be created.',
    );
  }

  Future<void> _showShareSheet(FeedPost post) async {
    final destination = await showModalBottomSheet<String>(
      context: context,
      useSafeArea: true,
      builder: (context) => _ShareSheet(post: post),
    );

    if (!mounted || destination == null) return;
    _showMessage(destination);
  }

  void _showMessage(String message) {
    final messenger = ScaffoldMessenger.of(context);
    messenger
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }
}

class _HomeHeader extends StatelessWidget {
  const _HomeHeader({
    required this.activePet,
    required this.horizontalPadding,
    required this.onSelectPet,
    required this.onCreatePost,
    required this.onMessages,
  });

  final _ActivePet activePet;
  final double horizontalPadding;
  final VoidCallback onSelectPet;
  final VoidCallback onCreatePost;
  final VoidCallback onMessages;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        horizontalPadding,
        AppSpacing.md,
        horizontalPadding,
        AppSpacing.xl,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: AppSpacing.huge,
                height: AppSpacing.huge,
                decoration: const BoxDecoration(
                  gradient: AppColors.primaryGradient,
                  borderRadius: AppRadius.md,
                  boxShadow: AppShadows.primaryGlow,
                ),
                child: const Icon(
                  Icons.pets_rounded,
                  color: AppColors.white,
                  size: AppSpacing.xxl,
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(
                  'PetConnect',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTextStyles.headingMedium.copyWith(
                    letterSpacing: -0.5,
                  ),
                ),
              ),
              _HeaderAction(
                tooltip: 'Create post',
                icon: Icons.add_rounded,
                onTap: onCreatePost,
              ),
              const SizedBox(width: AppSpacing.sm),
              _HeaderAction(
                tooltip: 'Messages',
                icon: Icons.chat_bubble_outline_rounded,
                showBadge: true,
                onTap: onMessages,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.xxl),
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '${activePet.name}\'s world',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTextStyles.headingLarge,
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Text(
                      'Fresh moments from pets you love',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTextStyles.bodySmall,
                    ),
                  ],
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              _ActivePetSelector(pet: activePet, onTap: onSelectPet),
            ],
          ),
        ],
      ),
    );
  }
}

class _HeaderAction extends StatelessWidget {
  const _HeaderAction({
    required this.tooltip,
    required this.icon,
    required this.onTap,
    this.showBadge = false,
  });

  final String tooltip;
  final IconData icon;
  final VoidCallback onTap;
  final bool showBadge;

  @override
  Widget build(BuildContext context) {
    return Stack(
      clipBehavior: Clip.none,
      children: [
        Material(
          color: AppColors.surface,
          borderRadius: AppRadius.circular,
          child: InkWell(
            onTap: onTap,
            borderRadius: AppRadius.circular,
            child: Tooltip(
              message: tooltip,
              child: SizedBox.square(
                dimension: AppSpacing.huge,
                child: Icon(icon, size: AppSpacing.xxl),
              ),
            ),
          ),
        ),
        if (showBadge)
          Positioned(
            right: AppSpacing.xs,
            top: AppSpacing.xs,
            child: Container(
              width: AppSpacing.sm,
              height: AppSpacing.sm,
              decoration: const BoxDecoration(
                color: AppColors.accent,
                shape: BoxShape.circle,
              ),
            ),
          ),
      ],
    );
  }
}

class _ActivePetSelector extends StatelessWidget {
  const _ActivePetSelector({required this.pet, required this.onTap});

  final _ActivePet pet;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.card,
      borderRadius: AppRadius.pill,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.pill,
        child: Container(
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.xs,
            AppSpacing.xs,
            AppSpacing.md,
            AppSpacing.xs,
          ),
          decoration: BoxDecoration(
            border: Border.all(color: AppColors.border),
            borderRadius: AppRadius.pill,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _AssetAvatar(imageAsset: pet.imageAsset, size: AppSpacing.huge),
              const SizedBox(width: AppSpacing.sm),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 72),
                child: Text(
                  pet.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTextStyles.labelMedium,
                ),
              ),
              const SizedBox(width: AppSpacing.xs),
              const Icon(
                Icons.keyboard_arrow_down_rounded,
                color: AppColors.textSecondary,
                size: AppSpacing.xl,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StoriesSection extends StatelessWidget {
  const _StoriesSection({
    required this.stories,
    required this.activePet,
    required this.horizontalPadding,
    required this.onStoryTap,
    required this.onWatchAll,
  });

  final List<PetStory> stories;
  final _ActivePet activePet;
  final double horizontalPadding;
  final ValueChanged<PetStory> onStoryTap;
  final VoidCallback onWatchAll;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: const BoxDecoration(
        border: Border(
          top: BorderSide(color: AppColors.border),
          bottom: BorderSide(color: AppColors.border),
        ),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
        child: Column(
          children: [
            Padding(
              padding: EdgeInsets.symmetric(horizontal: horizontalPadding),
              child: Row(
                children: [
                  Expanded(
                    child: Text('Stories', style: AppTextStyles.headingSmall),
                  ),
                  TextButton(
                    onPressed: onWatchAll,
                    child: const Text('Watch all'),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
            SizedBox(
              height: 94,
              child: ListView.separated(
                padding: EdgeInsets.symmetric(horizontal: horizontalPadding),
                scrollDirection: Axis.horizontal,
                physics: const BouncingScrollPhysics(),
                itemCount: stories.length,
                separatorBuilder: (_, _) =>
                    const SizedBox(width: AppSpacing.lg),
                itemBuilder: (context, index) {
                  final story = stories[index];
                  return _StoryBubble(
                    story: story,
                    imageAsset: story.isOwn
                        ? activePet.imageAsset
                        : story.imageAsset,
                    onTap: () => onStoryTap(story),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _StoryBubble extends StatelessWidget {
  const _StoryBubble({
    required this.story,
    required this.imageAsset,
    required this.onTap,
  });

  final PetStory story;
  final String imageAsset;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: story.isOwn ? 'Add your story' : '${story.petName}\'s story',
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.md,
        child: SizedBox(
          width: 68,
          child: Column(
            children: [
              Stack(
                clipBehavior: Clip.none,
                children: [
                  Container(
                    width: AppSpacing.section,
                    height: AppSpacing.section,
                    padding: const EdgeInsets.all(AppSpacing.xxs),
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      gradient: story.isViewed
                          ? null
                          : AppColors.primaryGradient,
                      border: story.isViewed
                          ? Border.all(color: AppColors.border, width: 2)
                          : null,
                    ),
                    child: Container(
                      padding: const EdgeInsets.all(AppSpacing.xxs),
                      decoration: const BoxDecoration(
                        color: AppColors.background,
                        shape: BoxShape.circle,
                      ),
                      child: ClipOval(
                        child: AppImage(
                          imageAsset,
                          fit: BoxFit.cover,
                          cacheWidth: 180,
                          errorBuilder: (_, _, _) => const ColoredBox(
                            color: AppColors.elevatedSurface,
                            child: Icon(Icons.pets_rounded),
                          ),
                        ),
                      ),
                    ),
                  ),
                  if (story.isOwn)
                    Positioned(
                      right: -AppSpacing.xxs,
                      bottom: -AppSpacing.xxs,
                      child: Container(
                        width: AppSpacing.xl,
                        height: AppSpacing.xl,
                        decoration: BoxDecoration(
                          color: AppColors.primary,
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: AppColors.background,
                            width: AppSpacing.xxs,
                          ),
                        ),
                        child: const Icon(
                          Icons.add_rounded,
                          color: AppColors.white,
                          size: AppSpacing.lg,
                        ),
                      ),
                    ),
                ],
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                story.isOwn ? 'Your story' : story.petName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                textAlign: TextAlign.center,
                style: AppTextStyles.labelSmall.copyWith(
                  color: story.isViewed
                      ? AppColors.textDisabled
                      : AppColors.textPrimary,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _FeedHeading extends StatelessWidget {
  const _FeedHeading();

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('From your pack', style: AppTextStyles.headingSmall),
              const SizedBox(height: AppSpacing.xs),
              Text('Picked for you today', style: AppTextStyles.bodySmall),
            ],
          ),
        ),
        Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.md,
            vertical: AppSpacing.sm,
          ),
          decoration: BoxDecoration(
            color: AppColors.primary.withValues(alpha: 0.12),
            borderRadius: AppRadius.pill,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(
                Icons.near_me_rounded,
                size: AppSpacing.lg,
                color: AppColors.primaryLight,
              ),
              const SizedBox(width: AppSpacing.xs),
              Text(
                'Nearby',
                style: AppTextStyles.labelSmall.copyWith(
                  color: AppColors.primaryLight,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _FeedPostCard extends StatelessWidget {
  const _FeedPostCard({
    super.key,
    required this.post,
    required this.onLike,
    required this.onSave,
    required this.onComment,
    required this.onShare,
    required this.onMore,
  });

  final FeedPost post;
  final VoidCallback onLike;
  final VoidCallback onSave;
  final VoidCallback onComment;
  final VoidCallback onShare;
  final VoidCallback onMore;

  @override
  Widget build(BuildContext context) {
    return Container(
      clipBehavior: Clip.antiAlias,
      decoration: BoxDecoration(
        color: AppColors.card,
        borderRadius: AppRadius.xl,
        border: Border.all(color: AppColors.border),
        boxShadow: AppShadows.card,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.all(AppSpacing.md),
            child: Row(
              children: [
                Container(
                  width: AppSpacing.giant,
                  height: AppSpacing.giant,
                  padding: const EdgeInsets.all(AppSpacing.xxs),
                  decoration: const BoxDecoration(
                    gradient: AppColors.primaryGradient,
                    shape: BoxShape.circle,
                  ),
                  child: _AssetAvatar(
                    imageAsset: post.avatarAsset,
                    size: AppSpacing.giant,
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
                              post.petName,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: AppTextStyles.labelLarge,
                            ),
                          ),
                          const SizedBox(width: AppSpacing.xs),
                          const Icon(
                            Icons.verified_rounded,
                            color: AppColors.info,
                            size: AppSpacing.lg,
                          ),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.xxs),
                      Text(
                        '${post.ownerHandle}  ·  ${post.location}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTextStyles.caption,
                      ),
                    ],
                  ),
                ),
                Text(post.postedAgo, style: AppTextStyles.caption),
                IconButton(
                  onPressed: onMore,
                  tooltip: 'Post options',
                  visualDensity: VisualDensity.compact,
                  icon: const Icon(
                    Icons.more_horiz_rounded,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
          LayoutBuilder(
            builder: (context, constraints) {
              final isWide = constraints.maxWidth >= 600;
              final pixelRatio = MediaQuery.devicePixelRatioOf(context);
              final cacheWidth = (constraints.maxWidth * pixelRatio)
                  .round()
                  .clamp(600, 1600);

              return GestureDetector(
                onDoubleTap: post.isLiked ? null : onLike,
                child: Semantics(
                  image: true,
                  label: 'Photo posted by ${post.petName}',
                  child: AspectRatio(
                    aspectRatio: isWide ? 16 / 10 : 4 / 5,
                    child: Stack(
                      fit: StackFit.expand,
                      children: [
                        AppImage(
                          post.mediaAsset,
                          fit: BoxFit.cover,
                          cacheWidth: cacheWidth,
                          errorBuilder: (_, _, _) => const ColoredBox(
                            color: AppColors.elevatedSurface,
                            child: Icon(
                              Icons.image_not_supported_outlined,
                              color: AppColors.textDisabled,
                              size: AppSpacing.giant,
                            ),
                          ),
                        ),
                        Positioned(
                          left: AppSpacing.md,
                          bottom: AppSpacing.md,
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
                                const Icon(
                                  Icons.location_on_rounded,
                                  color: AppColors.white,
                                  size: AppSpacing.lg,
                                ),
                                const SizedBox(width: AppSpacing.xs),
                                ConstrainedBox(
                                  constraints: const BoxConstraints(
                                    maxWidth: 190,
                                  ),
                                  child: Text(
                                    post.location,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTextStyles.labelSmall.copyWith(
                                      color: AppColors.white,
                                    ),
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
            },
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.sm,
              AppSpacing.sm,
              AppSpacing.sm,
              0,
            ),
            child: Row(
              children: [
                _PostAction(
                  tooltip: post.isLiked ? 'Unlike post' : 'Like post',
                  icon: post.isLiked
                      ? Icons.favorite_rounded
                      : Icons.favorite_border_rounded,
                  color: post.isLiked
                      ? AppColors.accent
                      : AppColors.textPrimary,
                  onTap: onLike,
                ),
                _PostAction(
                  tooltip: 'Comment',
                  icon: Icons.chat_bubble_outline_rounded,
                  onTap: onComment,
                ),
                _PostAction(
                  tooltip: 'Share post',
                  icon: Icons.send_rounded,
                  onTap: onShare,
                ),
                const Spacer(),
                _PostAction(
                  tooltip: post.isSaved ? 'Remove from saved' : 'Save post',
                  icon: post.isSaved
                      ? Icons.bookmark_rounded
                      : Icons.bookmark_border_rounded,
                  color: post.isSaved
                      ? AppColors.primaryLight
                      : AppColors.textPrimary,
                  onTap: onSave,
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.lg,
              AppSpacing.xs,
              AppSpacing.lg,
              AppSpacing.lg,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                AnimatedSwitcher(
                  duration: const Duration(milliseconds: 180),
                  child: Text(
                    '${_compactCount(post.likes)} likes',
                    key: ValueKey(post.likes),
                    style: AppTextStyles.labelMedium,
                  ),
                ),
                const SizedBox(height: AppSpacing.sm),
                Text.rich(
                  TextSpan(
                    children: [
                      TextSpan(
                        text: '${post.ownerHandle}  ',
                        style: AppTextStyles.labelMedium,
                      ),
                      TextSpan(
                        text: _cleanCaption(post),
                        style: AppTextStyles.bodyMedium,
                      ),
                    ],
                  ),
                  maxLines: 3,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: AppSpacing.sm),
                InkWell(
                  onTap: onComment,
                  borderRadius: AppRadius.xs,
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      vertical: AppSpacing.xs,
                    ),
                    child: Text(
                      'View all ${post.comments} comments',
                      style: AppTextStyles.bodySmall.copyWith(
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _PostAction extends StatelessWidget {
  const _PostAction({
    required this.tooltip,
    required this.icon,
    required this.onTap,
    this.color = AppColors.textPrimary,
  });

  final String tooltip;
  final IconData icon;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      onPressed: onTap,
      tooltip: tooltip,
      visualDensity: VisualDensity.compact,
      icon: AnimatedSwitcher(
        duration: const Duration(milliseconds: 180),
        transitionBuilder: (child, animation) =>
            ScaleTransition(scale: animation, child: child),
        child: Icon(icon, key: ValueKey(icon), color: color),
      ),
    );
  }
}

class _PetPickerSheet extends StatelessWidget {
  const _PetPickerSheet({required this.pets, required this.selectedIndex});

  final List<_ActivePet> pets;
  final int selectedIndex;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.xl,
        AppSpacing.md,
        AppSpacing.xl,
        AppSpacing.xxl,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Center(child: _SheetHandle()),
          const SizedBox(height: AppSpacing.xl),
          Text('Choose your active pet', style: AppTextStyles.headingMedium),
          const SizedBox(height: AppSpacing.xs),
          Text(
            'Your feed and social actions will appear as this pet.',
            style: AppTextStyles.bodySmall,
          ),
          const SizedBox(height: AppSpacing.xl),
          for (var index = 0; index < pets.length; index++) ...[
            _PetPickerTile(
              pet: pets[index],
              isSelected: index == selectedIndex,
              onTap: () => Navigator.of(context).pop(index),
            ),
            if (index != pets.length - 1) const SizedBox(height: AppSpacing.sm),
          ],
          const SizedBox(height: AppSpacing.md),
          const Divider(),
          ListTile(
            onTap: () => Navigator.of(context).pop(-1),
            contentPadding: EdgeInsets.zero,
            leading: Container(
              width: AppSpacing.giant,
              height: AppSpacing.giant,
              decoration: BoxDecoration(
                color: AppColors.primary.withValues(alpha: 0.12),
                shape: BoxShape.circle,
              ),
              child: const Icon(
                Icons.add_rounded,
                color: AppColors.primaryLight,
              ),
            ),
            title: Text('Add another pet', style: AppTextStyles.labelLarge),
            trailing: const Icon(
              Icons.arrow_forward_ios_rounded,
              color: AppColors.textDisabled,
              size: AppSpacing.lg,
            ),
          ),
        ],
      ),
    );
  }
}

class _PetPickerTile extends StatelessWidget {
  const _PetPickerTile({
    required this.pet,
    required this.isSelected,
    required this.onTap,
  });

  final _ActivePet pet;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: isSelected
          ? AppColors.primary.withValues(alpha: 0.12)
          : AppColors.card,
      borderRadius: AppRadius.lg,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: Container(
          padding: const EdgeInsets.all(AppSpacing.md),
          decoration: BoxDecoration(
            borderRadius: AppRadius.lg,
            border: Border.all(
              color: isSelected ? AppColors.primary : AppColors.border,
            ),
          ),
          child: Row(
            children: [
              _AssetAvatar(
                imageAsset: pet.imageAsset,
                size: AppSpacing.massive,
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(pet.name, style: AppTextStyles.labelLarge),
                    const SizedBox(height: AppSpacing.xxs),
                    Text(pet.detail, style: AppTextStyles.caption),
                  ],
                ),
              ),
              Icon(
                isSelected ? Icons.check_circle_rounded : Icons.circle_outlined,
                color: isSelected
                    ? AppColors.primaryLight
                    : AppColors.textDisabled,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StoryViewer extends StatelessWidget {
  const _StoryViewer({required this.story, required this.onReact});

  final PetStory story;
  final VoidCallback onReact;

  @override
  Widget build(BuildContext context) {
    return FractionallySizedBox(
      heightFactor: 0.88,
      child: SafeArea(
        child: ClipRRect(
          borderRadius: const BorderRadius.vertical(
            top: Radius.circular(AppSpacing.xxxl),
          ),
          child: Stack(
            fit: StackFit.expand,
            children: [
              AppImage(
                story.imageAsset,
                fit: BoxFit.cover,
                cacheWidth: 1200,
                errorBuilder: (_, _, _) => const ColoredBox(
                  color: AppColors.elevatedSurface,
                  child: Icon(Icons.pets_rounded, size: AppSpacing.section),
                ),
              ),
              DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      AppColors.overlayHeavy,
                      AppColors.transparent,
                      AppColors.overlay,
                    ],
                    stops: [0, 0.5, 1],
                  ),
                ),
              ),
              Align(
                alignment: Alignment.topCenter,
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.lg),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      ClipRRect(
                        borderRadius: AppRadius.pill,
                        child: LinearProgressIndicator(
                          value: 0.72,
                          minHeight: AppSpacing.xs,
                          backgroundColor: AppColors.glassBorder,
                          valueColor: AlwaysStoppedAnimation(AppColors.white),
                        ),
                      ),
                      const SizedBox(height: AppSpacing.md),
                      Row(
                        children: [
                          _AssetAvatar(
                            imageAsset: story.imageAsset,
                            size: AppSpacing.huge,
                          ),
                          const SizedBox(width: AppSpacing.md),
                          Expanded(
                            child: Text(
                              '${story.petName}  ·  now',
                              style: AppTextStyles.labelMedium.copyWith(
                                color: AppColors.white,
                              ),
                            ),
                          ),
                          IconButton(
                            onPressed: () => Navigator.of(context).pop(),
                            tooltip: 'Close story',
                            icon: const Icon(
                              Icons.close_rounded,
                              color: AppColors.white,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
              Align(
                alignment: Alignment.bottomCenter,
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: Row(
                    children: [
                      Expanded(
                        child: Container(
                          height: AppSpacing.giant,
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppSpacing.lg,
                          ),
                          alignment: Alignment.centerLeft,
                          decoration: BoxDecoration(
                            color: AppColors.overlayLight,
                            borderRadius: AppRadius.pill,
                            border: Border.all(color: AppColors.glassBorder),
                          ),
                          child: Text(
                            'Reply to ${story.petName}…',
                            style: AppTextStyles.bodySmall.copyWith(
                              color: AppColors.white,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Material(
                        color: AppColors.overlayLight,
                        shape: CircleBorder(
                          side: BorderSide(color: AppColors.glassBorder),
                        ),
                        child: IconButton(
                          onPressed: onReact,
                          tooltip: 'Send a heart',
                          icon: const Icon(
                            Icons.favorite_rounded,
                            color: AppColors.accent,
                          ),
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

class _CommentsSheet extends ConsumerStatefulWidget {
  const _CommentsSheet({required this.post});

  final FeedPost post;

  @override
  ConsumerState<_CommentsSheet> createState() => _CommentsSheetState();
}

class _CommentsSheetState extends ConsumerState<_CommentsSheet> {
  final _controller = TextEditingController();
  final _comments = <String>[];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    Future<void>.microtask(_loadComments);
  }

  Future<void> _loadComments() async {
    final comments = await ref
        .read(socialControllerProvider.notifier)
        .loadComments(widget.post.id);
    if (!mounted) return;
    setState(() {
      _comments
        ..clear()
        ..addAll(comments);
      _loading = false;
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _sendComment() async {
    final comment = _controller.text.trim();
    if (comment.isEmpty) return;
    _controller.clear();
    FocusScope.of(context).unfocus();
    final created = await ref
        .read(socialControllerProvider.notifier)
        .addComment(widget.post.id, comment);
    if (!mounted || created == null) return;
    setState(() => _comments.add(created));
  }

  @override
  Widget build(BuildContext context) {
    final socialState = ref.watch(socialControllerProvider);
    final ownImage = socialState.pets.isEmpty
        ? MockSocialData.golden
        : socialState.pets.first.imageAsset;
    final keyboardInset = MediaQuery.viewInsetsOf(context).bottom;
    final availableHeight = MediaQuery.sizeOf(context).height * 0.72;

    return AnimatedPadding(
      duration: const Duration(milliseconds: 180),
      curve: Curves.easeOut,
      padding: EdgeInsets.only(bottom: keyboardInset),
      child: SizedBox(
        height: availableHeight,
        child: Column(
          children: [
            const SizedBox(height: AppSpacing.md),
            const _SheetHandle(),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.xl,
                AppSpacing.lg,
                AppSpacing.xl,
                AppSpacing.md,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: Text('Comments', style: AppTextStyles.headingMedium),
                  ),
                  Text(
                    _compactCount(_comments.length),
                    style: AppTextStyles.labelMedium.copyWith(
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ),
            ),
            const Divider(height: 1),
            Expanded(
              child: ListView(
                padding: const EdgeInsets.all(AppSpacing.xl),
                children: [
                  if (_loading)
                    const Center(child: CircularProgressIndicator())
                  else if (_comments.isEmpty)
                    Center(
                      child: Text(
                        'No comments yet. Start the conversation.',
                        style: AppTextStyles.bodySmall,
                      ),
                    ),
                  for (final comment in _comments) ...[
                    const SizedBox(height: AppSpacing.xl),
                    _CommentTile(
                      imageAsset: ownImage,
                      handle: socialState.userProfile.ownerName,
                      text: comment,
                      time: '',
                    ),
                  ],
                ],
              ),
            ),
            const Divider(height: 1),
            SafeArea(
              top: false,
              child: Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Row(
                  children: [
                    _AssetAvatar(imageAsset: ownImage, size: AppSpacing.huge),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: TextField(
                        controller: _controller,
                        textInputAction: TextInputAction.send,
                        onSubmitted: (_) => _sendComment(),
                        decoration: InputDecoration(
                          hintText: 'Add a kind comment…',
                          isDense: true,
                          contentPadding: const EdgeInsets.symmetric(
                            horizontal: AppSpacing.lg,
                            vertical: AppSpacing.md,
                          ),
                          suffixIcon: IconButton(
                            onPressed: _sendComment,
                            tooltip: 'Send comment',
                            icon: const Icon(
                              Icons.arrow_upward_rounded,
                              color: AppColors.primaryLight,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CommentTile extends StatelessWidget {
  const _CommentTile({
    required this.imageAsset,
    required this.handle,
    required this.text,
    required this.time,
  });

  final String imageAsset;
  final String handle;
  final String text;
  final String time;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _AssetAvatar(imageAsset: imageAsset, size: AppSpacing.huge),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(handle, style: AppTextStyles.labelMedium),
                  ),
                  Text(time, style: AppTextStyles.caption),
                ],
              ),
              const SizedBox(height: AppSpacing.xs),
              Text(text, style: AppTextStyles.bodyMedium),
              const SizedBox(height: AppSpacing.sm),
              Text(
                'Reply',
                style: AppTextStyles.labelSmall.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
            ],
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        const Icon(
          Icons.favorite_border_rounded,
          color: AppColors.textDisabled,
          size: AppSpacing.lg,
        ),
      ],
    );
  }
}

class _CreateSheet extends StatelessWidget {
  const _CreateSheet({required this.pet});

  final _ActivePet pet;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.xl,
        AppSpacing.md,
        AppSpacing.xl,
        AppSpacing.xxl,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Center(child: _SheetHandle()),
          const SizedBox(height: AppSpacing.xl),
          Row(
            children: [
              _AssetAvatar(imageAsset: pet.imageAsset, size: AppSpacing.giant),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Create as ${pet.name}',
                      style: AppTextStyles.headingMedium,
                    ),
                    const SizedBox(height: AppSpacing.xxs),
                    Text(
                      'Share a new moment with your pack.',
                      style: AppTextStyles.bodySmall,
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.xl),
          _CreateOption(
            icon: Icons.grid_on_rounded,
            title: 'Post',
            subtitle: 'Photo, video, or carousel',
            color: AppColors.primary,
            onTap: () => Navigator.of(context).pop('create_post'),
          ),
          const SizedBox(height: AppSpacing.sm),
          _CreateOption(
            icon: Icons.auto_stories_rounded,
            title: 'Story',
            subtitle: 'A moment that lasts 24 hours',
            color: AppColors.accent,
            onTap: () => Navigator.of(
              context,
            ).pop('${pet.name}\'s story creator is ready.'),
          ),
          const SizedBox(height: AppSpacing.sm),
          _CreateOption(
            icon: Icons.movie_creation_outlined,
            title: 'Reel',
            subtitle: 'Create a short vertical video',
            color: AppColors.info,
            onTap: () => Navigator.of(
              context,
            ).pop('${pet.name}\'s reel creator is ready.'),
          ),
        ],
      ),
    );
  }
}

class _CreateOption extends StatelessWidget {
  const _CreateOption({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.color,
    required this.onTap,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.card,
      borderRadius: AppRadius.lg,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: Container(
          padding: const EdgeInsets.all(AppSpacing.md),
          decoration: BoxDecoration(
            border: Border.all(color: AppColors.border),
            borderRadius: AppRadius.lg,
          ),
          child: Row(
            children: [
              Container(
                width: AppSpacing.giant,
                height: AppSpacing.giant,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.14),
                  shape: BoxShape.circle,
                ),
                child: Icon(icon, color: color),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title, style: AppTextStyles.labelLarge),
                    const SizedBox(height: AppSpacing.xxs),
                    Text(subtitle, style: AppTextStyles.caption),
                  ],
                ),
              ),
              const Icon(
                Icons.chevron_right_rounded,
                color: AppColors.textDisabled,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShareSheet extends StatelessWidget {
  const _ShareSheet({required this.post});

  final FeedPost post;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.xl,
        AppSpacing.md,
        AppSpacing.xl,
        AppSpacing.xxl,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Center(child: _SheetHandle()),
          const SizedBox(height: AppSpacing.xl),
          Text(
            'Share ${post.petName}\'s post',
            style: AppTextStyles.headingMedium,
          ),
          const SizedBox(height: AppSpacing.xl),
          Row(
            children: [
              Expanded(
                child: _ShareOption(
                  icon: Icons.chat_bubble_rounded,
                  label: 'PetConnect',
                  onTap: () => Navigator.of(
                    context,
                  ).pop('Shared with your PetConnect friends.'),
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: _ShareOption(
                  icon: Icons.link_rounded,
                  label: 'Copy link',
                  onTap: () => Navigator.of(context).pop('Post link copied.'),
                ),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: _ShareOption(
                  icon: Icons.more_horiz_rounded,
                  label: 'More',
                  onTap: () => Navigator.of(
                    context,
                  ).pop('More sharing options are ready.'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ShareOption extends StatelessWidget {
  const _ShareOption({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.card,
      borderRadius: AppRadius.lg,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppSpacing.sm,
            vertical: AppSpacing.lg,
          ),
          child: Column(
            children: [
              Container(
                width: AppSpacing.giant,
                height: AppSpacing.giant,
                decoration: BoxDecoration(
                  color: AppColors.primary.withValues(alpha: 0.12),
                  shape: BoxShape.circle,
                ),
                child: Icon(icon, color: AppColors.primaryLight),
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.labelSmall.copyWith(
                  color: AppColors.textPrimary,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _AssetAvatar extends StatelessWidget {
  const _AssetAvatar({required this.imageAsset, required this.size});

  final String imageAsset;
  final double size;

  @override
  Widget build(BuildContext context) {
    return SizedBox.square(
      dimension: size,
      child: ClipOval(
        child: AppImage(
          imageAsset,
          fit: BoxFit.cover,
          cacheWidth: 180,
          errorBuilder: (_, _, _) => const ColoredBox(
            color: AppColors.elevatedSurface,
            child: Icon(Icons.pets_rounded, color: AppColors.textSecondary),
          ),
        ),
      ),
    );
  }
}

class _SheetHandle extends StatelessWidget {
  const _SheetHandle();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: AppSpacing.huge,
      height: AppSpacing.xs,
      decoration: const BoxDecoration(
        color: AppColors.divider,
        borderRadius: AppRadius.pill,
      ),
    );
  }
}

class _ActivePet {
  const _ActivePet({
    required this.name,
    required this.detail,
    required this.imageAsset,
  });

  final String name;
  final String detail;
  final String imageAsset;
}

String _compactCount(int value) {
  if (value >= 1000000) {
    final count = value / 1000000;
    return '${count.toStringAsFixed(count >= 10 ? 0 : 1)}M';
  }
  if (value >= 1000) {
    final count = value / 1000;
    return '${count.toStringAsFixed(count >= 10 ? 0 : 1)}K';
  }
  return '$value';
}

String _cleanCaption(FeedPost post) {
  return switch (post.id) {
    'post-playdate' =>
      'When a quick hello turns into the best playdate ever. Same time tomorrow? 🐾',
    'post-simba' => 'Today’s agenda: one cuddle, three naps, zero meetings.',
    _ => post.caption,
  };
}
