import 'package:flutter/material.dart';

import '../api/api_exception.dart';
import '../session/password_rules.dart';
import '../session/session_controller.dart';
import '../widgets/password_field.dart';
import 'login_screen.dart' show tooManyAttemptsText;

/// Message after a successful password change (docs/API.md 3.14: all other
/// tokens of the account end).
const passwordChangedMessage =
    'Passwort geändert. Andere Geräte wurden abgemeldet.';

/// „Eigenes Passwort festlegen“ after the first login with a start password
/// ([mandatory]), or „Passwort ändern“ from the settings.
///
/// The rules are checked in the app first ([checkNewPassword]); the backend
/// checks again. The passwords live only in the text fields.
class ChangePasswordScreen extends StatefulWidget {
  const ChangePasswordScreen({super.key, required this.mandatory});

  /// `true`: shown by the session instead of the app, with „Abmelden“ as the
  /// only way out. `false`: opened from the settings, closes on success.
  final bool mandatory;

  @override
  State<ChangePasswordScreen> createState() => _ChangePasswordScreenState();
}

class _ChangePasswordScreenState extends State<ChangePasswordScreen> {
  final _current = TextEditingController();
  final _next = TextEditingController();
  final _repeat = TextEditingController();
  var _busy = false;
  String? _error;

  @override
  void dispose() {
    _current.dispose();
    _next.dispose();
    _repeat.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;
    final problem = checkNewPassword(
      current: _current.text,
      next: _next.text,
      repeat: _repeat.text,
    );
    if (problem != null) {
      setState(() => _error = problem);
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() {
      _busy = true;
      _error = null;
    });
    // Taken before the await: afterwards this screen may be gone (mandatory
    // case) while the app-wide messenger stays.
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);
    try {
      await SessionScope.of(context).changePassword(_current.text, _next.text);
      messenger.showSnackBar(
        const SnackBar(content: Text(passwordChangedMessage)),
      );
      if (!widget.mandatory && mounted) navigator.pop();
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.code == ErrorCode.tooManyAttempts
            ? tooManyAttemptsText(e)
            : e.message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          widget.mandatory ? 'Eigenes Passwort festlegen' : 'Passwort ändern',
        ),
        automaticallyImplyLeading: !widget.mandatory,
        actions: [
          if (widget.mandatory)
            TextButton(
              onPressed: _busy ? null : SessionScope.of(context).logout,
              child: const Text('Abmelden'),
            ),
        ],
      ),
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: AutofillGroup(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    if (widget.mandatory) ...[
                      Text(
                        'Du hast dich mit einem Startpasswort angemeldet. '
                        'Bitte lege jetzt ein eigenes Passwort fest.',
                        style: theme.textTheme.bodyLarge,
                      ),
                      const SizedBox(height: 24),
                    ],
                    PasswordField(
                      controller: _current,
                      label: 'Bisheriges Passwort',
                      enabled: !_busy,
                    ),
                    const SizedBox(height: 16),
                    PasswordField(
                      controller: _next,
                      label: 'Neues Passwort',
                      helper:
                          'Mindestens $minPasswordLength Zeichen, höchstens '
                          '$maxPasswordBytes Byte (Umlaute zählen doppelt).',
                      autofillHints: const [AutofillHints.newPassword],
                      enabled: !_busy,
                    ),
                    const SizedBox(height: 16),
                    PasswordField(
                      controller: _repeat,
                      label: 'Neues Passwort wiederholen',
                      autofillHints: const [AutofillHints.newPassword],
                      textInputAction: TextInputAction.done,
                      onSubmitted: (_) => _submit(),
                      enabled: !_busy,
                    ),
                    if (_error case final error?) ...[
                      const SizedBox(height: 12),
                      Text(error, style: TextStyle(color: colors.error)),
                    ],
                    const SizedBox(height: 24),
                    FilledButton(
                      onPressed: _busy ? null : _submit,
                      style: FilledButton.styleFrom(
                        minimumSize: const Size.fromHeight(48),
                      ),
                      child: _busy
                          ? const SizedBox.square(
                              dimension: 20,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            )
                          : const Text('Passwort speichern'),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
