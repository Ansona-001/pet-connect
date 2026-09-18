import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../core/network/api_exception.dart';
import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_radius.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../../social/application/social_controller.dart';
import '../../data/pets_repository.dart';
import '../../domain/pet.dart';

/// Lists every pet the owner has, lets them switch which one is active
/// app-wide (reusing SocialController.selectActivePet), and is the entry
/// point for add/edit/delete — brief Milestone 1: "Multi-pet add/edit/
/// delete/media UI with consistent active-pet selection". Reached from
/// Profile's owner card.
class MyPetsScreen extends ConsumerStatefulWidget {
  const MyPetsScreen({super.key});

  @override
  ConsumerState<MyPetsScreen> createState() => _MyPetsScreenState();
}

class _MyPetsScreenState extends ConsumerState<MyPetsScreen> {
  List<Pet>? _pets;
  String? _error;
  bool _loading = true;
  String? _deletingId;

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
      final pets = await ref.read(petsRepositoryProvider).listMine();
      if (!mounted) return;
      setState(() {
        _pets = pets;
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

  Future<void> _addPet() async {
    await context.push(AppRoutes.petForm);
    if (!mounted) return;
    await _load();
    await ref.read(socialControllerProvider.notifier).refreshPets();
  }

  Future<void> _editPet(Pet pet) async {
    await context.push('${AppRoutes.petForm}?petId=${pet.id}');
    if (!mounted) return;
    await _load();
    await ref.read(socialControllerProvider.notifier).refreshPets();
  }

  Future<void> _deletePet(Pet pet) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        backgroundColor: AppColors.surface,
        title: Text('Remove ${pet.name}?'),
        content: const Text(
          "This removes the pet from your profile and match candidates. "
          "It can't be undone from here.",
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('Remove'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    setState(() => _deletingId = pet.id);
    try {
      await ref.read(petsRepositoryProvider).delete(pet.id);
      if (!mounted) return;
      setState(() {
        _pets = _pets?.where((item) => item.id != pet.id).toList();
      });
      await ref.read(socialControllerProvider.notifier).refreshPets();
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
        ..hideCurrentSnackBar()
        ..showSnackBar(
          SnackBar(content: Text(ApiException.from(error).message)),
        );
    } finally {
      if (mounted) setState(() => _deletingId = null);
    }
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
    final activeIndex = ref.watch(
      socialControllerProvider.select((s) => s.activePetIndex),
    );
    final activePets = ref.watch(
      socialControllerProvider.select((s) => s.pets),
    );
    final activePetId = activeIndex < activePets.length
        ? activePets[activeIndex].id
        : null;

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
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('My pets', style: AppTextStyles.headingLarge),
                          const SizedBox(height: AppSpacing.xs),
                          Text(
                            'Manage your pets and who\'s active right now',
                            style: AppTextStyles.bodySmall,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.xl),
                ..._buildBody(activePetId),
              ],
            ),
          ),
        ),
      ),
    );
  }

  List<Widget> _buildBody(String? activePetId) {
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

    final pets = _pets ?? const [];
    final petsRepository = ref.read(petsRepositoryProvider);
    return [
      for (final pet in pets) ...[
        _PetTile(
          pet: pet,
          imageUrl: petsRepository.resolveMediaUrl(pet.primaryImageUrl),
          isActive: pet.id == activePetId,
          deleting: _deletingId == pet.id,
          onTap: () => _editPet(pet),
          onSetActive: pet.id == activePetId
              ? null
              : () {
                  final index = pets.indexOf(pet);
                  ref
                      .read(socialControllerProvider.notifier)
                      .selectActivePet(index);
                },
          onDelete: () => _deletePet(pet),
        ),
        const SizedBox(height: AppSpacing.md),
      ],
      if (pets.isEmpty)
        Text(
          "You haven't added a pet yet.",
          style: AppTextStyles.bodyMedium.copyWith(
            color: AppColors.textSecondary,
          ),
        ),
      const SizedBox(height: AppSpacing.lg),
      PrimaryButton(
        text: 'Add a pet',
        icon: Icons.add_rounded,
        onPressed: _addPet,
      ),
    ];
  }
}

class _PetTile extends StatelessWidget {
  const _PetTile({
    required this.pet,
    required this.imageUrl,
    required this.isActive,
    required this.deleting,
    required this.onTap,
    required this.onSetActive,
    required this.onDelete,
  });

  final Pet pet;
  final String imageUrl;
  final bool isActive;
  final bool deleting;
  final VoidCallback onTap;
  final VoidCallback? onSetActive;
  final VoidCallback onDelete;

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
            border: Border.all(
              color: isActive ? AppColors.primary : AppColors.border,
              width: isActive ? 1.5 : 1,
            ),
          ),
          child: Row(
            children: [
              ClipRRect(
                borderRadius: AppRadius.md,
                child: SizedBox(
                  width: 56,
                  height: 56,
                  child: AppImage(
                    imageUrl,
                    fit: BoxFit.cover,
                    errorBuilder: (_, _, _) => const ColoredBox(
                      color: AppColors.elevatedSurface,
                      child: Icon(
                        Icons.pets_rounded,
                        color: AppColors.textDisabled,
                      ),
                    ),
                  ),
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
                            pet.name,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTextStyles.bodyMedium,
                          ),
                        ),
                        if (isActive) ...[
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
                              'Active',
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
                      pet.breed.isEmpty ? pet.petType : pet.breed,
                      style: AppTextStyles.bodySmall.copyWith(
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              if (deleting)
                const Padding(
                  padding: EdgeInsets.all(10),
                  child: SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: AppColors.error,
                    ),
                  ),
                )
              else
                PopupMenuButton<String>(
                  onSelected: (value) {
                    if (value == 'activate' && onSetActive != null) {
                      onSetActive!();
                    } else if (value == 'delete') {
                      onDelete();
                    }
                  },
                  itemBuilder: (context) => [
                    if (onSetActive != null)
                      const PopupMenuItem(
                        value: 'activate',
                        child: Text('Set as active'),
                      ),
                    const PopupMenuItem(value: 'delete', child: Text('Remove')),
                  ],
                ),
            ],
          ),
        ),
      ),
    );
  }
}
