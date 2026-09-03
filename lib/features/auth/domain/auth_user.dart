class AuthUser {
  const AuthUser({
    required this.id,
    required this.email,
    required this.name,
    required this.bio,
    required this.city,
    required this.profilePhotoUrl,
    required this.onboardingCompleted,
  });

  final String id;
  final String email;
  final String name;
  final String bio;
  final String city;
  final String profilePhotoUrl;
  final bool onboardingCompleted;

  factory AuthUser.fromJson(Map<String, dynamic> json) => AuthUser(
    id: json['id']?.toString() ?? '',
    email: json['email']?.toString() ?? '',
    name: json['name']?.toString() ?? '',
    bio: json['bio']?.toString() ?? '',
    city: json['city']?.toString() ?? '',
    profilePhotoUrl: json['profile_photo_url']?.toString() ?? '',
    onboardingCompleted: json['onboarding_completed'] == true,
  );
}
