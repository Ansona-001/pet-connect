import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/theme/app_shadows.dart';
import '../../../core/theme/app_text_styles.dart';

class PrimaryButton extends StatefulWidget {
  const PrimaryButton({
    super.key,
    required this.text,
    required this.onPressed,
    this.icon,
    this.isLoading = false,
    this.enabled = true,
    this.height = 58,
    this.width = double.infinity,
  });

  final String text;
  final VoidCallback? onPressed;
  final IconData? icon;
  final bool isLoading;
  final bool enabled;
  final double height;
  final double width;

  @override
  State<PrimaryButton> createState() => _PrimaryButtonState();
}

class _PrimaryButtonState extends State<PrimaryButton> {
  bool _pressed = false;

  bool get _disabled =>
      !widget.enabled || widget.isLoading || widget.onPressed == null;

  @override
  Widget build(BuildContext context) {
    return AnimatedScale(
      duration: const Duration(milliseconds: 120),
      scale: _pressed ? 0.97 : 1,
      curve: Curves.easeOut,
      child: AnimatedOpacity(
        duration: const Duration(milliseconds: 200),
        opacity: _disabled ? 0.6 : 1,
        child: Material(
          color: Colors.transparent,
          child: Ink(
            width: widget.width,
            height: widget.height,
            decoration: const BoxDecoration(
              gradient: AppColors.primaryGradient,
              borderRadius: AppRadius.pill,
              boxShadow: AppShadows.primaryGlow,
            ),
            child: InkWell(
              borderRadius: AppRadius.pill,
              splashColor: Colors.white24,
              highlightColor: Colors.transparent,
              onTap: _disabled ? null : widget.onPressed,
              onHighlightChanged: (value) {
                setState(() {
                  _pressed = value;
                });
              },
              child: Center(
                child: widget.isLoading
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(
                          strokeWidth: 2.5,
                          color: Colors.white,
                        ),
                      )
                    : Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (widget.icon != null) ...[
                            Icon(widget.icon, color: Colors.white, size: 20),
                            const SizedBox(width: 10),
                          ],
                          Text(widget.text, style: AppTextStyles.buttonLarge),
                        ],
                      ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
