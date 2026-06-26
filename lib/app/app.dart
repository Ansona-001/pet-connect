import 'package:flutter/material.dart';

import '../core/routes/app_router.dart';
import '../core/theme/app_theme.dart';

class PetConnectApp extends StatelessWidget {
  const PetConnectApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      debugShowCheckedModeBanner: false,
      title: 'PetConnect',
      theme: AppTheme.darkTheme,
      routerConfig: AppRouter.router,
    );
  }
}
