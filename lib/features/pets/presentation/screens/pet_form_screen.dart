import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../core/network/api_exception.dart';
import '../../../../core/routes/routes.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_text_styles.dart';
import '../../../../shared/widgets/app_image.dart';
import '../../../../shared/widgets/buttons/primary_button.dart';
import '../../../../shared/widgets/inputs/app_text_field.dart';
import '../../../../shared/widgets/layout/app_scaffold.dart';
import '../../data/pets_repository.dart';
import '../../domain/pet.dart';

const _personalityOptions = [
  'Playful',
  'Cuddly',
  'Energetic',
  'Calm',
  'Curious',
  'Independent',
  'Social',
  'Shy',
];

const _interestOptions = [
  'Playdates',
  'Parks',
  'Training',
  'Swimming',
  'Hiking',
  'Treats',
];

/// Add/edit a single pet — brief Milestone 1: "Multi-pet add/edit/delete/
/// media UI". A null [petId] means create; otherwise this loads and edits
/// that pet. Deletion lives on [MyPetsScreen] instead of duplicating it
/// here.
class PetFormScreen extends ConsumerStatefulWidget {
  const PetFormScreen({super.key, this.petId});

  final String? petId;

  bool get isEditing => petId != null;

  @override
  ConsumerState<PetFormScreen> createState() => _PetFormScreenState();
}

class _PetFormScreenState extends ConsumerState<PetFormScreen> {
  final _nameController = TextEditingController();
  final _breedController = TextEditingController();
  final _ageLabelController = TextEditingController();
  final _weightController = TextEditingController();
  final _bioController = TextEditingController();
  String _petType = petTypes.first;
  String _gender = petGenders.last;
  final Set<String> _personality = {};
  final Set<String> _interests = {};
  String _primaryImageUrl = '';

  bool _loading = false;
  bool _uploadingPhoto = false;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _nameController.addListener(() => setState(() {}));
    if (widget.isEditing) {
      _loadExisting();
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _breedController.dispose();
    _ageLabelController.dispose();
    _weightController.dispose();
    _bioController.dispose();
    super.dispose();
  }

  Future<void> _loadExisting() async {
    setState(() => _loading = true);
    try {
      final pets = await ref.read(petsRepositoryProvider).listMine();
      final pet = pets.firstWhere((item) => item.id == widget.petId);
      if (!mounted) return;
      setState(() {
        _nameController.text = pet.name;
        _breedController.text = pet.breed;
        _ageLabelController.text = pet.ageLabel;
        _weightController.text = pet.weightKg?.toString() ?? '';
        _bioController.text = pet.bio;
        _petType = petTypes.contains(pet.petType)
            ? pet.petType
            : petTypes.first;
        _gender = petGenders.contains(pet.gender)
            ? pet.gender
            : petGenders.last;
        _personality
          ..clear()
          ..addAll(pet.personality);
        _interests
          ..clear()
          ..addAll(pet.interests);
        _primaryImageUrl = pet.primaryImageUrl;
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

  bool get _canSave =>
      _nameController.text.trim().isNotEmpty && !_uploadingPhoto;

  Future<void> _pickPhoto() async {
    final image = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      imageQuality: 88,
      maxWidth: 1400,
    );
    if (!mounted || image == null) return;
    setState(() => _uploadingPhoto = true);
    try {
      final path = await ref
          .read(petsRepositoryProvider)
          .uploadPhoto(fileName: image.name, bytes: await image.readAsBytes());
      if (!mounted) return;
      setState(() => _primaryImageUrl = path);
    } catch (error) {
      if (!mounted) return;
      _showSnack(ApiException.from(error).message);
    } finally {
      if (mounted) setState(() => _uploadingPhoto = false);
    }
  }

  void _showSnack(String message) {
    ScaffoldMessenger.of(context)
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  Future<void> _save() async {
    setState(() {
      _saving = true;
      _error = null;
    });
    final repository = ref.read(petsRepositoryProvider);
    final weight = double.tryParse(_weightController.text.trim());
    try {
      if (widget.isEditing) {
        await repository.update(
          widget.petId!,
          name: _nameController.text,
          petType: _petType,
          breed: _breedController.text,
          ageLabel: _ageLabelController.text,
          gender: _gender,
          bio: _bioController.text,
          personality: _personality.toList(),
          interests: _interests.toList(),
          primaryImageUrl: _primaryImageUrl,
          weightKg: weight,
        );
      } else {
        await repository.create(
          name: _nameController.text,
          petType: _petType,
          breed: _breedController.text,
          ageLabel: _ageLabelController.text,
          gender: _gender,
          bio: _bioController.text,
          personality: _personality.toList(),
          interests: _interests.toList(),
          primaryImageUrl: _primaryImageUrl,
          weightKg: weight,
        );
      }
      if (!mounted) return;
      context.pop();
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _saving = false;
        _error = ApiException.from(error).message;
      });
    }
  }

  void _back() {
    if (context.canPop()) {
      context.pop();
    } else {
      context.go(AppRoutes.pets);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AppScaffold(
      child: LayoutBuilder(
        builder: (context, constraints) => Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: _loading
                ? const Center(
                    child: Padding(
                      padding: EdgeInsets.only(top: 120),
                      child: CircularProgressIndicator(
                        color: AppColors.primary,
                      ),
                    ),
                  )
                : ListView(
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
                          Text(
                            widget.isEditing ? 'Edit pet' : 'Add a pet',
                            style: AppTextStyles.headingLarge,
                          ),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.xl),
                      Center(
                        child: _PhotoPicker(
                          imageUrl: _primaryImageUrl.isEmpty
                              ? ''
                              : ref
                                    .read(petsRepositoryProvider)
                                    .resolveMediaUrl(_primaryImageUrl),
                          uploading: _uploadingPhoto,
                          onTap: _uploadingPhoto ? null : _pickPhoto,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.xl),
                      AppTextField(
                        controller: _nameController,
                        label: 'Pet name',
                        textInputAction: TextInputAction.next,
                        prefix: const Icon(
                          Icons.pets_outlined,
                          color: AppColors.textSecondary,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      Text('Pet type', style: AppTextStyles.labelLarge),
                      const SizedBox(height: AppSpacing.sm),
                      Wrap(
                        spacing: AppSpacing.sm,
                        runSpacing: AppSpacing.sm,
                        children: petTypes
                            .map(
                              (type) => _SelectableChip(
                                label: _titleCase(type),
                                selected: _petType == type,
                                onSelected: () =>
                                    setState(() => _petType = type),
                              ),
                            )
                            .toList(growable: false),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      Text('Gender', style: AppTextStyles.labelLarge),
                      const SizedBox(height: AppSpacing.sm),
                      Wrap(
                        spacing: AppSpacing.sm,
                        runSpacing: AppSpacing.sm,
                        children: petGenders
                            .map(
                              (gender) => _SelectableChip(
                                label: _titleCase(gender),
                                selected: _gender == gender,
                                onSelected: () =>
                                    setState(() => _gender = gender),
                              ),
                            )
                            .toList(growable: false),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      Row(
                        children: [
                          Expanded(
                            child: AppTextField(
                              controller: _breedController,
                              label: 'Breed',
                              textInputAction: TextInputAction.next,
                            ),
                          ),
                          const SizedBox(width: AppSpacing.md),
                          Expanded(
                            child: AppTextField(
                              controller: _ageLabelController,
                              label: 'Age (e.g. "2 years")',
                              textInputAction: TextInputAction.next,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      AppTextField(
                        controller: _weightController,
                        label: 'Weight (kg, optional)',
                        keyboardType: const TextInputType.numberWithOptions(
                          decimal: true,
                        ),
                        textInputAction: TextInputAction.next,
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      AppTextField(
                        controller: _bioController,
                        label: 'Bio',
                        maxLines: 4,
                        minLines: 3,
                        textInputAction: TextInputAction.done,
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      Text('Personality', style: AppTextStyles.labelLarge),
                      const SizedBox(height: AppSpacing.sm),
                      Wrap(
                        spacing: AppSpacing.sm,
                        runSpacing: AppSpacing.sm,
                        children: _personalityOptions
                            .map(
                              (trait) => _SelectableChip(
                                label: trait,
                                selected: _personality.contains(trait),
                                onSelected: () => setState(() {
                                  if (!_personality.add(trait)) {
                                    _personality.remove(trait);
                                  }
                                }),
                              ),
                            )
                            .toList(growable: false),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      Text('Interests', style: AppTextStyles.labelLarge),
                      const SizedBox(height: AppSpacing.sm),
                      Wrap(
                        spacing: AppSpacing.sm,
                        runSpacing: AppSpacing.sm,
                        children: _interestOptions
                            .map(
                              (interest) => _SelectableChip(
                                label: interest,
                                selected: _interests.contains(interest),
                                onSelected: () => setState(() {
                                  if (!_interests.add(interest)) {
                                    _interests.remove(interest);
                                  }
                                }),
                              ),
                            )
                            .toList(growable: false),
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
                        text: _saving
                            ? 'Saving...'
                            : widget.isEditing
                            ? 'Save changes'
                            : 'Add pet',
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

String _titleCase(String value) =>
    value.isEmpty ? value : '${value[0].toUpperCase()}${value.substring(1)}';

class _PhotoPicker extends StatelessWidget {
  const _PhotoPicker({
    required this.imageUrl,
    required this.uploading,
    required this.onTap,
  });

  final String imageUrl;
  final bool uploading;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 96,
        height: 96,
        decoration: BoxDecoration(
          color: AppColors.card,
          shape: BoxShape.circle,
          border: Border.all(color: AppColors.border),
        ),
        child: ClipOval(
          child: uploading
              ? const Center(
                  child: CircularProgressIndicator(color: AppColors.primary),
                )
              : imageUrl.isEmpty
              ? const Icon(
                  Icons.add_a_photo_outlined,
                  color: AppColors.textSecondary,
                  size: 28,
                )
              : AppImage(
                  imageUrl,
                  fit: BoxFit.cover,
                  errorBuilder: (_, _, _) => const Icon(
                    Icons.pets_rounded,
                    color: AppColors.textDisabled,
                  ),
                ),
        ),
      ),
    );
  }
}

class _SelectableChip extends StatelessWidget {
  const _SelectableChip({
    required this.label,
    required this.selected,
    required this.onSelected,
  });

  final String label;
  final bool selected;
  final VoidCallback onSelected;

  @override
  Widget build(BuildContext context) {
    return ChoiceChip(
      label: Text(label),
      selected: selected,
      onSelected: (_) => onSelected(),
      selectedColor: AppColors.primary.withValues(alpha: 0.18),
      labelStyle: AppTextStyles.labelSmall.copyWith(
        color: selected ? AppColors.primary : AppColors.textSecondary,
      ),
      side: BorderSide(color: selected ? AppColors.primary : AppColors.border),
    );
  }
}
