import 'package:flutter/material.dart';

class OnboardingItem {
  const OnboardingItem({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.gradientColors,
  });

  final String title;
  final String subtitle;
  final IconData icon;
  final List<Color> gradientColors;
}
