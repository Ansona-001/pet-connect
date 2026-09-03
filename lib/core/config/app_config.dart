import 'package:flutter/foundation.dart';

class AppConfig {
  const AppConfig({required this.apiBaseUrl});

  final String apiBaseUrl;

  String get apiOrigin {
    final uri = Uri.parse(apiBaseUrl);
    return uri
        .replace(path: '', query: null, fragment: null)
        .toString()
        .replaceAll(RegExp(r'/$'), '');
  }

  String resolveMediaUrl(String value) {
    if (value.isEmpty ||
        value.startsWith('http://') ||
        value.startsWith('https://')) {
      return value;
    }
    return '$apiOrigin${value.startsWith('/') ? value : '/$value'}';
  }

  static AppConfig resolve() {
    const configured = String.fromEnvironment('PETCONNECT_API_URL');
    if (configured.isNotEmpty) {
      return AppConfig(apiBaseUrl: configured.replaceAll(RegExp(r'/$'), ''));
    }
    final host = !kIsWeb && defaultTargetPlatform == TargetPlatform.android
        ? '10.0.2.2'
        : 'localhost';
    return AppConfig(apiBaseUrl: 'http://$host:8080/v1');
  }
}
