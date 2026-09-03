import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_client.dart';
import '../../../core/network/api_exception.dart';
import '../domain/auth_user.dart';

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(ref.watch(apiClientProvider));
});

class AuthRepository {
  const AuthRepository(this._api);

  final ApiClient _api;

  Future<AuthUser> login({required String email, required String password}) {
    return _createSession('/auth/login', {
      'email': email,
      'password': password,
    });
  }

  Future<AuthUser> register({
    required String email,
    required String password,
    required String name,
  }) {
    return _createSession('/auth/register', {
      'email': email,
      'password': password,
      'name': name,
    });
  }

  Future<AuthUser> _createSession(
    String path,
    Map<String, dynamic> body,
  ) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        path,
        data: body,
      );
      final session = _data(response);
      await _api.installSession(session);
      return AuthUser.fromJson(_map(session['user']));
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<AuthUser> restore() async {
    try {
      if (!await _api.refreshSession()) {
        throw const ApiException(
          'Your session has expired.',
          code: 'session_expired',
        );
      }
      return me();
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<AuthUser> me() async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>('/me');
      return AuthUser.fromJson(_data(response));
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<AuthUser> completeOnboarding({
    required String ownerName,
    required String city,
    required String petName,
    required String petType,
    required List<String> interests,
  }) async {
    try {
      final petsResponse = await _api.dio.get<Map<String, dynamic>>('/me/pets');
      final pets = petsResponse.data?['data'];
      if (pets is! List || pets.isEmpty) {
        await _api.dio.post<Map<String, dynamic>>(
          '/me/pets',
          data: {
            'name': petName.trim(),
            'pet_type': petType.toLowerCase(),
            'breed': '',
            'age_label': '',
            'gender': 'unknown',
            'bio': '',
            'personality': <String>[],
            'interests': interests,
            'primary_image_url': '',
          },
        );
      }
      final response = await _api.dio.patch<Map<String, dynamic>>(
        '/me',
        data: {
          'name': ownerName.trim(),
          'city': city.trim(),
          'complete_onboarding': true,
        },
      );
      return AuthUser.fromJson(_data(response));
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  Future<void> logout() => _api.logout();

  static Map<String, dynamic> _data(Response<Map<String, dynamic>> response) {
    return _map(response.data?['data']);
  }

  static Map<String, dynamic> _map(Object? value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    throw const ApiException('The server returned an invalid response.');
  }
}
