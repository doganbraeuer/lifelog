import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/answer.dart';
import '../module/module_catalog.dart';
import '../widgets/motion.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import '../widgets/skeleton.dart';

/// Höchstlänge einer Frage laut docs/API.md.
const maxQuestionLength = 500;

/// Eine Frage an die eigenen Dokumente stellen. Die Antwort kommt vom
/// Backend und stützt sich nur auf die angezeigten Quellen. Die App zeigt
/// Antwort und Quellen unverändert an und fügt nichts hinzu.
class FrageScreen extends StatefulWidget {
  const FrageScreen({
    super.key,
    required this.api,
    required this.moduleId,
    required this.eintragOeffnen,
  });

  final ApiClient api;

  /// Modul, in dem gefragt wird (`modul` bei `POST /fragen`).
  final String moduleId;

  /// Öffnet den Eintrag einer Quelle. „Zurück“ führt wieder hierher.
  final Future<void> Function(String entryId) eintragOeffnen;

  @override
  State<FrageScreen> createState() => _FrageScreenState();
}

class _FrageScreenState extends State<FrageScreen> {
  final _feld = TextEditingController();

  /// Die zuletzt gestellte Frage; bleibt über der Antwort sichtbar.
  String? _gestellt;
  Answer? _antwort;
  ApiException? _fehler;
  var _sucht = false;

  @override
  void initState() {
    super.initState();
    // Damit der Senden-Knopf beim Tippen aktiv/inaktiv wird.
    _feld.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _feld.dispose();
    super.dispose();
  }

  bool get _kannSenden => !_sucht && _feld.text.trim().isNotEmpty;

  Future<void> _fragen([String? question]) async {
    final text = (question ?? _feld.text).trim();
    if (text.isEmpty || _sucht) return;
    FocusScope.of(context).unfocus(); // Tastatur schließen
    setState(() {
      _gestellt = text;
      _antwort = null;
      _fehler = null;
      _sucht = true;
    });
    try {
      final answer = await widget.api.ask(text, module: widget.moduleId);
      if (!mounted) return;
      setState(() => _antwort = answer);
    } on ApiException catch (fehler) {
      if (mounted) setState(() => _fehler = fehler);
    } finally {
      if (mounted) setState(() => _sucht = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: const BereicheKnopf(),
        title: const Text('Fragen'),
        actions: [MockHinweis(sichtbar: widget.api.isMock)],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [_eingabe(), const SizedBox(height: 16), ..._ergebnis()],
      ),
    );
  }

  Widget _eingabe() {
    return TextField(
      controller: _feld,
      minLines: 1,
      maxLines: 4,
      maxLength: maxQuestionLength,
      textInputAction: TextInputAction.send,
      onSubmitted: (_) => _fragen(),
      decoration: InputDecoration(
        labelText: 'Deine Frage',
        hintText: 'z. B. Welchen Hämoglobinwert hatte ich im Oktober?',
        border: const OutlineInputBorder(),
        suffixIcon: IconButton(
          icon: const Icon(Icons.send),
          tooltip: 'Frage senden',
          onPressed: _kannSenden ? _fragen : null,
        ),
      ),
    );
  }

  List<Widget> _ergebnis() {
    final gestellt = _gestellt;
    if (gestellt == null) {
      return [
        Text(
          'Die Antwort stützt sich nur auf deine hochgeladenen Dokumente '
          'und nennt die Stellen, aus denen sie stammt.',
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ];
    }

    final theme = Theme.of(context);
    final fehler = _fehler;
    final answer = _antwort;
    return [
      Text('„$gestellt“', style: theme.textTheme.titleMedium),
      const SizedBox(height: 12),
      if (_sucht) ...[
        Text(
          'Suche in deinen Dokumenten …',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 12),
        // Platzhalter in der Form von Antwort und Quellen.
        const AntwortPlatzhalter(),
      ],
      if (fehler != null)
        Card(
          color: theme.colorScheme.errorContainer,
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  fehler.message,
                  style: TextStyle(color: theme.colorScheme.onErrorContainer),
                ),
                Align(
                  alignment: Alignment.centerRight,
                  child: TextButton(
                    onPressed: () => _fragen(gestellt),
                    child: const Text('Erneut versuchen'),
                  ),
                ),
              ],
            ),
          ),
        ),
      // Antwort und Quellen blenden nacheinander ein; der Text ist sofort
      // vollständig da (kein Tipp-Effekt). Neue Antwort, neuer Durchlauf.
      if (answer != null)
        GestaffeltEinblenden(
          key: ObjectKey(answer),
          kinder: [
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  answer.answer,
                  style: theme.textTheme.bodyLarge,
                ),
              ),
            ),
            if (answer.sources.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(4, 20, 4, 8),
                child: Text(
                  'Quellen',
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: theme.colorScheme.primary,
                  ),
                ),
              ),
            for (final source in answer.sources)
              _QuellenKarte(
                source: source,
                oeffnen: () => widget.eintragOeffnen(source.entryId),
              ),
          ],
        ),
    ];
  }
}

/// Eine Quelle: Nummer wie im Antworttext, Dokument und Auszug.
class _QuellenKarte extends StatelessWidget {
  const _QuellenKarte({required this.source, required this.oeffnen});

  final Source source;
  final VoidCallback oeffnen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final catalog = ModuleCatalog.of(context);
    // Typname aus dem Modul der Quelle; ein typ ist nur damit eindeutig.
    final kopf = [
      catalog.typeName(source.module, source.type),
      source.documentDate?.display ?? 'ohne Datum',
    ].join(' · ');
    return Card(
      child: InkWell(
        onTap: oeffnen,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CircleAvatar(radius: 16, child: Text('${source.number}')),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          catalog.typeIcon(source.module, source.type),
                          size: 16,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(kopf, style: theme.textTheme.labelLarge),
                        ),
                      ],
                    ),
                    Text(
                      source.issuer ?? 'Quelle unbekannt',
                      style: theme.textTheme.bodySmall,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '„${source.excerpt}“',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right),
            ],
          ),
        ),
      ),
    );
  }
}
