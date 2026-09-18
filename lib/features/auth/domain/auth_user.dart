class AuthUser {
  const AuthUser({
    required this.id,
    required this.email,
    required this.name,
    required this.bio,
    required this.city,
    required this.profilePhotoUrl,
    required this.onboardingCompleted,
    this.isPrivate = false,
  });

  final String id;
  final String email;
  final String name;
  final String bio;
  final String city;
  final String profilePhotoUrl;
  final bool onboardingCompleted;

  /// Whether the owner has restricted their profile/pets to people they've
  /// connected with — see server/internal/modules/account/account.go's
  /// `is_private` field. Enforcement of what "restricted" means across
  /// each read path is separate, later work; this flag today only reflects
  /// the owner's stated preference.
  final bool isPrivate;

  factory AuthUser.fromJson(Map<String, dynamic> json) => AuthUser(
    id: json['id']?.toString() ?? '',
    email: json['email']?.toString() ?? '',
    name: json['name']?.toString() ?? '',
    bio: json['bio']?.toString() ?? '',
    city: json['city']?.toString() ?? '',
    profilePhotoUrl: json['profile_photo_url']?.toString() ?? '',
    onboardingCompleted: json['onboarding_completed'] == true,
    isPrivate: json['is_private'] == true,
  );
}
