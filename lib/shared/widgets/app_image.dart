import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

ImageProvider<Object>? appImageProvider(String source, {int? cacheWidth}) {
  if (source.isEmpty) return null;
  final provider = source.startsWith('http://') || source.startsWith('https://')
      ? NetworkImage(source) as ImageProvider<Object>
      : AssetImage(source) as ImageProvider<Object>;
  if (cacheWidth == null) return provider;
  return ResizeImage(provider, width: cacheWidth);
}

class AppImage extends StatelessWidget {
  const AppImage(
    this.source, {
    super.key,
    this.width,
    this.height,
    this.fit,
    this.alignment = Alignment.center,
    this.cacheWidth,
    this.filterQuality = FilterQuality.low,
    this.errorBuilder,
  });

  final String source;
  final double? width;
  final double? height;
  final BoxFit? fit;
  final AlignmentGeometry alignment;
  final int? cacheWidth;
  final FilterQuality filterQuality;
  final ImageErrorWidgetBuilder? errorBuilder;

  @override
  Widget build(BuildContext context) {
    final fallback =
        errorBuilder ??
        (context, error, stackTrace) => const ColoredBox(
          color: AppColors.surface,
          child: Center(
            child: Icon(Icons.pets_rounded, color: AppColors.textSecondary),
          ),
        );
    if (source.startsWith('http://') || source.startsWith('https://')) {
      return Image.network(
        source,
        width: width,
        height: height,
        fit: fit,
        alignment: alignment,
        cacheWidth: cacheWidth,
        filterQuality: filterQuality,
        errorBuilder: fallback,
      );
    }
    if (source.isEmpty) {
      return SizedBox(
        width: width,
        height: height,
        child: fallback(context, '', null),
      );
    }
    return Image.asset(
      source,
      width: width,
      height: height,
      fit: fit,
      alignment: alignment,
      cacheWidth: cacheWidth,
      filterQuality: filterQuality,
      errorBuilder: fallback,
    );
  }
}
