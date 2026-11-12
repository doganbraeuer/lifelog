import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:image_picker/image_picker.dart';

import '../api/api_client.dart';

/// A file chosen by the user.
class ChosenFile {
  const ChosenFile({required this.bytes, required this.name});

  final Uint8List bytes;
  final String name;
}

/// The chosen file is larger than allowed. Thrown before the file is read
/// if its size is known in advance.
class FileTooLargeException implements Exception {
  const FileTooLargeException(this.size);

  final int size;
}

/// Camera, gallery and file picker behind one interface.
///
/// The plugins open windows of the operating system that cannot be used in
/// tests. Tests therefore pass their own implementation.
///
/// All methods return `null` if the user cancels.
abstract interface class FileChooser {
  Future<ChosenFile?> takePhoto();

  Future<ChosenFile?> pickImage();

  Future<ChosenFile?> pickPdf();

  /// Android can end the app while the camera app is open (low memory).
  /// The photo can be retrieved here after the restart.
  Future<ChosenFile?> retrieveLostFile();
}

/// Real selection with `image_picker` (camera, gallery) and `file_picker`
/// (PDF).
class PluginFileChooser implements FileChooser {
  PluginFileChooser();

  /// Longest side of a photo in pixels. 3000 px keep even small print
  /// readable and stay well below 10 MB as JPEG.
  static const maxSide = 3000.0;

  /// JPEG quality (0–100).
  static const jpegQuality = 85;

  final _imagePicker = ImagePicker();

  @override
  Future<ChosenFile?> takePhoto() => _image(ImageSource.camera);

  @override
  Future<ChosenFile?> pickImage() => _image(ImageSource.gallery);

  Future<ChosenFile?> _image(ImageSource source) async {
    final file = await _imagePicker.pickImage(
      source: source,
      maxWidth: maxSide,
      maxHeight: maxSide,
      imageQuality: jpegQuality,
      // Without full metadata iOS needs no permission for the whole photo
      // library; the selection runs through the system dialog.
      requestFullMetadata: false,
    );
    if (file == null) return null;
    return ChosenFile(bytes: await file.readAsBytes(), name: file.name);
  }

  @override
  Future<ChosenFile?> pickPdf() async {
    final file = await FilePicker.pickFile(
      type: FileType.custom,
      allowedExtensions: const ['pdf'],
    );
    if (file == null) return null;
    final size = file.lengthSync();
    if (size != null && size > maxFileSize) {
      throw FileTooLargeException(size);
    }
    return ChosenFile(bytes: await file.readAsBytes(), name: file.name);
  }

  @override
  Future<ChosenFile?> retrieveLostFile() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) return null;
    final response = await _imagePicker.retrieveLostData();
    final file = response.file;
    if (response.isEmpty || file == null) return null;
    return ChosenFile(bytes: await file.readAsBytes(), name: file.name);
  }
}
