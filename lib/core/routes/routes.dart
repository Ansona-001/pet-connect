class AppRoutes {
  AppRoutes._();

  static const String splash = '/splash';
  static const String onboarding = '/onboarding';
  static const String login = '/login';
  static const String verifyEmail = '/verify-email';
  static const String setupProfile = '/setup-profile';
  static const String home = '/home';
  static const String inbox = '/inbox';
  static const String chatPath = '/chat/:chatId';

  static String chat(String chatId) => '/chat/$chatId';
}
