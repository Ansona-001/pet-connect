/// Which third-party sign-in options the server has real credentials
/// configured for — see `GET /v1/auth/providers` in
/// server/internal/modules/auth/oauth.go.
class AuthProviders {
  const AuthProviders({
    required this.googleEnabled,
    required this.appleEnabled,
  });

  final bool googleEnabled;
  final bool appleEnabled;
}
