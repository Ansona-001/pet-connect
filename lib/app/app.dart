import 'package:flutter/material.dart';
import '../core/routes/app_router.dart';
import '../core/theme/app_theme.dart';

class App extends StatelessWidget {
  const App({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      debugShowCheckedModeBanner: false,
      routerConfig: AppRouter.router,
      theme: AppTheme.darkTheme,
      themeMode: ThemeMode.dark,
    );
  }
}
