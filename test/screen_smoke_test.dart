import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/theme/app_theme.dart';
import 'package:frontend/features/social/application/social_controller.dart';
import 'package:frontend/features/social/presentation/screens/chat_screen.dart';
import 'package:frontend/features/social/presentation/screens/match_screen.dart';
import 'package:frontend/features/social/presentation/screens/social_shell_screen.dart';

void main() {
  Future<void> pumpPhoneSurface(WidgetTester tester, Widget screen) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          socialControllerProvider.overrideWith((ref) => SocialController()),
        ],
        child: MaterialApp(theme: AppTheme.darkTheme, home: screen),
      ),
    );
    await tester.pump(const Duration(milliseconds: 300));
  }

  testWidgets('social shell renders on a standard phone surface', (
    tester,
  ) async {
    await pumpPhoneSurface(tester, const SocialShellScreen());

    expect(find.byType(NavigationBar), findsOneWidget);
    for (final destination in const ['Discover', 'Match', 'Reels', 'Profile']) {
      await tester.tap(
        find.descendant(
          of: find.byType(NavigationBar),
          matching: find.text(destination),
        ),
      );
      await tester.pump(const Duration(milliseconds: 300));
      expect(tester.takeException(), isNull, reason: destination);
    }
  });

  testWidgets('match renders on a standard phone surface', (tester) async {
    await pumpPhoneSurface(tester, const MatchScreen());
    expect(tester.takeException(), isNull);
  });

  for (final chat in const {
    'chat-coco': 'Coco',
    'chat-milo': 'Milo',
    'chat-simba': 'Simba',
  }.entries) {
    testWidgets('${chat.value} chat renders on a standard phone surface', (
      tester,
    ) async {
      await pumpPhoneSurface(tester, ChatScreen(chatId: chat.key));

      expect(find.text(chat.value), findsWidgets);
      expect(find.byType(TextField), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }
}
