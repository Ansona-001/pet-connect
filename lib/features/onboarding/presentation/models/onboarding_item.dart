import 'package:flutter/material.dart';

class OnboardingItem {
  const OnboardingItem({
    required this.title,
    required this.highlight,
    required this.subtitle,
    required this.icon,
    required this.accentColor,
    required this.imageAsset,
  });

  final String title;
  final String highlight;
  final String subtitle;
  final IconData icon;
  final Color accentColor;
  final String imageAsset;
}
