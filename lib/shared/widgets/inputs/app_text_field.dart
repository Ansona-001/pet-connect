import 'package:flutter/material.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_radius.dart';
import '../../../core/theme/app_text_styles.dart';

/// Production text field used throughout the application.
///
/// Supports:
/// - Validation
/// - Password visibility
/// - Prefix/Suffix widgets
/// - Focus animation
/// - Disabled state
class AppTextField extends StatefulWidget {
  const AppTextField({
    super.key,
    this.controller,
    this.focusNode,
    this.label,
    this.hint,
    this.prefix,
    this.suffix,
    this.keyboardType,
    this.textInputAction,
    this.validator,
    this.onChanged,
    this.onSubmitted,
    this.isPassword = false,
    this.enabled = true,
    this.readOnly = false,
    this.maxLines = 1,
    this.minLines,
    this.autofocus = false,
  });

  final TextEditingController? controller;
  final FocusNode? focusNode;

  final String? label;
  final String? hint;

  final Widget? prefix;
  final Widget? suffix;

  final TextInputType? keyboardType;
  final TextInputAction? textInputAction;

  final String? Function(String?)? validator;

  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onSubmitted;

  final bool isPassword;
  final bool enabled;
  final bool readOnly;
  final bool autofocus;

  final int maxLines;
  final int? minLines;

  @override
  State<AppTextField> createState() => _AppTextFieldState();
}

class _AppTextFieldState extends State<AppTextField> {
  late bool _obscure;

  bool _focused = false;

  @override
  void initState() {
    super.initState();
    _obscure = widget.isPassword;
  }

  OutlineInputBorder _border(Color color, double width) {
    return OutlineInputBorder(
      borderRadius: AppRadius.lg,
      borderSide: BorderSide(color: color, width: width),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Focus(
      onFocusChange: (value) {
        setState(() {
          _focused = value;
        });
      },
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        curve: Curves.easeOutCubic,
        decoration: BoxDecoration(
          borderRadius: AppRadius.lg,
          boxShadow: [
            if (_focused)
              BoxShadow(
                color: AppColors.primary.withValues(alpha: .18),
                blurRadius: 18,
                spreadRadius: 1,
              ),
          ],
        ),
        child: TextFormField(
          controller: widget.controller,
          focusNode: widget.focusNode,
          validator: widget.validator,
          enabled: widget.enabled,
          readOnly: widget.readOnly,
          autofocus: widget.autofocus,
          keyboardType: widget.keyboardType,
          textInputAction: widget.textInputAction,
          obscureText: _obscure,
          maxLines: widget.isPassword ? 1 : widget.maxLines,
          minLines: widget.minLines,
          onChanged: widget.onChanged,
          onFieldSubmitted: widget.onSubmitted,
          cursorColor: AppColors.primary,
          style: AppTextStyles.bodyLarge,
          decoration: InputDecoration(
            labelText: widget.label,
            hintText: widget.hint,

            labelStyle: AppTextStyles.bodyMedium.copyWith(
              color: _focused ? AppColors.primary : AppColors.textSecondary,
            ),

            hintStyle: AppTextStyles.bodyMedium.copyWith(
              color: AppColors.textDisabled,
            ),

            filled: true,
            fillColor: AppColors.surface,

            contentPadding: const EdgeInsets.symmetric(
              horizontal: 20,
              vertical: 18,
            ),

            prefixIcon: widget.prefix == null
                ? null
                : Padding(
                    padding: const EdgeInsets.only(left: 16, right: 12),
                    child: widget.prefix,
                  ),

            prefixIconConstraints: const BoxConstraints(
              minWidth: 0,
              minHeight: 0,
            ),

            suffixIcon: widget.isPassword
                ? IconButton(
                    splashRadius: 20,
                    onPressed: () {
                      setState(() {
                        _obscure = !_obscure;
                      });
                    },
                    icon: Icon(
                      _obscure
                          ? Icons.visibility_off_rounded
                          : Icons.visibility_rounded,
                      color: AppColors.textSecondary,
                    ),
                  )
                : widget.suffix,

            border: _border(AppColors.border, 1),

            enabledBorder: _border(AppColors.border, 1),

            focusedBorder: _border(AppColors.primary, 1.5),

            errorBorder: _border(AppColors.error, 1),

            focusedErrorBorder: _border(AppColors.error, 1.5),

            disabledBorder: _border(AppColors.border.withValues(alpha: .45), 1),
          ),
        ),
      ),
    );
  }
}
