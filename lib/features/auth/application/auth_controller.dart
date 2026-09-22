import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_client.dart';
import '../../../core/network/api_exception.dart';
import '../data/auth_repository.dart';
import '../domain/auth_user.dart';

class AuthState {
  const AuthState({
    this.initialized = false,
    this.isLoading = false,
    this.user,
    this.error,
    this.sessionExpired = false,
  });

  final bool initialized;
  final bool isLoading;
  final AuthUser? user;
  final String? error;

  /// True when the last time this app held an authenticated session, it
  /// ended because a 401 could not be resolved by refreshing — not because
  /// the user was simply never signed in. The router uses this to send an
  /// expired session to `/login` instead of the `/onboarding` pitch screens
  /// a first-time visitor would see.
  final bool sessionExpired;
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
    } catch (error) {
      // Distinguishes "this device held a session that has since died" from
      // a plain first-time visitor, so a returning user sees /login with an
      // explanatory message instead of the /onboarding pitch screens again
      // — see AuthRepository.restore(), which throws this exact code when
      // the stored refresh token is rejected.
      final expired = error is ApiException && error.code == 'session_expired';
      state = AuthState(
        initialized: true,
        sessionExpired: expired,
        error: expired
            ? 'Your session has expired. Please sign in again.'
            : null,
      );
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

  /// Updates the owner's own profile and refreshes [AuthState.user] on
  /// success so every screen reading it (profile, feed captions, etc.)
  /// sees the change immediately without a separate refetch.
  Future<bool> updateProfile({
    required String name,
    required String bio,
    required String city,
    required bool isPrivate,
    required bool isDiscoverable,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final user = await _repository.updateProfile(
        name: name,
        bio: bio,
        city: city,
        isPrivate: isPrivate,
        isDiscoverable: isDiscoverable,
      );
      state = AuthState(initialized: true, user: user);
      return true;
    } catch (error) {
      state = state.copyWith(
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

  Future<void> logoutAll() async {
    await _repository.logoutAll();
    state = const AuthState(initialized: true);
  }

  /// Reacts to [sessionExpiredEventsProvider]. Guarded on
  /// [AuthState.isAuthenticated] so this can never race with — or override
  /// the result of — a cold-start [restore] that never had a user to begin
  /// with: `isAuthenticated` is false for the entire duration of a restore
  /// attempt, so this is a no-op unless a session that was genuinely in use
  /// just died.
  void handleSessionExpired() {
    if (!state.isAuthenticated) return;
    state = const AuthState(
      initialized: true,
      sessionExpired: true,
      error: 'Your session has expired. Please sign in again.',
    );
  }
}

final authControllerProvider = StateNotifierProvider<AuthController, AuthState>(
  (ref) {
    final controller = AuthController(ref.watch(authRepositoryProvider));
    final subscription = ref
        .watch(sessionExpiredEventsProvider)
        .stream
        .listen((_) => controller.handleSessionExpired());
    ref.onDispose(subscription.cancel);
    return controller;
  },
);
