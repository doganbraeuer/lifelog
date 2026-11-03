import 'package:flutter/material.dart';

/// Symbol für einen Dokumenttyp.
IconData typIcon(String typ) => switch (typ) {
  'laborbefund' => Icons.science_outlined,
  'arztbrief' => Icons.description_outlined,
  'rezept' => Icons.medication_outlined,
  _ => Icons.insert_drive_file_outlined,
};
