import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_client.dart';
import '../../../core/network/api_exception.dart';
import '../domain/pet.dart';

final petsRepositoryProvider = Provider<PetsRepository>((ref) {
  return PetsRepository(ref.watch(apiClientProvider));
});

/// Full pet CRUD against server/internal/modules/pets/pets.go — the
/// dedicated repository for the "My Pets" management screens, distinct
/// from `SocialRepository`'s lighter `/me/pets` read used to populate the
/// app-wide active-pet switcher (see SocialController.refreshPets, which
/// this repository's mutations should be followed by so that switcher
/// stays in sync).
class PetsRepository {
  const PetsRepository(this._api);

  final ApiClient _api;

  String resolveMediaUrl(String value) => _api.config.resolveMediaUrl(value);

  Future<List<Pet>> listMine() async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>('/me/pets');
      final rows = _list(response.data?['data']);
      return rows.map(Pet.fromJson).toList();
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<Pet> create({
    required String name,
    required String petType,
    required String breed,
    required String ageLabel,
    required String gender,
    required String bio,
    required List<String> personality,
    required List<String> interests,
    required String primaryImageUrl,
    double? weightKg,
    String? birthDate,
  }) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/me/pets',
        data: {
          'name': name.trim(),
          'pet_type': petType,
          'breed': breed.trim(),
          'age_label': ageLabel.trim(),
          'gender': gender,
          'weight_kg': weightKg,
          'bio': bio.trim(),
          'personality': personality,
          'interests': interests,
          'primary_image_url': primaryImageUrl,
          'birth_date': birthDate,
        },
      );
      return Pet.fromJson(_data(response));
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<Pet> update(
    String petId, {
    required String name,
    required String petType,
    required String breed,
    required String ageLabel,
    required String gender,
    required String bio,
    required List<String> personality,
    required List<String> interests,
    required String primaryImageUrl,
    double? weightKg,
    String? birthDate,
  }) async {
    try {
      final response = await _api.dio.patch<Map<String, dynamic>>(
        '/pets/$petId',
        data: {
          'name': name.trim(),
          'pet_type': petType,
          'breed': breed.trim(),
          'age_label': ageLabel.trim(),
          'gender': gender,
          'weight_kg': weightKg,
          'clear_weight': weightKg == null,
          'bio': bio.trim(),
          'personality': personality,
          'interests': interests,
          'primary_image_url': primaryImageUrl,
          'birth_date': birthDate ?? '',
        },
      );
      return Pet.fromJson(_data(response));
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<void> delete(String petId) async {
    try {
      await _api.dio.delete<void>('/pets/$petId');
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Uploads a picked image and returns the server media path to store as
  /// `primary_image_url` — same `/media/uploads` contract
  /// `SocialRepository.createImagePost` already uses for post media.
  Future<String> uploadPhoto({
    required String fileName,
    required List<int> bytes,
  }) async {
    try {
      final extension = fileName.toLowerCase().split('.').last;
      final subtype = extension == 'png'
          ? 'png'
          : extension == 'webp'
          ? 'webp'
          : 'jpeg';
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/media/uploads',
        data: FormData.fromMap({
          'file': MultipartFile.fromBytes(
            bytes,
            filename: fileName,
            contentType: DioMediaType('image', subtype),
          ),
        }),
      );
      final path = _data(response)['path']?.toString();
      if (path == null || path.isEmpty) {
        throw const ApiException('The media upload returned no file path.');
      }
      return path;
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  static Map<String, dynamic> _data(Response<Map<String, dynamic>> response) {
    final value = response.data?['data'];
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    throw const ApiException('The server returned an invalid response.');
  }

  static List<Map<String, dynamic>> _list(Object? value) {
    if (value is! List) return const [];
    return value
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }
}
