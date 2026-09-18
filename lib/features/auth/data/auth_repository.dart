import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_client.dart';
import '../../../core/network/api_exception.dart';
import '../domain/auth_providers.dart';
import '../domain/auth_user.dart';
import '../domain/session_info.dart';

final authRepositoryProvider = Provider<AuthRepository>((ref) {
  return AuthRepository(ref.watch(apiClientProvider));
});

/// Backs the "Continue with Google"/"Continue with Apple" buttons'
/// visibility on [LoginScreen] — fetched once per app session since
/// provider configuration doesn't change while the app is running.
final authProvidersProvider = FutureProvider<AuthProviders>((ref) {
  return ref.watch(authRepositoryProvider).authProviders();
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
      // Checked before attempting the refresh: a stored token that gets
      // rejected is a session that died ('session_expired'), which is a
      // different situation from never having signed in at all
      // ('no_session') — AuthController.restore() shows a "please sign in
      // again" message only for the former.
      final hadStoredSession = await _api.hasStoredRefreshToken();
      if (!await _api.refreshSession()) {
        throw ApiException(
          'Your session has expired.',
          code: hadStoredSession ? 'session_expired' : 'no_session',
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

  /// Begins Google Sign-In (server/internal/modules/auth/oauth.go's
  /// GET /auth/oauth/google/start) and returns the URL to open in the
  /// system browser. Only meaningful when [authProviders] reports
  /// `googleEnabled` — otherwise the server rejects this with a clear
  /// "not configured" error.
  Future<String> startGoogleSignIn() => _startOAuth('/auth/oauth/google/start');

  /// Begins Sign in with Apple (server/internal/modules/auth/
  /// apple_oauth.go's GET /auth/oauth/apple/start) — same contract as
  /// [startGoogleSignIn], gated on `appleEnabled`.
  Future<String> startAppleSignIn() => _startOAuth('/auth/oauth/apple/start');

  Future<String> _startOAuth(String path) async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>(path);
      final data = _data(response);
      final url = data['authorization_url']?.toString() ?? '';
      if (url.isEmpty) {
        throw const ApiException('The server did not return a sign-in URL.');
      }
      return url;
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Which third-party sign-in buttons the server has real credentials
  /// configured for (server/internal/modules/auth/oauth.go's
  /// GET /auth/providers) — the single source of truth for button
  /// visibility, so the client never needs its own copy of "is Google
  /// configured" to keep in sync with the server's. Defaults every
  /// provider to disabled on any error (network failure, server down):
  /// hiding a button that might have worked is a much smaller problem
  /// than showing one that's guaranteed to fail.
  Future<AuthProviders> authProviders() async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>(
        '/auth/providers',
      );
      final data = _data(response);
      return AuthProviders(
        googleEnabled: data['google_enabled'] == true,
        appleEnabled: data['apple_enabled'] == true,
      );
    } catch (_) {
      return const AuthProviders(googleEnabled: false, appleEnabled: false);
    }
  }

  Future<void> logout() => _api.logout();

  /// Ends every session for this account, not just the current device —
  /// see server/internal/modules/auth/sessions.go's logout-all. Never
  /// throws (mirrors [logout]'s best-effort contract): local credentials
  /// are cleared even if the request itself fails.
  Future<void> logoutAll() => _api.logoutAll();

  /// Lists this account's currently active sessions/devices (brief
  /// Milestone 1: "session/device list"). Ordered by most recently used.
  Future<List<SessionInfo>> listSessions() async {
    try {
      final response = await _api.dio.get<Map<String, dynamic>>('/sessions');
      final data = response.data?['data'];
      if (data is! List) return const [];
      return data
          .whereType<Map>()
          .map(
            (entry) => SessionInfo.fromJson(Map<String, dynamic>.from(entry)),
          )
          .toList();
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Ends one session by id — the server enforces that it belongs to the
  /// caller (BOLA-safe), so this can't be used to sign another account out.
  Future<void> revokeSession(String sessionId) async {
    try {
      await _api.dio.delete<void>('/sessions/$sessionId');
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Consumes a one-time verification token — see
  /// server/internal/modules/auth/verification.go. Throws [ApiException]
  /// with a generic message for a token that's missing, already used, or
  /// expired; the server deliberately doesn't distinguish which.
  Future<void> verifyEmail(String token) async {
    try {
      await _api.dio.post<void>('/auth/verify-email', data: {'token': token});
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Requests a fresh verification token by email. Always succeeds from
  /// the caller's point of view — the backend returns the same generic
  /// message whether or not the account exists, to avoid revealing account
  /// existence (brief §13).
  Future<String> resendVerification(String email) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/auth/resend-verification',
        data: {'email': email},
      );
      final data = _data(response);
      return data['message']?.toString() ?? '';
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Requests a password reset link by email. Same enumeration-safe
  /// contract as [resendVerification] — always returns a generic message
  /// regardless of whether the account exists (brief §13).
  Future<String> forgotPassword(String email) async {
    try {
      final response = await _api.dio.post<Map<String, dynamic>>(
        '/auth/forgot-password',
        data: {'email': email},
      );
      final data = _data(response);
      return data['message']?.toString() ?? '';
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  /// Consumes a one-time reset token and sets a new password — see
  /// server/internal/modules/auth/password_reset.go. Success revokes every
  /// existing session server-side, so the caller lands back on the login
  /// screen rather than being kept signed in.
  Future<void> resetPassword({
    required String token,
    required String password,
  }) async {
    try {
      await _api.dio.post<void>(
        '/auth/reset-password',
        data: {'token': token, 'password': password},
      );
    } catch (error) {
      throw ApiException.from(error);
    }
  }

  static Map<String, dynamic> _data(Response<Map<String, dynamic>> response) {
    return _map(response.data?['data']);
  }

  static Map<String, dynamic> _map(Object? value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    throw const ApiException('The server returned an invalid response.');
  }
}
