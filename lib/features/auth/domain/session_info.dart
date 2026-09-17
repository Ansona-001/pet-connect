/// A signed-in session as reported by `GET /v1/sessions` — see
/// server/internal/modules/auth/sessions.go. One row per device/sign-in,
/// not per access-token issuance (a session persists across many refresh
/// rotations sharing the same underlying token family).
class SessionInfo {
  const SessionInfo({
    required this.id,
    required this.deviceName,
    required this.platform,
    required this.createdAt,
    required this.lastUsedAt,
    required this.isCurrent,
  });

  final String id;
  final String deviceName;
  final String platform;
  final DateTime createdAt;
  final DateTime lastUsedAt;
  final bool isCurrent;

  factory SessionInfo.fromJson(Map<String, dynamic> json) => SessionInfo(
    id: json['id']?.toString() ?? '',
    deviceName: json['device_name']?.toString() ?? 'Unknown device',
    platform: json['platform']?.toString() ?? '',
    createdAt:
        DateTime.tryParse(json['created_at']?.toString() ?? '') ??
        DateTime.now(),
    lastUsedAt:
        DateTime.tryParse(json['last_used_at']?.toString() ?? '') ??
        DateTime.now(),
    isCurrent: json['is_current'] == true,
  );
}
