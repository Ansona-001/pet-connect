import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../application/social_controller.dart';
import '../../data/social_repository.dart';

/// Lets the owner compose a post from one or more photos/videos, in a
/// reorderable order, with a per-item upload progress indicator — brief
/// Milestone 3's "Post composer" day. Each item is uploaded individually
/// (server/internal/modules/media/handler.go's `/media/uploads`, which
/// already re-encodes images and strips EXIF) before the post itself is
/// created from the resulting ordered media ids
/// (SocialRepository.createCarouselPost) — a single photo is just a
/// one-item carousel, there's no separate "simple" code path.
class PostComposerScreen extends ConsumerStatefulWidget {
  const PostComposerScreen({super.key});

  @override
  ConsumerState<PostComposerScreen> createState() => _PostComposerScreenState();
}

/// The server's own limits (media/handler.go's maxCarouselItems,
/// maxImageUploadBytes) mirrored here purely so a picked file can be
/// rejected instantly instead of only after a round trip — the server
/// remains the actual authority and re-checks all of this itself.
const _maxItems = 10;
const _maxImageBytes = 15 * 1024 * 1024;
const _maxUploadBytes = 50 * 1024 * 1024;
const _videoExtensions = {'mp4', 'mov', 'webm', 'm4v', 'avi', 'mkv'};

enum _UploadStatus { pending, uploading, done, failed }

class _ComposerItem {
  _ComposerItem({
    required this.file,
    required this.bytes,
    required this.isVideo,
  });

  final XFile file;
  final Uint8List bytes;
  final bool isVideo;
  double progress = 0;
  _UploadStatus status = _UploadStatus.pending;
  String? mediaId;
}

class _PostComposerScreenState extends ConsumerState<PostComposerScreen> {
  final _captionController = TextEditingController();
  final List<_ComposerItem> _items = [];
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _captionController.dispose();
    super.dispose();
  }

  bool _isVideoFile(XFile file) {
    final mimeType = file.mimeType;
    if (mimeType != null) return mimeType.startsWith('video/');
    final extension = file.name.toLowerCase().split('.').last;
    return _videoExtensions.contains(extension);
  }

  Future<void> _pickMedia() async {
    if (_items.length >= _maxItems) {
      _showError('You can add up to $_maxItems photos or videos.');
      return;
    }
    final picked = await ImagePicker().pickMultipleMedia(imageQuality: 88);
    if (!mounted || picked.isEmpty) return;

    final remaining = _maxItems - _items.length;
    final accepted = <_ComposerItem>[];
    for (final file in picked.take(remaining)) {
      final bytes = await file.readAsBytes();
      final isVideo = _isVideoFile(file);
      final limit = isVideo ? _maxUploadBytes : _maxImageBytes;
      if (bytes.length > limit) {
        _showError(
          '${file.name} is too large (${isVideo ? 'videos' : 'images'} must be under ${limit ~/ (1024 * 1024)}MB).',
        );
        continue;
      }
      accepted.add(_ComposerItem(file: file, bytes: bytes, isVideo: isVideo));
    }
    if (picked.length > remaining) {
      _showError(
        'Only the first $remaining item(s) were added ($_maxItems max per post).',
      );
    }
    if (!mounted) return;
    setState(() => _items.addAll(accepted));
  }

  void _removeItem(int index) {
    setState(() => _items.removeAt(index));
  }

  // onReorderItem (unlike the deprecated onReorder) already adjusts
  // newIndex for the removed item at oldIndex, so no manual `-1` here.
  void _reorder(int oldIndex, int newIndex) {
    setState(() {
      final item = _items.removeAt(oldIndex);
      _items.insert(newIndex, item);
    });
  }

  void _showError(String message) {
    setState(() => _error = message);
  }

  Future<void> _submit() async {
    if (_items.isEmpty || _submitting) return;
    setState(() {
      _submitting = true;
      _error = null;
    });

    final repository = ref.read(socialRepositoryProvider);
    final mediaIds = <String>[];
    for (final item in _items) {
      if (item.mediaId != null) {
        mediaIds.add(item.mediaId!);
        continue;
      }
      setState(() => item.status = _UploadStatus.uploading);
      try {
        final uploaded = await repository.uploadMedia(
          fileName: item.file.name,
          bytes: item.bytes,
          isVideo: item.isVideo,
          onProgress: (sent, total) {
            if (!mounted || total <= 0) return;
            setState(() => item.progress = sent / total);
          },
        );
        item.mediaId = uploaded.id;
        setState(() => item.status = _UploadStatus.done);
        mediaIds.add(uploaded.id);
      } catch (error) {
        setState(() {
          item.status = _UploadStatus.failed;
          _submitting = false;
          _error = 'One of your files could not be uploaded. Try again.';
        });
        return;
      }
    }

    final created = await ref
        .read(socialControllerProvider.notifier)
        .createCarouselPost(
          caption: _captionController.text,
          mediaIds: mediaIds,
        );
    if (!mounted) return;
    if (created) {
      ScaffoldMessenger.of(context)
        ..hideCurrentSnackBar()
        ..showSnackBar(const SnackBar(content: Text('Your post is live.')));
      context.pop();
      return;
    }
    setState(() {
      _submitting = false;
      _error =
          ref.read(socialControllerProvider).error ??
          'The post could not be created.';
    });
  }

  @override
  Widget build(BuildContext context) {
    final canSubmit = _items.isNotEmpty && !_submitting;
    return AppScaffold(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              IconButton(
                tooltip: 'Cancel',
                onPressed: _submitting ? null : () => context.pop(),
                icon: const Icon(Icons.close_rounded),
              ),
              Expanded(
                child: Text('New post', style: AppTextStyles.headingLarge),
              ),
              TextButton(
                onPressed: canSubmit ? _submit : null,
                child: _submitting
                    ? const SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(strokeWidth: 2),
                      )
                    : const Text('Share'),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          Expanded(
            child: SingleChildScrollView(
              physics: const BouncingScrollPhysics(),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (_items.isEmpty)
                    _EmptyPicker(onTap: _submitting ? null : _pickMedia)
                  else ...[
                    SizedBox(
                      height: 140,
                      child: ReorderableListView.builder(
                        scrollDirection: Axis.horizontal,
                        buildDefaultDragHandles: !_submitting,
                        itemCount: _items.length,
                        onReorderItem: _submitting ? (_, _) {} : _reorder,
                        itemBuilder: (context, index) => _ComposerTile(
                          key: ValueKey(_items[index].file.path),
                          item: _items[index],
                          onRemove: _submitting
                              ? null
                              : () => _removeItem(index),
                        ),
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    if (_items.length < _maxItems)
                      TextButton.icon(
                        onPressed: _submitting ? null : _pickMedia,
                        icon: const Icon(Icons.add_photo_alternate_outlined),
                        label: Text('Add more (${_items.length}/$_maxItems)'),
                      ),
                  ],
                  const SizedBox(height: AppSpacing.lg),
                  TextField(
                    controller: _captionController,
                    enabled: !_submitting,
                    maxLines: 4,
                    maxLength: 2200,
                    decoration: const InputDecoration(
                      hintText: 'Write a caption for your pack...',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  if (_error != null) ...[
                    const SizedBox(height: AppSpacing.sm),
                    Text(
                      _error!,
                      style: AppTextStyles.bodySmall.copyWith(
                        color: AppColors.error,
                      ),
                    ),
                  ],
                  const SizedBox(height: AppSpacing.lg),
                  PrimaryButton(
                    text: _submitting ? 'Sharing...' : 'Share post',
                    icon: Icons.check_rounded,
                    isLoading: _submitting,
                    enabled: canSubmit,
                    onPressed: canSubmit ? _submit : null,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _EmptyPicker extends StatelessWidget {
  const _EmptyPicker({required this.onTap});

  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: AppRadius.lg,
      child: Container(
        height: 220,
        width: double.infinity,
        decoration: BoxDecoration(
          color: AppColors.card,
          borderRadius: AppRadius.lg,
          border: Border.all(color: AppColors.border),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.add_photo_alternate_outlined,
              size: 48,
              color: AppColors.textSecondary,
            ),
            const SizedBox(height: AppSpacing.sm),
            Text(
              'Add photos or videos',
              style: AppTextStyles.bodyMedium.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ComposerTile extends StatelessWidget {
  const _ComposerTile({super.key, required this.item, required this.onRemove});

  final _ComposerItem item;
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(right: AppSpacing.sm),
      child: SizedBox(
        width: 110,
        child: Stack(
          fit: StackFit.expand,
          children: [
            ClipRRect(
              borderRadius: AppRadius.md,
              child: item.isVideo
                  ? Container(
                      color: AppColors.elevatedSurface,
                      child: const Icon(
                        Icons.movie_creation_outlined,
                        size: 36,
                        color: AppColors.textSecondary,
                      ),
                    )
                  : Image.memory(item.bytes, fit: BoxFit.cover),
            ),
            if (item.status == _UploadStatus.uploading)
              DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.overlay,
                  borderRadius: AppRadius.md,
                ),
                child: Center(
                  child: CircularProgressIndicator(
                    value: item.progress > 0 ? item.progress : null,
                    color: AppColors.white,
                    strokeWidth: 3,
                  ),
                ),
              ),
            if (item.status == _UploadStatus.failed)
              DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.overlay,
                  borderRadius: AppRadius.md,
                ),
                child: const Center(
                  child: Icon(
                    Icons.error_outline_rounded,
                    color: AppColors.white,
                  ),
                ),
              ),
            if (onRemove != null)
              Positioned(
                top: 4,
                right: 4,
                child: GestureDetector(
                  onTap: onRemove,
                  child: const CircleAvatar(
                    radius: 12,
                    backgroundColor: Colors.black54,
                    child: Icon(
                      Icons.close_rounded,
                      size: 14,
                      color: Colors.white,
                    ),
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }
}
