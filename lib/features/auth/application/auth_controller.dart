import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_exception.dart';
import '../data/auth_repository.dart';
import '../domain/auth_user.dart';

class AuthState {
  const AuthState({
    this.initialized = false,
    this.isLoading = false,
    this.user,
    this.error,
  });

  final bool initialized;
  final bool isLoading;
  final AuthUser? user;
  final String? error;
  bool get isAuthenticated => user != null;

  AuthState copyWith({
    bool? initialized,
    bool? isLoading,
    AuthUser? user,
    String? error,
    bool clearUser = false,
    bool clearError = false,
  }) {
    return AuthState(
      initialized: initialized ?? this.initialized,
      isLoading: isLoading ?? this.isLoading,
      user: clearUser ? null : user ?? this.user,
      error: clearError ? null : error ?? this.error,
    );
  }
}

class AuthController extends StateNotifier<AuthState> {
  AuthController(this._repository) : super(const AuthState());

  final AuthRepository _repository;

  Future<void> restore() async {
    if (state.initialized || state.isLoading) return;
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final user = await _repository.restore();
      state = AuthState(initialized: true, user: user);
    } catch (_) {
      state = const AuthState(initialized: true);
    }
  }

  Future<AuthUser?> login({required String email, required String password}) {
    return _authenticate(
      () => _repository.login(email: email, password: password),
    );
  }

  Future<AuthUser?> register({
    required String email,
    required String password,
    required String name,
  }) {
    return _authenticate(
      () => _repository.register(email: email, password: password, name: name),
    );
  }

  Future<AuthUser?> _authenticate(Future<AuthUser> Function() operation) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final user = await operation();
      state = AuthState(initialized: true, user: user);
      return user;
    } catch (error) {
      state = AuthState(
        initialized: true,
        error: ApiException.from(error).message,
      );
      return null;
    }
  }

  Future<bool> completeOnboarding({
    required String ownerName,
    required String city,
    required String petName,
    required String petType,
    required List<String> interests,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final user = await _repository.completeOnboarding(
        ownerName: ownerName,
        city: city,
        petName: petName,
        petType: petType,
        interests: interests,
      );
      state = AuthState(initialized: true, user: user);
      return true;
    } catch (error) {
      state = state.copyWith(
        initialized: true,
        isLoading: false,
        error: ApiException.from(error).message,
      );
      return false;
    }
  }

  Future<void> logout() async {
    await _repository.logout();
    state = const AuthState(initialized: true);
  }
}

final authControllerProvider = StateNotifierProvider<AuthController, AuthState>(
  (ref) {
    return AuthController(ref.watch(authRepositoryProvider));
  },
);
