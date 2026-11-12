import 'package:flutter/material.dart';

import '../api/api_exception.dart';

/// Shows an [ApiException] with its message and a button to try again.
/// The message comes from the backend (or from the client) and according
/// to the API is meant for display.
class ErrorView extends StatelessWidget {
  const ErrorView({super.key, required this.error, this.onRetry});

  final ApiException error;

  /// Called when „Erneut versuchen“ (try again) is tapped. Without a
  /// callback there is no button.
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    final offline = error.code == ErrorCode.networkError;
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
            Text(error.message, textAlign: TextAlign.center),
            if (onRetry != null) ...[
              const SizedBox(height: 16),
              FilledButton.tonal(
                onPressed: onRetry,
                child: const Text('Erneut versuchen'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
