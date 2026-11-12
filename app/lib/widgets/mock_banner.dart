import 'package:flutter/material.dart';

/// Small badge in the app bar while the app runs with mock data.
/// That way nobody mistakes the sample data for real results.
class MockBanner extends StatelessWidget {
  const MockBanner({super.key, required this.visible});

  final bool visible;

  @override
  Widget build(BuildContext context) {
    if (!visible) return const SizedBox.shrink();
    final colors = Theme.of(context).colorScheme;
    return Tooltip(
      message: 'Die App zeigt Beispieldaten, kein Backend verbunden.',
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 8),
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        decoration: BoxDecoration(
          color: colors.tertiaryContainer,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text(
          'Mock',
          style: TextStyle(color: colors.onTertiaryContainer, fontSize: 12),
        ),
      ),
    );
  }
}
