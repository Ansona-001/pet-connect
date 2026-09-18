/// A pet owned by the current user, with the full field set the backend's
/// CRUD endpoints (server/internal/modules/pets/pets.go) expose — used for
/// add/edit/delete management, unlike the lighter `PetProfile` view model
/// (lib/features/social/domain/social_models.dart) used for feed/discovery
/// display, which deliberately omits fields management doesn't need there.
class Pet {
  const Pet({
    required this.id,
    required this.ownerId,
    required this.name,
    required this.petType,
    required this.breed,
    required this.birthDate,
    required this.ageLabel,
    required this.gender,
    required this.weightKg,
    required this.bio,
    required this.personality,
    required this.interests,
    required this.primaryImageUrl,
    required this.isVerified,
    required this.status,
  });

  final String id;
  final String ownerId;
  final String name;
  final String petType;
  final String breed;
  final String? birthDate;
  final String ageLabel;
  final String gender;
  final double? weightKg;
  final String bio;
  final List<String> personality;
  final List<String> interests;
  final String primaryImageUrl;
  final bool isVerified;
  final String status;

  factory Pet.fromJson(Map<String, dynamic> json) => Pet(
    id: json['id']?.toString() ?? '',
    ownerId: json['owner_id']?.toString() ?? '',
    name: json['name']?.toString() ?? '',
    petType: json['pet_type']?.toString() ?? '',
    breed: json['breed']?.toString() ?? '',
    birthDate: json['birth_date']?.toString(),
    ageLabel: json['age_label']?.toString() ?? '',
    gender: json['gender']?.toString() ?? 'unknown',
    weightKg: json['weight_kg'] == null
        ? null
        : double.tryParse(json['weight_kg'].toString()),
    bio: json['bio']?.toString() ?? '',
    personality: _strings(json['personality']),
    interests: _strings(json['interests']),
    primaryImageUrl: json['primary_image_url']?.toString() ?? '',
    isVerified: json['is_verified'] == true,
    status: json['status']?.toString() ?? 'active',
  );

  static List<String> _strings(Object? value) {
    if (value is! List) return const [];
    return value.map((item) => item.toString()).toList(growable: false);
  }
}

const petTypes = ['dog', 'cat', 'bird', 'rabbit', 'exotic', 'other'];
const petGenders = ['male', 'female', 'unknown'];
