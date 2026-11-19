import 'package:flutter/material.dart';

import '../api/api_exception.dart';
import '../api/mock_api_client.dart';
import '../session/password_rules.dart';
import '../session/session_controller.dart';
import '../widgets/mock_banner.dart';
import '../widgets/password_field.dart';

/// Login with username and password (docs/API.md 3.10).
///
/// Shown by the session while there is no valid token. There is no
/// „Passwort vergessen“ in the test phase; accounts are created by us.
class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _username = TextEditingController();
  final _password = TextEditingController();
  var _busy = false;
  String? _error;

  @override
  void dispose() {
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) return;
    final username = _username.text.trim();
    final password = _password.text;
    if (username.isEmpty || password.isEmpty) {
      setState(() => _error = 'Bitte Benutzername und Passwort eingeben.');
      return;
    }
    FocusScope.of(context).unfocus();
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await SessionScope.of(context).login(username, password);
      // Success: the session shows the next screen; this one goes away.
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = loginErrorText(e);
        // A wrong password is typed again from scratch.
        _password.clear();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final session = SessionScope.of(context);
    final theme = Theme.of(context);
    final colors = theme.colorScheme;
    final notice = session.notice;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Anmelden'),
        actions: [MockBanner(visible: session.api.isMock)],
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
                    Icon(Icons.lock_outline, size: 48, color: colors.primary),
                    const SizedBox(height: 12),
                    Text(
                      'Willkommen bei Lifelog',
                      textAlign: TextAlign.center,
                      style: theme.textTheme.headlineSmall,
                    ),
                    const SizedBox(height: 24),
                    if (notice != null) ...[
                      _NoticeCard(text: notice),
                      const SizedBox(height: 16),
                    ],
                    TextField(
                      controller: _username,
                      enabled: !_busy,
                      // Usernames are lowercase letters and digits; the
                      // keyboard must not capitalize or "correct" them.
                      autocorrect: false,
                      enableSuggestions: false,
                      textCapitalization: TextCapitalization.none,
                      keyboardType: TextInputType.visiblePassword,
                      autofillHints: const [AutofillHints.username],
                      textInputAction: TextInputAction.next,
                      decoration: const InputDecoration(
                        labelText: 'Benutzername',
                        border: OutlineInputBorder(),
                      ),
                    ),
                    const SizedBox(height: 16),
                    PasswordField(
                      controller: _password,
                      label: 'Passwort',
                      enabled: !_busy,
                      textInputAction: TextInputAction.done,
                      onSubmitted: (_) => _submit(),
                    ),
                    if (_error case final error?) ...[
                      const SizedBox(height: 12),
                      Text(
                        error,
                        style: TextStyle(color: colors.error),
                        // Read out by a screen reader as soon as it appears.
                        semanticsLabel: error,
                      ),
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
                          : const Text('Anmelden'),
                    ),
                    const SizedBox(height: 16),
                    Text(
                      'Passwort vergessen? Melde dich bei uns.',
                      textAlign: TextAlign.center,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: colors.onSurfaceVariant,
                      ),
                    ),
                    if (session.api.isMock) ...[
                      const SizedBox(height: 24),
                      Text(
                        'Mock-Zugänge: tester01 / '
                        '${MockApiClient.mockStartPassword} oder admin / '
                        '${MockApiClient.mockAdminPassword}',
                        textAlign: TextAlign.center,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: colors.onSurfaceVariant,
                        ),
                      ),
                    ],
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

/// Text for an error at login. Wrong credentials always get the same
/// sentence; a lockout adds the wait in minutes.
String loginErrorText(ApiException error) => switch (error.code) {
  ErrorCode.invalidCredentials => 'Benutzername oder Passwort stimmt nicht.',
  ErrorCode.tooManyAttempts => tooManyAttemptsText(error),
  _ => error.message,
};

/// The server's message plus „Bitte warte etwa N Minuten.“ if it sent the
/// wait.
String tooManyAttemptsText(ApiException error) =>
    switch (error.retryAfterSeconds) {
      final seconds? => '${error.message} ${waitHint(seconds)}',
      null => error.message,
    };

class _NoticeCard extends StatelessWidget {
  const _NoticeCard({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Card(
      color: colors.secondaryContainer,
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(Icons.info_outline, color: colors.onSecondaryContainer),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                text,
                style: TextStyle(color: colors.onSecondaryContainer),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
