import 'package:dio/dio.dart';

class ApiException implements Exception {
  const ApiException(this.message, {this.code, this.statusCode});

  final String message;
  final String? code;
  final int? statusCode;

  factory ApiException.from(Object error) {
    if (error is ApiException) return error;
    if (error is DioException) {
      final body = error.response?.data;
      if (body is Map) {
        final problem = body['error'];
        if (problem is Map) {
          return ApiException(
            problem['message']?.toString() ??
                'PetConnect could not complete the request.',
            code: problem['code']?.toString(),
            statusCode: error.response?.statusCode,
          );
        }
      }
      if (error.type == DioExceptionType.connectionError ||
          error.type == DioExceptionType.connectionTimeout) {
        return const ApiException(
          'Cannot reach PetConnect. Check that the local API is running.',
          code: 'connection_failed',
        );
      }
      return ApiException(
        error.message ?? 'PetConnect could not complete the request.',
        statusCode: error.response?.statusCode,
      );
    }
    return ApiException(error.toString());
  }

  @override
  String toString() => message;
}
