import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/theme/app_theme.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:frontend/features/auth/domain/auth_providers.dart';
import 'package:frontend/features/auth/presentation/screens/login_screen.dart';

void main() {
  Future<void> pumpLogin(WidgetTester tester, AuthProviders providers) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authProvidersProvider.overrideWith((ref) async => providers),
        ],
        child: MaterialApp(
          theme: AppTheme.darkTheme,
          home: const LoginScreen(),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));
  }

  testWidgets('Google button is hidden when the provider is not configured', (
    tester,
  ) async {
    await pumpLogin(
      tester,
      const AuthProviders(googleEnabled: false, appleEnabled: false),
    );

    expect(find.text('Continue with Google'), findsNothing);
    expect(
      find.textContaining('become available after provider credentials'),
      findsOneWidget,
    );
  });

  testWidgets('Google button appears once the provider is configured', (
    tester,
  ) async {
    await pumpLogin(
      tester,
      const AuthProviders(googleEnabled: true, appleEnabled: false),
    );

    expect(find.text('Continue with Google'), findsOneWidget);
    expect(
      find.textContaining('become available after provider credentials'),
      findsNothing,
    );
  });
}
