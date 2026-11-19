import 'dart:io';

import 'package:flutter/painting.dart';
import 'package:path_provider/path_provider.dart';

/// What the app leaves on the phone outside the secure storage: temporary
/// files and decoded images in memory. On logout it is removed, so that the
/// next user of the phone finds nothing of the previous one.
abstract interface class LocalData {
  Future<void> clear();
}

/// Deletes the content of the app's temporary folder (recordings of voice
/// notes, copies made by image_picker and file_picker) and empties the image
/// cache (photos and rendered PDF pages of opened originals).
class PluginLocalData implements LocalData {
  PluginLocalData();

  @override
  Future<void> clear() async {
    PaintingBinding.instance.imageCache
      ..clear()
      ..clearLiveImages();
    try {
      final temp = await getTemporaryDirectory();
      await deleteContents(temp);
    } on Exception {
      // No temporary folder (e.g. on an unsupported platform): nothing to do.
    }
  }

  /// Deletes everything inside [directory], but not the folder itself.
  /// A file in use is skipped; the rest is still deleted.
  static Future<void> deleteContents(Directory directory) async {
    if (!await directory.exists()) return;
    await for (final entity in directory.list(followLinks: false)) {
      try {
        await entity.delete(recursive: true);
      } on FileSystemException {
        // Locked or already gone.
      }
    }
  }
}
