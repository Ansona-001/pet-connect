import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../application/social_controller.dart';
import '../../domain/social_models.dart';

class InboxScreen extends ConsumerStatefulWidget {
  const InboxScreen({super.key});

  @override
  ConsumerState<InboxScreen> createState() => _InboxScreenState();
}

class _InboxScreenState extends ConsumerState<InboxScreen> {
  final TextEditingController _searchController = TextEditingController();
  bool _showUnreadOnly = false;

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _openChat(String chatId) {
    ref.read(socialControllerProvider.notifier).markChatRead(chatId);
    context.push(AppRoutes.chat(chatId));
  }

  void _showFeedback(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  List<ChatPreview> _filteredChats(SocialState socialState) {
    final query = _searchController.text.trim().toLowerCase();
    return socialState.chats
        .map((chat) {
          final messages = socialState.messagesFor(chat.id);
          final latest = messages.isEmpty ? null : messages.last;
          return ChatPreview(
            id: chat.id,
            petName: chat.petName,
            ownerName: chat.ownerName,
            imageAsset: chat.imageAsset,
            lastMessage: latest?.text ?? chat.lastMessage,
            time: latest?.time ?? chat.time,
            unreadCount: socialState.readChatIds.contains(chat.id)
                ? 0
                : chat.unreadCount,
            isOnline: chat.isOnline,
          );
        })
        .where((chat) {
          final matchesQuery =
              query.isEmpty ||
              chat.petName.toLowerCase().contains(query) ||
              chat.ownerName.toLowerCase().contains(query) ||
              chat.lastMessage.toLowerCase().contains(query);
          final matchesUnread = !_showUnreadOnly || chat.unreadCount > 0;
          return matchesQuery && matchesUnread;
        })
        .toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final socialState = ref.watch(socialControllerProvider);
    final chats = _filteredChats(socialState);

    return AppScaffold(
      child: LayoutBuilder(
        builder: (context, constraints) => Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: ListView(
              keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
              physics: const BouncingScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.sm,
                AppSpacing.lg,
                AppSpacing.xxxl,
              ),
              children: [
                _InboxHeader(
                  onBack: () {
                    if (context.canPop()) {
                      context.pop();
                    } else {
                      context.go(AppRoutes.home);
                    }
                  },
                  onComposeTap: () =>
                      _showFeedback('Choose a new match to start chatting.'),
                ),
                const SizedBox(height: AppSpacing.xl),
                _SearchBar(
                  controller: _searchController,
                  unreadOnly: _showUnreadOnly,
                  onChanged: (_) => setState(() {}),
                  onUnreadTap: () {
                    setState(() => _showUnreadOnly = !_showUnreadOnly);
                  },
                ),
                const SizedBox(height: AppSpacing.xxl),
                _SectionTitle(
                  title: 'New matches',
                  trailing: 'See all',
                  onTrailingTap: () =>
                      _showFeedback('All your new friends are shown here.'),
                ),
                const SizedBox(height: AppSpacing.md),
                SizedBox(
                  height: 112,
                  child: ListView.separated(
                    scrollDirection: Axis.horizontal,
                    physics: const BouncingScrollPhysics(),
                    itemCount: socialState.chats.length,
                    separatorBuilder: (_, _) =>
                        const SizedBox(width: AppSpacing.lg),
                    itemBuilder: (context, index) {
                      final chat = socialState.chats[index];
                      return _NewMatch(
                        petName: chat.petName,
                        imageAsset: chat.imageAsset,
                        isOnline: chat.isOnline,
                        isNew: index == 0,
                        onTap: () => _openChat(chat.id),
                      );
                    },
                  ),
                ),
                const SizedBox(height: AppSpacing.xxl),
                _SectionTitle(
                  title: _showUnreadOnly ? 'Unread messages' : 'Messages',
                  trailing: '${chats.length}',
                ),
                const SizedBox(height: AppSpacing.sm),
                if (chats.isEmpty)
                  _EmptyInbox(
                    hasQuery: _searchController.text.trim().isNotEmpty,
                    onReset: () {
                      _searchController.clear();
                      setState(() => _showUnreadOnly = false);
                    },
                  )
                else
                  ...chats.map(
                    (chat) => _ChatListTile(
                      chat: chat,
                      onTap: () => _openChat(chat.id),
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

class _InboxHeader extends StatelessWidget {
  const _InboxHeader({required this.onBack, required this.onComposeTap});

  final VoidCallback onBack;
  final VoidCallback onComposeTap;

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
              Text('Messages', style: AppTextStyles.headingLarge),
              const SizedBox(height: AppSpacing.xs),
              Text(
                'Plan the next great playdate',
                style: AppTextStyles.bodySmall,
              ),
            ],
          ),
        ),
        IconButton.filled(
          tooltip: 'New message',
          onPressed: onComposeTap,
          style: IconButton.styleFrom(
            backgroundColor: AppColors.primary,
            foregroundColor: AppColors.white,
          ),
          icon: const Icon(Icons.edit_square),
        ),
      ],
    );
  }
}

class _SearchBar extends StatelessWidget {
  const _SearchBar({
    required this.controller,
    required this.unreadOnly,
    required this.onChanged,
    required this.onUnreadTap,
  });

  final TextEditingController controller;
  final bool unreadOnly;
  final ValueChanged<String> onChanged;
  final VoidCallback onUnreadTap;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: TextField(
            controller: controller,
            onChanged: onChanged,
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              hintText: 'Search conversations',
              prefixIcon: const Icon(Icons.search_rounded),
              suffixIcon: controller.text.isEmpty
                  ? null
                  : IconButton(
                      tooltip: 'Clear search',
                      onPressed: () {
                        controller.clear();
                        onChanged('');
                      },
                      icon: const Icon(Icons.close_rounded),
                    ),
            ),
          ),
        ),
        const SizedBox(width: AppSpacing.sm),
        SizedBox.square(
          dimension: AppSpacing.massive,
          child: IconButton(
            tooltip: unreadOnly ? 'Show all messages' : 'Show unread messages',
            onPressed: onUnreadTap,
            style: IconButton.styleFrom(
              backgroundColor: unreadOnly ? AppColors.primary : AppColors.card,
              foregroundColor: unreadOnly
                  ? AppColors.white
                  : AppColors.textSecondary,
              side: BorderSide(
                color: unreadOnly ? AppColors.primary : AppColors.border,
              ),
            ),
            icon: const Icon(Icons.mark_chat_unread_outlined, size: 21),
          ),
        ),
      ],
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle({required this.title, this.trailing, this.onTrailingTap});

  final String title;
  final String? trailing;
  final VoidCallback? onTrailingTap;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(child: Text(title, style: AppTextStyles.headingSmall)),
        if (trailing != null)
          TextButton(
            onPressed: onTrailingTap,
            style: TextButton.styleFrom(
              foregroundColor: onTrailingTap == null
                  ? AppColors.textDisabled
                  : AppColors.primaryLight,
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.sm),
              minimumSize: const Size(AppSpacing.xxxl, AppSpacing.xxxl),
            ),
            child: Text(trailing!),
          ),
      ],
    );
  }
}

class _NewMatch extends StatelessWidget {
  const _NewMatch({
    required this.petName,
    required this.imageAsset,
    required this.isOnline,
    required this.isNew,
    required this.onTap,
  });

  final String petName;
  final String imageAsset;
  final bool isOnline;
  final bool isNew;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: 'Open chat with $petName',
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: SizedBox(
          width: 76,
          child: Column(
            children: [
              Stack(
                clipBehavior: Clip.none,
                children: [
                  Container(
                    width: AppSpacing.section,
                    height: AppSpacing.section,
                    padding: const EdgeInsets.all(AppSpacing.xxs),
                    decoration: const BoxDecoration(
                      gradient: AppColors.primaryGradient,
                      shape: BoxShape.circle,
                    ),
                    child: CircleAvatar(
                      backgroundColor: AppColors.card,
                      backgroundImage: appImageProvider(
                        imageAsset,
                        cacheWidth: 180,
                      ),
                    ),
                  ),
                  if (isOnline)
                    Positioned(
                      right: AppSpacing.xs,
                      bottom: AppSpacing.xs,
                      child: Container(
                        width: AppSpacing.md,
                        height: AppSpacing.md,
                        decoration: BoxDecoration(
                          color: AppColors.success,
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: AppColors.background,
                            width: AppSpacing.xxs,
                          ),
                        ),
                      ),
                    ),
                  if (isNew)
                    Positioned(
                      top: -AppSpacing.xs,
                      right: -AppSpacing.sm,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: AppSpacing.sm,
                          vertical: AppSpacing.xxs,
                        ),
                        decoration: const BoxDecoration(
                          color: AppColors.accent,
                          borderRadius: AppRadius.pill,
                        ),
                        child: Text(
                          'NEW',
                          style: AppTextStyles.overline.copyWith(
                            color: AppColors.white,
                            letterSpacing: 0.4,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(
                petName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.labelMedium,
              ),
              Text('Matched', style: AppTextStyles.caption),
            ],
          ),
        ),
      ),
    );
  }
}

class _ChatListTile extends StatelessWidget {
  const _ChatListTile({required this.chat, required this.onTap});

  final ChatPreview chat;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final hasUnread = chat.unreadCount > 0;

    return Semantics(
      button: true,
      label: 'Chat with ${chat.petName}, ${chat.unreadCount} unread messages',
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
          child: Row(
            children: [
              Stack(
                children: [
                  CircleAvatar(
                    radius: 29,
                    backgroundColor: AppColors.card,
                    backgroundImage: appImageProvider(
                      chat.imageAsset,
                      cacheWidth: 180,
                    ),
                  ),
                  if (chat.isOnline)
                    Positioned(
                      right: 0,
                      bottom: AppSpacing.xs,
                      child: Container(
                        width: AppSpacing.md,
                        height: AppSpacing.md,
                        decoration: BoxDecoration(
                          color: AppColors.success,
                          shape: BoxShape.circle,
                          border: Border.all(
                            color: AppColors.background,
                            width: AppSpacing.xxs,
                          ),
                        ),
                      ),
                    ),
                ],
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            chat.petName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style:
                                (hasUnread
                                        ? AppTextStyles.labelLarge
                                        : AppTextStyles.bodyMedium)
                                    .copyWith(color: AppColors.textPrimary),
                          ),
                        ),
                        Text(
                          chat.time,
                          style: AppTextStyles.caption.copyWith(
                            color: hasUnread
                                ? AppColors.primaryLight
                                : AppColors.textDisabled,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: AppSpacing.xs),
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            '${chat.ownerName} • ${chat.lastMessage}',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTextStyles.bodySmall.copyWith(
                              color: hasUnread
                                  ? AppColors.textPrimary
                                  : AppColors.textSecondary,
                              fontWeight: hasUnread
                                  ? FontWeight.w600
                                  : FontWeight.w400,
                            ),
                          ),
                        ),
                        if (hasUnread) ...[
                          const SizedBox(width: AppSpacing.sm),
                          Container(
                            constraints: const BoxConstraints(
                              minWidth: AppSpacing.xl,
                              minHeight: AppSpacing.xl,
                            ),
                            padding: const EdgeInsets.symmetric(
                              horizontal: AppSpacing.xs,
                            ),
                            alignment: Alignment.center,
                            decoration: const BoxDecoration(
                              color: AppColors.primary,
                              borderRadius: AppRadius.pill,
                            ),
                            child: Text(
                              '${chat.unreadCount}',
                              style: AppTextStyles.labelSmall.copyWith(
                                color: AppColors.white,
                              ),
                            ),
                          ),
                        ],
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _EmptyInbox extends StatelessWidget {
  const _EmptyInbox({required this.hasQuery, required this.onReset});

  final bool hasQuery;
  final VoidCallback onReset;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.huge),
      child: Column(
        children: [
          Container(
            width: AppSpacing.section,
            height: AppSpacing.section,
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.12),
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Icons.chat_bubble_outline_rounded,
              color: AppColors.primaryLight,
            ),
          ),
          const SizedBox(height: AppSpacing.lg),
          Text(
            hasQuery ? 'No conversations found' : 'No unread messages',
            style: AppTextStyles.headingSmall,
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(
            hasQuery
                ? 'Try another pet or owner name.'
                : 'You’re all caught up for now.',
            textAlign: TextAlign.center,
            style: AppTextStyles.bodySmall,
          ),
          const SizedBox(height: AppSpacing.md),
          TextButton(
            onPressed: onReset,
            child: const Text('Show all messages'),
          ),
        ],
      ),
    );
  }
}
