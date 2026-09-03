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

class ChatScreen extends ConsumerStatefulWidget {
  const ChatScreen({super.key, required this.chatId});

  final String chatId;

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  final TextEditingController _messageController = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  final FocusNode _messageFocus = FocusNode();

  ChatPreview get _chat {
    for (final chat in ref.read(socialControllerProvider).chats) {
      if (chat.id == widget.chatId) return chat;
    }
    return const ChatPreview(
      id: '',
      petName: 'New match',
      ownerName: 'PetConnect member',
      imageAsset: '',
      lastMessage: '',
      time: '',
    );
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      final controller = ref.read(socialControllerProvider.notifier);
      await controller.loadChat(widget.chatId);
      await controller.markChatRead(widget.chatId);
      if (mounted) _scrollToEnd();
    });
  }

  @override
  void dispose() {
    _messageController.dispose();
    _scrollController.dispose();
    _messageFocus.dispose();
    super.dispose();
  }

  void _scrollToEnd() {
    if (!_scrollController.hasClients) return;
    _scrollController.animateTo(
      _scrollController.position.maxScrollExtent,
      duration: const Duration(milliseconds: 260),
      curve: Curves.easeOutCubic,
    );
  }

  void _sendMessage() {
    final text = _messageController.text.trim();
    if (text.isEmpty) return;
    ref
        .read(socialControllerProvider.notifier)
        .sendMessage(widget.chatId, text);
    _messageController.clear();
    _messageFocus.requestFocus();
  }

  void _showFeedback(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  void _showAttachmentPicker() {
    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (sheetContext) => SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.sm,
            AppSpacing.xl,
            AppSpacing.xxl,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                'Share with ${_chat.petName}',
                style: AppTextStyles.headingSmall,
              ),
              const SizedBox(height: AppSpacing.lg),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceAround,
                children: [
                  _AttachmentOption(
                    icon: Icons.photo_library_outlined,
                    label: 'Gallery',
                    color: AppColors.primaryLight,
                    onTap: () {
                      Navigator.of(sheetContext).pop();
                      _showFeedback('Gallery access will be available soon.');
                    },
                  ),
                  _AttachmentOption(
                    icon: Icons.camera_alt_outlined,
                    label: 'Camera',
                    color: AppColors.accent,
                    onTap: () {
                      Navigator.of(sheetContext).pop();
                      _showFeedback('Camera access will be available soon.');
                    },
                  ),
                  _AttachmentOption(
                    icon: Icons.location_on_outlined,
                    label: 'Location',
                    color: AppColors.success,
                    onTap: () {
                      Navigator.of(sheetContext).pop();
                      _showFeedback('Location sharing will be available soon.');
                    },
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final chat = _chat;
    final messages = ref.watch(
      socialControllerProvider.select(
        (state) => state.messagesFor(widget.chatId),
      ),
    );

    ref.listen<int>(
      socialControllerProvider.select(
        (state) => state.messagesFor(widget.chatId).length,
      ),
      (previous, next) {
        if (previous == null || next <= previous) return;
        WidgetsBinding.instance.addPostFrameCallback((_) => _scrollToEnd());
      },
    );

    return AppScaffold(
      safeArea: false,
      resizeToAvoidBottomInset: true,
      appBar: AppBar(
        leading: IconButton(
          tooltip: 'Back',
          onPressed: () {
            if (context.canPop()) {
              context.pop();
            } else {
              context.go(AppRoutes.home);
            }
          },
          icon: const Icon(Icons.arrow_back_rounded),
        ),
        titleSpacing: 0,
        title: _ChatTitle(chat: chat),
        actions: [
          IconButton(
            tooltip: 'Video call',
            onPressed: () =>
                _showFeedback('Video calling will be available soon.'),
            icon: const Icon(Icons.videocam_outlined),
          ),
          IconButton(
            tooltip: 'Conversation details',
            onPressed: () => _showFeedback(
              '${chat.petName} is ${chat.isOnline ? 'online now' : 'currently offline'}.',
            ),
            icon: const Icon(Icons.info_outline_rounded),
          ),
          const SizedBox(width: AppSpacing.xs),
        ],
      ),
      child: Column(
        children: [
          Expanded(
            child: _MessageList(
              controller: _scrollController,
              messages: messages,
              chat: chat,
              onPlaydateTap: () =>
                  _showFeedback('Playdate details added to your draft.'),
            ),
          ),
          _MessageComposer(
            controller: _messageController,
            focusNode: _messageFocus,
            onSend: _sendMessage,
            onAttachmentTap: _showAttachmentPicker,
            onVoiceTap: () =>
                _showFeedback('Hold-to-record voice notes are coming soon.'),
          ),
        ],
      ),
    );
  }
}

class _ChatTitle extends StatelessWidget {
  const _ChatTitle({required this.chat});

  final ChatPreview chat;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Stack(
          children: [
            CircleAvatar(
              radius: 20,
              backgroundColor: AppColors.card,
              backgroundImage: appImageProvider(
                chat.imageAsset,
                cacheWidth: 120,
              ),
            ),
            if (chat.isOnline)
              Positioned(
                right: 0,
                bottom: 0,
                child: Container(
                  width: AppSpacing.sm,
                  height: AppSpacing.sm,
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
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                chat.petName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.labelLarge,
              ),
              Text(
                chat.isOnline
                    ? 'Online • with ${chat.ownerName}'
                    : chat.ownerName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTextStyles.caption.copyWith(
                  color: chat.isOnline
                      ? AppColors.success
                      : AppColors.textSecondary,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _MessageList extends StatelessWidget {
  const _MessageList({
    required this.controller,
    required this.messages,
    required this.chat,
    required this.onPlaydateTap,
  });

  final ScrollController controller;
  final List<ChatMessage> messages;
  final ChatPreview chat;
  final VoidCallback onPlaydateTap;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 840),
        child: ListView.builder(
          controller: controller,
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          physics: const BouncingScrollPhysics(),
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.md,
            AppSpacing.lg,
            AppSpacing.xxl,
          ),
          itemCount: messages.length + 2,
          itemBuilder: (context, index) {
            if (index == 0) {
              return const _DaySeparator(label: 'Today');
            }

            final contentIndex = index - 1;
            final suggestionIndex = messages.length < 3 ? messages.length : 3;
            if (contentIndex == suggestionIndex) {
              return _PlaydateSuggestion(
                petName: chat.petName,
                onTap: onPlaydateTap,
              );
            }

            final messageIndex = contentIndex > suggestionIndex
                ? contentIndex - 1
                : contentIndex;
            final message = messages[messageIndex];
            return _MessageBubble(
              message: message,
              showReadReceipt:
                  message.isMine && messageIndex == messages.length - 1,
            );
          },
        ),
      ),
    );
  }
}

class _DaySeparator extends StatelessWidget {
  const _DaySeparator({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
      child: Row(
        children: [
          const Expanded(child: Divider()),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
            child: Text(label, style: AppTextStyles.caption),
          ),
          const Expanded(child: Divider()),
        ],
      ),
    );
  }
}

class _MessageBubble extends StatelessWidget {
  const _MessageBubble({required this.message, required this.showReadReceipt});

  final ChatMessage message;
  final bool showReadReceipt;

  @override
  Widget build(BuildContext context) {
    final isMine = message.isMine;
    final availableWidth = MediaQuery.sizeOf(context).width;

    return Semantics(
      label: '${isMine ? 'You' : 'They'} said ${message.text}, ${message.time}',
      child: Align(
        alignment: isMine ? Alignment.centerRight : Alignment.centerLeft,
        child: Container(
          constraints: BoxConstraints(maxWidth: availableWidth * 0.76),
          margin: const EdgeInsets.only(top: AppSpacing.sm),
          padding: const EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.md,
            AppSpacing.md,
            AppSpacing.sm,
          ),
          decoration: BoxDecoration(
            color: isMine ? AppColors.primary : AppColors.card,
            borderRadius: BorderRadius.only(
              topLeft: AppRadius.lg.topLeft,
              topRight: AppRadius.lg.topRight,
              bottomLeft: isMine
                  ? AppRadius.lg.bottomLeft
                  : AppRadius.xs.bottomLeft,
              bottomRight: isMine
                  ? AppRadius.xs.bottomRight
                  : AppRadius.lg.bottomRight,
            ),
            border: isMine ? null : Border.all(color: AppColors.border),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  message.text,
                  style: AppTextStyles.bodyMedium.copyWith(
                    color: isMine ? AppColors.white : AppColors.textPrimary,
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.xs),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    message.time,
                    style: AppTextStyles.caption.copyWith(
                      color: isMine
                          ? AppColors.white.withValues(alpha: 0.72)
                          : AppColors.textDisabled,
                    ),
                  ),
                  if (showReadReceipt) ...[
                    const SizedBox(width: AppSpacing.xs),
                    const Icon(
                      Icons.done_all_rounded,
                      size: 15,
                      color: AppColors.white,
                    ),
                  ],
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _PlaydateSuggestion extends StatelessWidget {
  const _PlaydateSuggestion({required this.petName, required this.onTap});

  final String petName;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.primary.withValues(alpha: 0.10),
        borderRadius: AppRadius.lg,
        border: Border.all(color: AppColors.primary.withValues(alpha: 0.30)),
      ),
      child: Row(
        children: [
          Container(
            width: AppSpacing.giant,
            height: AppSpacing.giant,
            decoration: BoxDecoration(
              color: AppColors.primary.withValues(alpha: 0.18),
              shape: BoxShape.circle,
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
                Text('Plan a playdate', style: AppTextStyles.labelMedium),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  'Find a time for Luna and $petName',
                  style: AppTextStyles.caption,
                ),
              ],
            ),
          ),
          TextButton(onPressed: onTap, child: const Text('Plan')),
        ],
      ),
    );
  }
}

class _MessageComposer extends StatelessWidget {
  const _MessageComposer({
    required this.controller,
    required this.focusNode,
    required this.onSend,
    required this.onAttachmentTap,
    required this.onVoiceTap,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final VoidCallback onSend;
  final VoidCallback onAttachmentTap;
  final VoidCallback onVoiceTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: AppColors.surface,
      child: SafeArea(
        top: false,
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 840),
            child: Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.sm,
                AppSpacing.sm,
                AppSpacing.md,
                AppSpacing.sm,
              ),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  IconButton(
                    tooltip: 'Add attachment',
                    onPressed: onAttachmentTap,
                    color: AppColors.primaryLight,
                    icon: const Icon(Icons.add_circle_outline_rounded),
                  ),
                  Expanded(
                    child: TextField(
                      controller: controller,
                      focusNode: focusNode,
                      minLines: 1,
                      maxLines: 5,
                      textCapitalization: TextCapitalization.sentences,
                      keyboardType: TextInputType.multiline,
                      textInputAction: TextInputAction.newline,
                      decoration: const InputDecoration(
                        hintText: 'Message…',
                        isDense: true,
                        contentPadding: EdgeInsets.symmetric(
                          horizontal: AppSpacing.lg,
                          vertical: AppSpacing.md,
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  ValueListenableBuilder<TextEditingValue>(
                    valueListenable: controller,
                    builder: (context, value, child) {
                      final hasText = value.text.trim().isNotEmpty;
                      return SizedBox.square(
                        dimension: AppSpacing.giant,
                        child: IconButton.filled(
                          tooltip: hasText ? 'Send message' : 'Voice message',
                          onPressed: hasText ? onSend : onVoiceTap,
                          style: IconButton.styleFrom(
                            backgroundColor: hasText
                                ? AppColors.primary
                                : AppColors.card,
                            foregroundColor: hasText
                                ? AppColors.white
                                : AppColors.textSecondary,
                          ),
                          icon: Icon(
                            hasText
                                ? Icons.send_rounded
                                : Icons.mic_none_rounded,
                            size: 21,
                          ),
                        ),
                      );
                    },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _AttachmentOption extends StatelessWidget {
  const _AttachmentOption({
    required this.icon,
    required this.label,
    required this.color,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final Color color;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: InkWell(
        onTap: onTap,
        borderRadius: AppRadius.lg,
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.sm),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: AppSpacing.massive,
                height: AppSpacing.massive,
                decoration: BoxDecoration(
                  color: color.withValues(alpha: 0.14),
                  shape: BoxShape.circle,
                ),
                child: Icon(icon, color: color),
              ),
              const SizedBox(height: AppSpacing.sm),
              Text(label, style: AppTextStyles.labelSmall),
            ],
          ),
        ),
      ),
    );
  }
}
