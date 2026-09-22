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
    this.isDiscoverable = true,
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
  /// `is_private` field, enforced across profile/feed/discovery/matching
  /// queries via internal/platform/visibility.
  final bool isPrivate;

  /// Whether the owner wants to appear in location-based discovery and
  /// matching at all — a separate, location-specific axis from
  /// [isPrivate] (see `is_discoverable`, migrations/000006_location_
  /// discovery_participation.sql). Defaults to true (participating),
  /// unlike [isPrivate]'s false default.
  final bool isDiscoverable;

  factory AuthUser.fromJson(Map<String, dynamic> json) => AuthUser(
    id: json['id']?.toString() ?? '',
    email: json['email']?.toString() ?? '',
    name: json['name']?.toString() ?? '',
    bio: json['bio']?.toString() ?? '',
    city: json['city']?.toString() ?? '',
    profilePhotoUrl: json['profile_photo_url']?.toString() ?? '',
    onboardingCompleted: json['onboarding_completed'] == true,
    isPrivate: json['is_private'] == true,
    isDiscoverable: json['is_discoverable'] != false,
  );
}
