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
const maxFrageLaenge = 500;

/// Eine Frage an die eigenen Dokumente stellen. Die Antwort kommt vom
/// Backend und stützt sich nur auf die angezeigten Quellen. Die App zeigt
/// Antwort und Quellen unverändert an und fügt nichts hinzu.
class FrageScreen extends StatefulWidget {
  const FrageScreen({
    super.key,
    required this.api,
    required this.modulId,
    required this.eintragOeffnen,
  });

  final ApiClient api;

  /// Modul, in dem gefragt wird (`modul` bei `POST /fragen`).
  final String modulId;

  /// Öffnet den Eintrag einer Quelle. „Zurück“ führt wieder hierher.
  final Future<void> Function(String eintragId) eintragOeffnen;

  @override
  State<FrageScreen> createState() => _FrageScreenState();
}

class _FrageScreenState extends State<FrageScreen> {
  final _feld = TextEditingController();

  /// Die zuletzt gestellte Frage; bleibt über der Antwort sichtbar.
  String? _gestellt;
  FrageAntwort? _antwort;
  ApiFehler? _fehler;
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

  Future<void> _fragen([String? frage]) async {
    final text = (frage ?? _feld.text).trim();
    if (text.isEmpty || _sucht) return;
    FocusScope.of(context).unfocus(); // Tastatur schließen
    setState(() {
      _gestellt = text;
      _antwort = null;
      _fehler = null;
      _sucht = true;
    });
    try {
      final antwort = await widget.api.fragen(text, modul: widget.modulId);
      if (!mounted) return;
      setState(() => _antwort = antwort);
    } on ApiFehler catch (fehler) {
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
        actions: [MockHinweis(sichtbar: widget.api.istMock)],
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
      maxLength: maxFrageLaenge,
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
    final antwort = _antwort;
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
                  fehler.nachricht,
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
      if (antwort != null)
        GestaffeltEinblenden(
          key: ObjectKey(antwort),
          kinder: [
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  antwort.antwort,
                  style: theme.textTheme.bodyLarge,
                ),
              ),
            ),
            if (antwort.quellen.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(4, 20, 4, 8),
                child: Text(
                  'Quellen',
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: theme.colorScheme.primary,
                  ),
                ),
              ),
            for (final quelle in antwort.quellen)
              _QuellenKarte(
                quelle: quelle,
                oeffnen: () => widget.eintragOeffnen(quelle.eintragId),
              ),
          ],
        ),
    ];
  }
}

/// Eine Quelle: Nummer wie im Antworttext, Dokument und Auszug.
class _QuellenKarte extends StatelessWidget {
  const _QuellenKarte({required this.quelle, required this.oeffnen});

  final Quelle quelle;
  final VoidCallback oeffnen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final katalog = ModulKatalog.von(context);
    // Typname aus dem Modul der Quelle; ein typ ist nur damit eindeutig.
    final kopf = [
      katalog.typName(quelle.modul, quelle.typ),
      quelle.dokumentdatum?.anzeige ?? 'ohne Datum',
    ].join(' · ');
    return Card(
      child: InkWell(
        onTap: oeffnen,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CircleAvatar(radius: 16, child: Text('${quelle.nr}')),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          katalog.typSymbol(quelle.modul, quelle.typ),
                          size: 16,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(kopf, style: theme.textTheme.labelLarge),
                        ),
                      ],
                    ),
                    Text(
                      quelle.quelle ?? 'Quelle unbekannt',
                      style: theme.textTheme.bodySmall,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '„${quelle.auszug}“',
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
