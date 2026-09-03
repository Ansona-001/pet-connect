import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../auth/token_store.dart';
import '../config/app_config.dart';

final appConfigProvider = Provider<AppConfig>((ref) => AppConfig.resolve());
final tokenStoreProvider = Provider<TokenStore>((ref) => TokenStore());
final apiClientProvider = Provider<ApiClient>((ref) {
  return ApiClient(ref.watch(appConfigProvider), ref.watch(tokenStoreProvider));
});

class ApiClient {
  ApiClient(this.config, this.tokens)
    : dio = Dio(_options(config.apiBaseUrl)),
      _sessionDio = Dio(_options(config.apiBaseUrl)) {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          final accessToken = tokens.accessToken;
          if (accessToken != null && accessToken.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $accessToken';
          }
          handler.next(options);
        },
        onError: (error, handler) async {
          final request = error.requestOptions;
          final canRefresh =
              error.response?.statusCode == 401 &&
              request.extra['petconnectRetried'] != true &&
              !request.path.contains('/auth/refresh') &&
              !request.path.contains('/auth/login') &&
              !request.path.contains('/auth/register');
          if (!canRefresh || !await refreshSession()) {
            handler.next(error);
            return;
          }
          request.extra['petconnectRetried'] = true;
          request.headers['Authorization'] = 'Bearer ${tokens.accessToken}';
          try {
            final response = await dio.fetch<dynamic>(request);
            handler.resolve(response);
          } on DioException catch (retryError) {
            handler.next(retryError);
          }
        },
      ),
    );
  }

  final AppConfig config;
  final TokenStore tokens;
  final Dio dio;
  final Dio _sessionDio;
  Future<bool>? _refreshInFlight;

  static BaseOptions _options(String baseUrl) => BaseOptions(
    baseUrl: baseUrl,
    connectTimeout: const Duration(seconds: 10),
    receiveTimeout: const Duration(seconds: 20),
    sendTimeout: const Duration(seconds: 30),
    contentType: Headers.jsonContentType,
    responseType: ResponseType.json,
  );

  Future<void> installSession(Map<String, dynamic> session) async {
    await tokens.saveSession(
      accessToken: session['access_token'] as String,
      refreshToken: session['refresh_token'] as String,
    );
  }

  Future<bool> refreshSession() {
    final current = _refreshInFlight;
    if (current != null) return current;
    final operation = _performRefresh();
    _refreshInFlight = operation;
    operation.whenComplete(() => _refreshInFlight = null);
    return operation;
  }

  Future<bool> _performRefresh() async {
    final refreshToken = await tokens.readRefreshToken();
    if (refreshToken == null || refreshToken.isEmpty) return false;
    try {
      final response = await _sessionDio.post<Map<String, dynamic>>(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
      );
      final data = _unwrap(response.data);
      await installSession(data);
      return true;
    } catch (_) {
      await tokens.clear();
      return false;
    }
  }

  Future<void> logout() async {
    final refreshToken = await tokens.readRefreshToken();
    if (refreshToken != null) {
      try {
        await _sessionDio.post<void>(
          '/auth/logout',
          data: {'refresh_token': refreshToken},
        );
      } catch (_) {
        // Local credentials are still cleared when the server is unavailable.
      }
    }
    await tokens.clear();
  }

  static Map<String, dynamic> _unwrap(Map<String, dynamic>? body) {
    final data = body?['data'];
    if (data is Map<String, dynamic>) return data;
    if (data is Map) return Map<String, dynamic>.from(data);
    throw const FormatException('The API returned an invalid response.');
  }
}
