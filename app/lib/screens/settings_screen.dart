import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import '../api/api_exception.dart';
import '../reminders/routine_sync.dart';
import '../session/session_controller.dart';
import 'change_password_screen.dart';
import 'consent_screen.dart';

/// Maximum length of the display name in Unicode characters (3.13).
const maxDisplayNameLength = 50;

/// „Einstellungen“: who is logged in, display name, password, privacy
/// notice and logout. Opened with the large icon on the „Bereiche“ screen.
class SettingsScreen extends StatelessWidget {
  const SettingsScreen({super.key});

  Future<void> _editDisplayName(BuildContext context) async {
    final session = SessionScope.of(context);
    final changed = await showDialog<bool>(
      context: context,
      builder: (_) => _DisplayNameDialog(
        initial: session.user?.displayName ?? '',
        save: session.updateDisplayName,
      ),
    );
    if (changed == true && context.mounted) {
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Anzeigename geändert.')));
    }
  }

  Future<void> _logout(BuildContext context) async {
    final session = SessionScope.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Abmelden?'),
        content: const Text(
          'Zum nächsten Öffnen brauchst du wieder Benutzername und Passwort.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Abbrechen'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Abmelden'),
          ),
        ],
      ),
    );
    // The session closes every page and shows the login.
    if (confirmed == true) await session.logout();
  }

  @override
  Widget build(BuildContext context) {
    final session = SessionScope.of(context);
    final user = session.user;
    final textTheme = Theme.of(context).textTheme;
    return Scaffold(
      appBar: AppBar(title: const Text('Einstellungen')),
      body: ListView(
        children: [
          if (user != null)
            ListTile(
              leading: const Icon(Icons.account_circle_outlined, size: 40),
              title: Text(
                'Angemeldet als ${user.displayName} (${user.username})',
                style: textTheme.titleMedium,
              ),
            ),
          const Divider(),
          ListTile(
            leading: const Icon(Icons.badge_outlined),
            title: const Text('Anzeigename ändern'),
            onTap: () => _editDisplayName(context),
          ),
          ListTile(
            leading: const Icon(Icons.password),
            title: const Text('Passwort ändern'),
            onTap: () => Navigator.push(
              context,
              MaterialPageRoute<void>(
                builder: (_) => const ChangePasswordScreen(mandatory: false),
              ),
            ),
          ),
          ListTile(
            leading: const Icon(Icons.privacy_tip_outlined),
            title: const Text('Hinweis zum Datenschutz'),
            onTap: () => Navigator.push(
              context,
              MaterialPageRoute<void>(
                builder: (_) => const ConsentScreen(readOnly: true),
              ),
            ),
          ),
          // Debug builds only: a reminder at once instead of waiting for
          // the approximate one (for trying it out and the demonstration).
          if (kDebugMode && RoutineSyncScope.maybeOf(context) != null)
            ListTile(
              leading: const Icon(Icons.notifications_active_outlined),
              title: const Text('Nächste Erinnerung jetzt zeigen'),
              subtitle: const Text('Nur in der Entwicklerversion'),
              onTap: () async {
                final shown = await RoutineSyncScope.maybeOf(context)!
                    .showNextNow();
                if (!shown && context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('Keine Erinnerung geplant.')),
                  );
                }
              },
            ),
          const Divider(),
          ListTile(
            leading: const Icon(Icons.logout),
            title: const Text('Abmelden'),
            onTap: () => _logout(context),
          ),
        ],
      ),
    );
  }
}

/// Checks a display name as the backend does (3.13): after trimming 1 to 50
/// characters, no control characters. Returns the German message or `null`.
String? checkDisplayName(String name) {
  final trimmed = name.trim();
  final length = trimmed.runes.length;
  if (length < 1 ||
      length > maxDisplayNameLength ||
      RegExp(r'\p{Cc}', unicode: true).hasMatch(trimmed)) {
    return 'Der Anzeigename muss 1 bis $maxDisplayNameLength Zeichen lang '
        'sein.';
  }
  return null;
}

class _DisplayNameDialog extends StatefulWidget {
  const _DisplayNameDialog({required this.initial, required this.save});

  final String initial;
  final Future<void> Function(String) save;

  @override
  State<_DisplayNameDialog> createState() => _DisplayNameDialogState();
}

class _DisplayNameDialogState extends State<_DisplayNameDialog> {
  late final _field = TextEditingController(text: widget.initial);
  var _busy = false;
  String? _error;

  @override
  void dispose() {
    _field.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final problem = checkDisplayName(_field.text);
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await widget.save(_field.text.trim());
      if (mounted) Navigator.pop(context, true);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Anzeigename ändern'),
      content: TextField(
        controller: _field,
        autofocus: true,
        enabled: !_busy,
        textCapitalization: TextCapitalization.words,
        textInputAction: TextInputAction.done,
        onSubmitted: (_) => _submit(),
        decoration: InputDecoration(
          labelText: 'Anzeigename',
          helperText: '1 bis $maxDisplayNameLength Zeichen',
          errorText: _error,
          errorMaxLines: 2,
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.pop(context, false),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: _busy ? null : _submit,
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}
