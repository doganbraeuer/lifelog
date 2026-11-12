import 'package:flutter/material.dart';

import '../api/api_exception.dart';

/// Zeigt einen [ApiFehler] mit seiner Nachricht und einem Knopf zum
/// erneuten Versuchen. Die Nachricht kommt vom Backend (oder vom Client)
/// und ist laut API für die Anzeige gedacht.
class FehlerAnzeige extends StatelessWidget {
  const FehlerAnzeige({super.key, required this.fehler, this.erneut});

  final ApiException fehler;

  /// Wird beim Tippen auf „Erneut versuchen“ aufgerufen. Ohne Callback
  /// gibt es keinen Knopf.
  final VoidCallback? erneut;

  @override
  Widget build(BuildContext context) {
    final offline = fehler.code == ErrorCode.networkError;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              offline ? Icons.cloud_off_outlined : Icons.error_outline,
              size: 48,
              color: Theme.of(context).colorScheme.error,
            ),
            const SizedBox(height: 12),
            Text(fehler.message, textAlign: TextAlign.center),
            if (erneut != null) ...[
              const SizedBox(height: 16),
              FilledButton.tonal(
                onPressed: erneut,
                child: const Text('Erneut versuchen'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
