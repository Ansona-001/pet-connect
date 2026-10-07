// Diagnoses a real bug reported from a live browser session: picking
// multiple photos in PostComposerScreen only shows one tile afterward.
// This test fakes ImagePickerPlatform to return two files directly (no
// real browser file dialog involved) so it isolates whether the bug is in
// post_composer_screen.dart's own state handling or further down in the
// image_picker/browser layer.
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:image_picker_platform_interface/image_picker_platform_interface.dart';
import 'package:frontend/features/social/presentation/screens/post_composer_screen.dart';

class _FakeImagePickerPlatform extends ImagePickerPlatform {
  @override
  Future<List<XFile>> getMedia({required MediaOptions options}) async {
    // Real web picker gives each file a distinct blob: URL as `path` (see
    // image_picker_for_web's _getSelectedXFiles, which constructs each
    // XFile from web.URL.createObjectURL(file) — unique per File object).
    // Passing explicit distinct paths here mirrors that, unlike
    // XFile.fromData's default (empty path for every instance on the VM's
    // io.dart branch), which would be a test-fixture artifact, not the
    // real bug.
    return <XFile>[
      XFile.fromData(
        Uint8ListFixture.onePixelPng,
        path: 'blob:mock-1',
        name: 'first.png',
        mimeType: 'image/png',
      ),
      XFile.fromData(
        Uint8ListFixture.onePixelPng,
        path: 'blob:mock-2',
        name: 'second.png',
        mimeType: 'image/png',
      ),
    ];
  }
}

abstract final class Uint8ListFixture {
  // A minimal valid 1x1 transparent PNG.
  static final onePixelPng = Uint8List.fromList(<int>[
    0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
    0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
    0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
    0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
    0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
    0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
  ]);
}

void main() {
  testWidgets(
    'picking multiple media adds one composer tile per picked file',
    (tester) async {
      final original = ImagePickerPlatform.instance;
      ImagePickerPlatform.instance = _FakeImagePickerPlatform();
      addTearDown(() => ImagePickerPlatform.instance = original);

      await tester.pumpWidget(
        const ProviderScope(
          child: MaterialApp(
            home: PostComposerScreen(),
          ),
        ),
      );
      await tester.pump();

      await tester.tap(find.text('Add photos or videos'));
      await tester.pumpAndSettle();

      expect(
        find.byType(Image),
        findsNWidgets(2),
        reason:
            'picked 2 files from the fake platform, expected 2 composer tiles',
      );
    },
  );
}
