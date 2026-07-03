import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/theme/app_text_styles.dart';

/// Secondary application button.
///
/// Used for lower-priority actions such as Skip, Cancel, Back,
/// and Sign In.
class SecondaryButton extends StatefulWidget {
  const SecondaryButton({
    super.key,
    required this.text,
    required this.onPressed,
    this.icon,
    this.isLoading = false,
    this.enabled = true,
    this.height = 54,
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
  State<SecondaryButton> createState() => _SecondaryButtonState();
}

class _SecondaryButtonState extends State<SecondaryButton> {
  bool _pressed = false;

  bool get _disabled =>
      !widget.enabled || widget.isLoading || widget.onPressed == null;

  @override
  Widget build(BuildContext context) {
    return AnimatedScale(
      duration: const Duration(milliseconds: 120),
      curve: Curves.easeOutCubic,
      scale: _pressed ? 0.97 : 1,
      child: AnimatedOpacity(
        duration: const Duration(milliseconds: 180),
        opacity: _disabled ? 0.48 : 1,
        child: Material(
          color: Colors.transparent,
          child: Ink(
            width: widget.width,
            height: widget.height,
            decoration: BoxDecoration(
              color: AppColors.white.withValues(alpha: 0.06),
              borderRadius: AppRadius.pill,
              border: Border.all(
                color: AppColors.white.withValues(alpha: 0.12),
              ),
            ),
            child: InkWell(
              borderRadius: AppRadius.pill,
              splashColor: AppColors.white.withValues(alpha: 0.08),
              highlightColor: AppColors.transparent,
              onTap: _disabled ? null : widget.onPressed,
              onHighlightChanged: (value) {
                setState(() => _pressed = value);
              },
              child: Center(
                child: widget.isLoading
                    ? const SizedBox(
                        width: 20,
                        height: 20,
                        child: CircularProgressIndicator(
                          strokeWidth: 2.2,
                          color: AppColors.textPrimary,
                        ),
                      )
                    : Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            widget.text,
                            style: AppTextStyles.buttonMedium.copyWith(
                              color: AppColors.textPrimary,
                            ),
                          ),
                          if (widget.icon != null) ...[
                            const SizedBox(width: 8),
                            Icon(
                              widget.icon,
                              size: 18,
                              color: AppColors.textPrimary,
                            ),
                          ],
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
