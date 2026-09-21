import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../../auth/application/auth_controller.dart';
import '../../application/social_controller.dart';
import '../../domain/social_models.dart';

class ReelsScreen extends ConsumerStatefulWidget {
  const ReelsScreen({super.key});

  @override
  ConsumerState<ReelsScreen> createState() => _ReelsScreenState();
}

class _ReelsScreenState extends ConsumerState<ReelsScreen> {
  int _activeIndex = 0;
  bool _isMuted = true;
  bool _isPaused = false;
  String? _heartPostId;

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
          duration: const Duration(seconds: 2),
        ),
      );
  }

  void _toggleLike(FeedPost post) {
    ref.read(socialControllerProvider.notifier).toggleLike(post.id);
  }

  Future<void> _likeFromDoubleTap(FeedPost post) async {
    if (!post.isLiked) {
      ref.read(socialControllerProvider.notifier).toggleLike(post.id);
    }
    setState(() {
      _heartPostId = post.id;
    });
    await Future<void>.delayed(const Duration(milliseconds: 650));
    if (!mounted || _heartPostId != post.id) return;
    setState(() => _heartPostId = null);
  }

  Future<void> _toggleFollow(FeedPost post) async {
    final wasFollowing = post.isFollowedByMe;
    await ref.read(socialControllerProvider.notifier).toggleFollow(post);
    if (!mounted) return;
    final error = ref.read(socialControllerProvider).error;
    if (error != null) {
      _showFeedback(error, icon: Icons.error_outline_rounded);
      return;
    }
    _showFeedback(
      wasFollowing
          ? 'Unfollowed ${post.ownerHandle}.'
          : 'Following ${post.ownerHandle}.',
      icon: wasFollowing ? Icons.person_remove_alt_1 : Icons.person_add_alt_1,
    );
  }

  void _toggleSave(FeedPost post) {
    ref.read(socialControllerProvider.notifier).toggleSave(post.id);
    final isSaved = !post.isSaved;
    _showFeedback(
      isSaved ? 'Reel saved to your collection.' : 'Reel removed from saved.',
      icon: isSaved ? Icons.bookmark : Icons.bookmark_border,
    );
  }

  void _openComments(FeedPost post) {
    unawaited(
      showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        useSafeArea: true,
        builder: (context) => FractionallySizedBox(
          heightFactor: 0.68,
          child: _CommentsSheet(post: post),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final posts = ref.watch(
      socialControllerProvider.select((state) => state.posts),
    );
    final currentUserId = ref.watch(authControllerProvider).user?.id;

    return Scaffold(
      backgroundColor: AppColors.black,
      body: Stack(
        children: [
          if (posts.isEmpty)
            Center(
              child: Text(
                'Fresh reels are on the way.',
                style: AppTextStyles.bodyMedium.copyWith(
                  color: AppColors.textSecondary,
                ),
              ),
            )
          else
            PageView.builder(
              scrollDirection: Axis.vertical,
              itemCount: posts.length,
              onPageChanged: (index) {
                setState(() {
                  _activeIndex = index;
                  _isPaused = false;
                  _heartPostId = null;
                });
              },
              itemBuilder: (context, index) {
                final post = posts[index];
                final isActive = index == _activeIndex;
                return Center(
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 720),
                    child: _ReelPage(
                      post: post,
                      reelNumber: index + 1,
                      totalReels: posts.length,
                      isLiked: post.isLiked,
                      isSaved: post.isSaved,
                      isFollowing: post.isFollowedByMe,
                      showFollow:
                          post.petId.isNotEmpty &&
                          post.authorUserId != currentUserId,
                      isMuted: _isMuted,
                      isPaused: isActive && _isPaused,
                      showHeart: _heartPostId == post.id,
                      likeCount: post.likes,
                      onLike: () => _toggleLike(post),
                      onDoubleTap: () => _likeFromDoubleTap(post),
                      onFollow: () => _toggleFollow(post),
                      onSave: () => _toggleSave(post),
                      onComment: () => _openComments(post),
                      onShare: () => _showFeedback(
                        'Share options opened for ${post.petName}.',
                        icon: Icons.ios_share_rounded,
                      ),
                      onToggleMuted: () => setState(() => _isMuted = !_isMuted),
                      onTogglePaused: () {
                        if (!isActive) return;
                        setState(() => _isPaused = !_isPaused);
                      },
                    ),
                  ),
                );
              },
            ),
          _ReelsHeader(
            current: posts.isEmpty ? 0 : _activeIndex + 1,
            total: posts.length,
            onCreate: () => _showFeedback(
              'Your camera is ready for a new pet reel.',
              icon: Icons.video_camera_back_rounded,
            ),
          ),
        ],
      ),
    );
  }
}

class _ReelsHeader extends StatelessWidget {
  const _ReelsHeader({
    required this.current,
    required this.total,
    required this.onCreate,
  });

  final int current;
  final int total;
  final VoidCallback onCreate;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      bottom: false,
      child: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 720),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.lg,
              AppSpacing.sm,
              AppSpacing.sm,
              AppSpacing.sm,
            ),
            child: Row(
              children: [
                Text(
                  'Reels',
                  style: AppTextStyles.headingMedium.copyWith(
                    color: AppColors.white,
                    shadows: const [
                      Shadow(color: AppColors.black, blurRadius: 12),
                    ],
                  ),
                ),
                const SizedBox(width: AppSpacing.md),
                if (total > 0)
                  DecoratedBox(
                    decoration: BoxDecoration(
                      color: AppColors.overlayLight,
                      borderRadius: AppRadius.pill,
                      border: Border.all(color: AppColors.glassBorder),
                    ),
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: AppSpacing.sm,
                        vertical: AppSpacing.xs,
                      ),
                      child: Text(
                        '$current / $total',
                        style: AppTextStyles.labelSmall.copyWith(
                          color: AppColors.white,
                        ),
                      ),
                    ),
                  ),
                const Spacer(),
                IconButton.filledTonal(
                  tooltip: 'Create reel',
                  onPressed: onCreate,
                  style: IconButton.styleFrom(
                    backgroundColor: AppColors.overlayLight,
                    foregroundColor: AppColors.white,
                  ),
                  icon: const Icon(Icons.add_a_photo_outlined),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ReelPage extends StatelessWidget {
  const _ReelPage({
    required this.post,
    required this.reelNumber,
    required this.totalReels,
    required this.isLiked,
    required this.isSaved,
    required this.isFollowing,
    required this.showFollow,
    required this.isMuted,
    required this.isPaused,
    required this.showHeart,
    required this.likeCount,
    required this.onLike,
    required this.onDoubleTap,
    required this.onFollow,
    required this.onSave,
    required this.onComment,
    required this.onShare,
    required this.onToggleMuted,
    required this.onTogglePaused,
  });

  final FeedPost post;
  final int reelNumber;
  final int totalReels;
  final bool isLiked;
  final bool isSaved;
  final bool isFollowing;
  final bool showFollow;
  final bool isMuted;
  final bool isPaused;
  final bool showHeart;
  final int likeCount;
  final VoidCallback onLike;
  final VoidCallback onDoubleTap;
  final VoidCallback onFollow;
  final VoidCallback onSave;
  final VoidCallback onComment;
  final VoidCallback onShare;
  final VoidCallback onToggleMuted;
  final VoidCallback onTogglePaused;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        ColoredBox(
          color: AppColors.black,
          child: AppImage(
            post.mediaAsset,
            fit: BoxFit.cover,
            cacheWidth: 1200,
            filterQuality: FilterQuality.medium,
            errorBuilder: (_, _, _) => const ColoredBox(
              color: AppColors.surface,
              child: Icon(
                Icons.pets_rounded,
                color: AppColors.textDisabled,
                size: AppSpacing.section,
              ),
            ),
          ),
        ),
        DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topCenter,
              end: Alignment.bottomCenter,
              stops: [0, 0.38, 0.7, 1],
              colors: [
                AppColors.overlayLight,
                AppColors.transparent,
                AppColors.overlayLight,
                AppColors.overlayHeavy,
              ],
            ),
          ),
        ),
        GestureDetector(
          behavior: HitTestBehavior.translucent,
          onTap: onTogglePaused,
          onDoubleTap: onDoubleTap,
        ),
        Positioned(
          top: AppSpacing.section + AppSpacing.lg,
          right: AppSpacing.lg,
          child: IconButton.filledTonal(
            tooltip: isMuted ? 'Unmute' : 'Mute',
            onPressed: onToggleMuted,
            style: IconButton.styleFrom(
              backgroundColor: AppColors.overlayLight,
              foregroundColor: AppColors.white,
            ),
            icon: Icon(
              isMuted ? Icons.volume_off_rounded : Icons.volume_up_rounded,
            ),
          ),
        ),
        Center(
          child: IgnorePointer(
            child: AnimatedScale(
              scale: showHeart ? 1 : 0.72,
              duration: const Duration(milliseconds: 180),
              curve: Curves.easeOutBack,
              child: AnimatedOpacity(
                opacity: showHeart ? 1 : 0,
                duration: const Duration(milliseconds: 150),
                child: const Icon(
                  Icons.favorite_rounded,
                  color: AppColors.white,
                  size: 104,
                  shadows: [Shadow(color: AppColors.black, blurRadius: 24)],
                ),
              ),
            ),
          ),
        ),
        Center(
          child: IgnorePointer(
            child: AnimatedScale(
              scale: isPaused ? 1 : 0.8,
              duration: const Duration(milliseconds: 180),
              child: AnimatedOpacity(
                opacity: isPaused ? 1 : 0,
                duration: const Duration(milliseconds: 180),
                child: Container(
                  width: AppSpacing.section,
                  height: AppSpacing.section,
                  decoration: BoxDecoration(
                    color: AppColors.overlay,
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(
                    Icons.play_arrow_rounded,
                    color: AppColors.white,
                    size: AppSpacing.huge,
                  ),
                ),
              ),
            ),
          ),
        ),
        Positioned(
          left: AppSpacing.lg,
          right: AppSpacing.section + AppSpacing.xxl,
          bottom: AppSpacing.xxl,
          child: SafeArea(
            top: false,
            child: _ReelDetails(
              post: post,
              isFollowing: isFollowing,
              showFollow: showFollow,
              onFollow: onFollow,
            ),
          ),
        ),
        Positioned(
          right: AppSpacing.sm,
          bottom: AppSpacing.xxl,
          child: SafeArea(
            top: false,
            child: _ReelActions(
              post: post,
              isLiked: isLiked,
              isSaved: isSaved,
              likeCount: likeCount,
              onLike: onLike,
              onComment: onComment,
              onShare: onShare,
              onSave: onSave,
            ),
          ),
        ),
        Positioned(
          left: AppSpacing.lg,
          right: AppSpacing.lg,
          bottom: AppSpacing.xs,
          child: ClipRRect(
            borderRadius: AppRadius.pill,
            child: LinearProgressIndicator(
              value: totalReels == 0 ? 0 : reelNumber / totalReels,
              minHeight: AppSpacing.xxs,
              color: AppColors.white,
              backgroundColor: AppColors.glassBorder,
            ),
          ),
        ),
      ],
    );
  }
}

class _ReelDetails extends StatelessWidget {
  const _ReelDetails({
    required this.post,
    required this.isFollowing,
    required this.showFollow,
    required this.onFollow,
  });

  final FeedPost post;
  final bool isFollowing;
  final bool showFollow;
  final VoidCallback onFollow;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Row(
          children: [
            Container(
              width: AppSpacing.huge,
              height: AppSpacing.huge,
              padding: const EdgeInsets.all(AppSpacing.xxs),
              decoration: const BoxDecoration(
                gradient: AppColors.primaryGradient,
                shape: BoxShape.circle,
              ),
              child: ClipOval(
                child: AppImage(
                  post.avatarAsset,
                  fit: BoxFit.cover,
                  cacheWidth: 180,
                  errorBuilder: (_, _, _) => const ColoredBox(
                    color: AppColors.surface,
                    child: Icon(
                      Icons.pets_rounded,
                      color: AppColors.white,
                      size: AppSpacing.xl,
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Flexible(
              child: Text(
                post.ownerHandle,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.labelMedium.copyWith(
                  color: AppColors.white,
                  shadows: const [
                    Shadow(color: AppColors.black, blurRadius: 8),
                  ],
                ),
              ),
            ),
            if (showFollow) ...[
              const SizedBox(width: AppSpacing.sm),
              SizedBox(
                height: AppSpacing.xxxl,
                child: isFollowing
                    ? TextButton(
                        onPressed: onFollow,
                        style: TextButton.styleFrom(
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppSpacing.sm,
                          ),
                          foregroundColor: AppColors.textSecondary,
                        ),
                        child: const Text('Following'),
                      )
                    : OutlinedButton(
                        onPressed: onFollow,
                        style: OutlinedButton.styleFrom(
                          minimumSize: Size.zero,
                          padding: const EdgeInsets.symmetric(
                            horizontal: AppSpacing.md,
                          ),
                          foregroundColor: AppColors.white,
                          side: const BorderSide(color: AppColors.white),
                        ),
                        child: const Text('Follow'),
                      ),
              ),
            ],
          ],
        ),
        const SizedBox(height: AppSpacing.md),
        Text(
          post.caption,
          maxLines: 3,
          overflow: TextOverflow.ellipsis,
          style: AppTextStyles.bodySmall.copyWith(
            color: AppColors.white,
            shadows: const [Shadow(color: AppColors.black, blurRadius: 8)],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        Row(
          children: [
            const Icon(
              Icons.music_note_rounded,
              size: AppSpacing.lg,
              color: AppColors.white,
            ),
            const SizedBox(width: AppSpacing.xs),
            Flexible(
              child: Text(
                'PetConnect original · ${post.location}',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.caption.copyWith(
                  color: AppColors.white.withValues(alpha: 0.86),
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }
}

class _ReelActions extends StatelessWidget {
  const _ReelActions({
    required this.post,
    required this.isLiked,
    required this.isSaved,
    required this.likeCount,
    required this.onLike,
    required this.onComment,
    required this.onShare,
    required this.onSave,
  });

  final FeedPost post;
  final bool isLiked;
  final bool isSaved;
  final int likeCount;
  final VoidCallback onLike;
  final VoidCallback onComment;
  final VoidCallback onShare;
  final VoidCallback onSave;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _ReelActionButton(
          tooltip: isLiked ? 'Unlike reel' : 'Like reel',
          icon: isLiked
              ? Icons.favorite_rounded
              : Icons.favorite_border_rounded,
          color: isLiked ? AppColors.accent : AppColors.white,
          label: _formatCount(likeCount),
          onPressed: onLike,
        ),
        const SizedBox(height: AppSpacing.sm),
        _ReelActionButton(
          tooltip: 'View comments',
          icon: Icons.mode_comment_outlined,
          label: _formatCount(post.comments),
          onPressed: onComment,
        ),
        const SizedBox(height: AppSpacing.sm),
        _ReelActionButton(
          tooltip: 'Share reel',
          icon: Icons.send_rounded,
          label: 'Share',
          onPressed: onShare,
        ),
        const SizedBox(height: AppSpacing.sm),
        _ReelActionButton(
          tooltip: isSaved ? 'Remove saved reel' : 'Save reel',
          icon: isSaved
              ? Icons.bookmark_rounded
              : Icons.bookmark_border_rounded,
          color: isSaved ? AppColors.warning : AppColors.white,
          label: isSaved ? 'Saved' : 'Save',
          onPressed: onSave,
        ),
      ],
    );
  }
}

class _ReelActionButton extends StatelessWidget {
  const _ReelActionButton({
    required this.tooltip,
    required this.icon,
    required this.label,
    required this.onPressed,
    this.color = AppColors.white,
  });

  final String tooltip;
  final IconData icon;
  final String label;
  final VoidCallback onPressed;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        IconButton(
          tooltip: tooltip,
          onPressed: onPressed,
          style: IconButton.styleFrom(
            backgroundColor: AppColors.overlayLight,
            foregroundColor: color,
          ),
          icon: Icon(icon, size: AppSpacing.xxxl),
        ),
        Text(
          label,
          style: AppTextStyles.labelSmall.copyWith(
            color: AppColors.white,
            shadows: const [Shadow(color: AppColors.black, blurRadius: 8)],
          ),
        ),
      ],
    );
  }
}

class _CommentsSheet extends StatefulWidget {
  const _CommentsSheet({required this.post});

  final FeedPost post;

  @override
  State<_CommentsSheet> createState() => _CommentsSheetState();
}

class _CommentsSheetState extends State<_CommentsSheet> {
  final TextEditingController _controller = TextEditingController();
  final List<String> _localComments = <String>[];

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _sendComment() {
    final value = _controller.text.trim();
    if (value.isEmpty) return;
    setState(() => _localComments.add(value));
    _controller.clear();
    FocusManager.instance.primaryFocus?.unfocus();
  }

  @override
  Widget build(BuildContext context) {
    final sampleComments = <({String name, String text})>[
      (name: 'Maya & Coco', text: 'That happy face made my day!'),
      (name: 'Noah & Milo', text: 'We need a playdate soon.'),
      (name: 'Sara & Simba', text: 'The energy is unmatched.'),
    ];

    return Padding(
      padding: EdgeInsets.only(
        left: AppSpacing.lg,
        top: AppSpacing.lg,
        right: AppSpacing.lg,
        bottom: MediaQuery.viewInsetsOf(context).bottom + AppSpacing.lg,
      ),
      child: Column(
        children: [
          Container(
            width: AppSpacing.huge,
            height: AppSpacing.xs,
            decoration: const BoxDecoration(
              color: AppColors.divider,
              borderRadius: AppRadius.pill,
            ),
          ),
          const SizedBox(height: AppSpacing.lg),
          Text(
            '${widget.post.comments + _localComments.length} comments',
            style: AppTextStyles.headingSmall,
          ),
          const SizedBox(height: AppSpacing.lg),
          Expanded(
            child: ListView(
              children: [
                ...sampleComments.map(
                  (comment) =>
                      _CommentTile(name: comment.name, text: comment.text),
                ),
                ..._localComments.map(
                  (comment) => _CommentTile(name: 'You', text: comment),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _controller,
                  textInputAction: TextInputAction.send,
                  onSubmitted: (_) => _sendComment(),
                  decoration: const InputDecoration(
                    hintText: 'Add a kind comment…',
                    prefixIcon: Icon(Icons.pets_outlined),
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              IconButton.filled(
                tooltip: 'Post comment',
                onPressed: _sendComment,
                icon: const Icon(Icons.arrow_upward_rounded),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _CommentTile extends StatelessWidget {
  const _CommentTile({required this.name, required this.text});

  final String name;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.lg),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: AppSpacing.huge,
            height: AppSpacing.huge,
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.16),
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Icons.pets_rounded,
              color: AppColors.primaryLight,
              size: AppSpacing.xl,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTextStyles.labelMedium),
                const SizedBox(height: AppSpacing.xs),
                Text(text, style: AppTextStyles.bodySmall),
              ],
            ),
          ),
          Text('now', style: AppTextStyles.caption),
        ],
      ),
    );
  }
}

String _formatCount(int value) {
  if (value >= 1000000) {
    return '${(value / 1000000).toStringAsFixed(1)}M';
  }
  if (value >= 1000) {
    final result = (value / 1000).toStringAsFixed(value >= 10000 ? 0 : 1);
    return '${result}K';
  }
  return '$value';
}
