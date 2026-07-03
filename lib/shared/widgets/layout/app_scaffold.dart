import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';

/// Base scaffold used across the application.
///
/// Provides:
/// - Consistent background
/// - Optional SafeArea
/// - Optional scrolling
/// - Keyboard handling
/// - FloatingActionButton support
/// - BottomNavigation support
class AppScaffold extends StatelessWidget {
  const AppScaffold({
    super.key,
    required this.child,
    this.appBar,
    this.backgroundColor,
    this.safeArea = true,
    this.scrollable = false,
    this.padding = EdgeInsets.zero,
    this.bottomNavigationBar,
    this.floatingActionButton,
    this.floatingActionButtonLocation,
    this.resizeToAvoidBottomInset = true,
  });

  final Widget child;

  final PreferredSizeWidget? appBar;

  final Color? backgroundColor;

  final bool safeArea;

  final bool scrollable;

  final EdgeInsetsGeometry padding;

  final Widget? bottomNavigationBar;

  final Widget? floatingActionButton;

  final FloatingActionButtonLocation? floatingActionButtonLocation;

  final bool resizeToAvoidBottomInset;

  @override
  Widget build(BuildContext context) {
    Widget body = Padding(padding: padding, child: child);

    if (scrollable) {
      body = SingleChildScrollView(
        physics: const BouncingScrollPhysics(),
        child: body,
      );
    }

    if (safeArea) {
      body = SafeArea(child: body);
    }

    return Scaffold(
      backgroundColor: backgroundColor ?? AppColors.background,
      resizeToAvoidBottomInset: resizeToAvoidBottomInset,
      appBar: appBar,
      body: body,
      bottomNavigationBar: bottomNavigationBar,
      floatingActionButton: floatingActionButton,
      floatingActionButtonLocation: floatingActionButtonLocation,
    );
  }
}
