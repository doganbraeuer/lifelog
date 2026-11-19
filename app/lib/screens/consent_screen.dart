import 'package:flutter/material.dart';

import '../api/api_exception.dart';
import '../session/consent_text.dart';
import '../session/session_controller.dart';

/// The consent notice (version [consentVersion]).
///
/// Shown by the session before the app until the user agrees
/// (`PUT /me/consent`). „Nicht jetzt“ logs out. With [readOnly] the same
/// text opens from the settings as „Hinweis zum Datenschutz“, without
/// buttons.
class ConsentScreen extends StatefulWidget {
  const ConsentScreen({super.key, this.readOnly = false});

  final bool readOnly;

  @override
  State<ConsentScreen> createState() => _ConsentScreenState();
}

class _ConsentScreenState extends State<ConsentScreen> {
  var _busy = false;

  Future<void> _agree() async {
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await SessionScope.of(context).recordConsent();
    } on ApiException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _notNow() async {
    setState(() => _busy = true);
    await SessionScope.of(context).logout();
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.readOnly ? 'Hinweis zum Datenschutz' : consentTitle),
        automaticallyImplyLeading: widget.readOnly,
      ),
      body: SafeArea(
        bottom: widget.readOnly,
        child: Scrollbar(
          child: ListView(
            padding: const EdgeInsets.fromLTRB(24, 16, 24, 24),
            children: [
              Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 640),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      if (widget.readOnly) ...[
                        Text(consentTitle, style: theme.textTheme.titleLarge),
                        const SizedBox(height: 16),
                      ],
                      for (final paragraph in consentParagraphs) ...[
                        _Paragraph(text: paragraph),
                        const SizedBox(height: 16),
                      ],
                      Text(
                        'Version $consentVersion',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: theme.colorScheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
      bottomNavigationBar: widget.readOnly
          ? null
          : SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
                child: Row(
                  children: [
                    Expanded(
                      child: OutlinedButton(
                        onPressed: _busy ? null : _notNow,
                        style: OutlinedButton.styleFrom(
                          minimumSize: const Size.fromHeight(48),
                        ),
                        child: const Text('Nicht jetzt'),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: FilledButton(
                        onPressed: _busy ? null : _agree,
                        style: FilledButton.styleFrom(
                          minimumSize: const Size.fromHeight(48),
                        ),
                        child: const Text('Zustimmen'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
    );
  }
}

/// A paragraph; a short lead-in up to the first colon („Widerruf:“) is
/// bold, so the reader finds the topics quickly.
class _Paragraph extends StatelessWidget {
  const _Paragraph({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    final style = Theme.of(context).textTheme.bodyLarge?.copyWith(height: 1.5);
    final colon = text.indexOf(': ');
    if (colon < 0 || colon > 40) return Text(text, style: style);
    return Text.rich(
      TextSpan(
        style: style,
        children: [
          TextSpan(
            text: text.substring(0, colon + 1),
            style: const TextStyle(fontWeight: FontWeight.w600),
          ),
          TextSpan(text: text.substring(colon + 1)),
        ],
      ),
    );
  }
}
