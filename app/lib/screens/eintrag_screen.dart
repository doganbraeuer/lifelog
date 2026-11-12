import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/api_fehler.dart';
import '../model/datum.dart';
import '../model/eintrag.dart';
import '../model/eintrag_aenderung.dart';
import '../module/modul_katalog.dart';
import '../widgets/fehler_anzeige.dart';
import '../widgets/format.dart';
import '../widgets/mock_hinweis.dart';
import '../widgets/navigation.dart';
import 'korrektur_dialoge.dart';
import 'originaldatei_screen.dart';

/// Zeigt einen Eintrag und erlaubt, falsch erkannte Werte zu korrigieren.
///
/// Jede Korrektur ist ein eigener PATCH. Angezeigt wird danach immer die
/// Antwort des Backends; die App rechnet nichts selbst nach.
class EintragScreen extends StatefulWidget {
  const EintragScreen({super.key, required this.api, required this.eintragId});

  final ApiClient api;
  final String eintragId;

  @override
  State<EintragScreen> createState() => _EintragScreenState();
}

class _EintragScreenState extends State<EintragScreen> {
  Eintrag? _eintrag;
  ApiFehler? _fehler;
  var _speichert = false;

  /// Originaldatei (nur bei Bildern geladen). Wird einmal angelegt, damit
  /// das Bild nicht bei jeder Korrektur neu geladen wird.
  Future<DateiInhalt>? _bild;

  @override
  void initState() {
    super.initState();
    _laden();
  }

  Future<void> _laden() async {
    setState(() => _fehler = null);
    try {
      final eintrag = await widget.api.eintrag(widget.eintragId);
      if (!mounted) return;
      setState(() {
        _eintrag = eintrag;
        if (eintrag.datei.istBild) _bild = widget.api.datei(eintrag.id);
      });
    } on ApiFehler catch (fehler) {
      if (mounted) setState(() => _fehler = fehler);
    }
  }

  // -------------------------------------------------------------------------
  // Korrigieren
  // -------------------------------------------------------------------------

  Future<void> _speichern(EintragAenderung aenderung) async {
    if (aenderung.istLeer) return;
    setState(() => _speichert = true);
    final nachricht = ScaffoldMessenger.of(context);
    try {
      final eintrag = await widget.api.korrigieren(widget.eintragId, aenderung);
      if (!mounted) return;
      setState(() => _eintrag = eintrag);
      HapticFeedback.lightImpact(); // Erfolg spürbar machen
      nachricht.showSnackBar(
        const SnackBar(content: Text('Korrektur gespeichert.')),
      );
    } on ApiFehler catch (fehler) {
      // Der alte Stand bleibt angezeigt; das Backend hat nichts geändert.
      nachricht.showSnackBar(SnackBar(content: Text(fehler.nachricht)));
    } finally {
      if (mounted) setState(() => _speichert = false);
    }
  }

  Future<void> _typKorrigieren(Eintrag eintrag) async {
    final typ = await zeigeTypDialog(
      context,
      // Erlaubt sind die Typen des Moduls, zu dem der Eintrag gehört.
      modulId: eintrag.modul,
      aktuell: eintrag.typ,
      korrektur: eintrag.korrektur('type'),
    );
    if (typ != null && typ != eintrag.typ) {
      await _speichern(EintragAenderung(typ: typ));
    }
  }

  Future<void> _datumKorrigieren(Eintrag eintrag) async {
    final datum = await zeigeDatumAuswahl(
      context,
      aktuell: eintrag.dokumentdatum,
      korrektur: eintrag.korrektur('documentDate'),
    );
    if (datum != null && datum.wert != eintrag.dokumentdatum) {
      await _speichern(EintragAenderung(dokumentdatum: datum));
    }
  }

  Future<void> _quelleKorrigieren(Eintrag eintrag) async {
    final quelle = await zeigeQuelleDialog(
      context,
      aktuell: eintrag.quelle,
      korrektur: eintrag.korrektur('issuer'),
    );
    if (quelle != null && quelle.wert != eintrag.quelle) {
      await _speichern(EintragAenderung(quelle: quelle));
    }
  }

  Future<void> _infoKorrigieren(Eintrag eintrag, Info info) async {
    final aenderung = await zeigeInfoDialog(
      context,
      info: info,
      wertKorrektur: eintrag.korrektur('facts.${info.id}.value'),
      einheitKorrektur: eintrag.korrektur('facts.${info.id}.unit'),
    );
    if (aenderung != null) {
      await _speichern(EintragAenderung(infos: [aenderung]));
    }
  }

  void _originalOeffnen(Eintrag eintrag, DateiInhalt? vorhanden) {
    oeffneSeite<void>(
      context,
      OriginaldateiScreen(
        api: widget.api,
        eintragId: eintrag.id,
        dateiname: eintrag.datei.dateiname,
        vorhanden: vorhanden,
      ),
    );
  }

  // -------------------------------------------------------------------------
  // Löschen
  // -------------------------------------------------------------------------

  /// Fragt nach und löscht dann Eintrag, Originaldatei und Suchabschnitte.
  /// Bei Erfolg schließt sich die Ansicht; die Zeitleiste lädt danach neu.
  Future<void> _loeschen() async {
    final bestaetigt = await showDialog<bool>(
      context: context,
      builder: (dialog) {
        final farben = Theme.of(dialog).colorScheme;
        return AlertDialog(
          title: const Text('Eintrag löschen?'),
          content: const Text(
            'Eintrag, Originaldatei und Suchabschnitte werden endgültig '
            'gelöscht.',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialog, false),
              child: const Text('Abbrechen'),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: farben.error,
                foregroundColor: farben.onError,
              ),
              onPressed: () => Navigator.pop(dialog, true),
              child: const Text('Löschen'),
            ),
          ],
        );
      },
    );
    if (bestaetigt != true || !mounted) return;

    final navigator = Navigator.of(context);
    final nachricht = ScaffoldMessenger.of(context);
    setState(() => _speichert = true);
    try {
      await widget.api.loeschen(widget.eintragId);
      navigator.pop();
      HapticFeedback.lightImpact();
      nachricht.showSnackBar(
        const SnackBar(content: Text('Eintrag gelöscht.')),
      );
    } on ApiFehler catch (fehler) {
      // Nichts gelöscht (bzw. der Eintrag ist noch sichtbar): Ansicht bleibt.
      nachricht.showSnackBar(SnackBar(content: Text(fehler.nachricht)));
      if (mounted) setState(() => _speichert = false);
    }
  }

  // -------------------------------------------------------------------------
  // Anzeige
  // -------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    final eintrag = _eintrag;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          eintrag == null
              ? 'Eintrag'
              : ModulKatalog.von(context).typName(eintrag.modul, eintrag.typ),
        ),
        actions: [
          MockHinweis(sichtbar: widget.api.istMock),
          IconButton(
            icon: const Icon(Icons.delete_outline),
            tooltip: 'Eintrag löschen',
            onPressed: eintrag == null || _speichert ? null : _loeschen,
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(4),
          child: SizedBox(
            height: 4,
            child: _speichert ? const LinearProgressIndicator() : null,
          ),
        ),
      ),
      body: _inhalt(context),
    );
  }

  Widget _inhalt(BuildContext context) {
    final fehler = _fehler;
    if (fehler != null) return FehlerAnzeige(fehler: fehler, erneut: _laden);
    final eintrag = _eintrag;
    if (eintrag == null) {
      return const Center(child: CircularProgressIndicator());
    }

    // Während gespeichert wird, sind die Stifte gesperrt.
    VoidCallback? bearbeiten(Future<void> Function() aktion) =>
        _speichert ? null : aktion;

    // Typnamen kommen aus dem Modul, zu dem der Eintrag gehört.
    final katalog = ModulKatalog.von(context);
    String typName(String typ) => katalog.typName(eintrag.modul, typ);

    return ListView(
      padding: const EdgeInsets.only(bottom: 24),
      children: [
        _Originaldatei(
          datei: eintrag.datei,
          bild: _bild,
          oeffnen: (vorhanden) => _originalOeffnen(eintrag, vorhanden),
        ),
        const _Abschnitt('Angaben'),
        _FeldZeile(
          label: 'Typ',
          wert: typName(eintrag.typ),
          korrektur: _hinweis(eintrag.korrektur('type'), typName),
          bearbeiten: bearbeiten(() => _typKorrigieren(eintrag)),
        ),
        _FeldZeile(
          label: 'Dokumentdatum',
          wert: eintrag.dokumentdatum?.anzeige ?? 'nicht erkannt',
          korrektur: _hinweis(
            eintrag.korrektur('documentDate'),
            (iso) => Datum.tryParse(iso)?.anzeige ?? iso,
          ),
          bearbeiten: bearbeiten(() => _datumKorrigieren(eintrag)),
        ),
        _FeldZeile(
          label: 'Quelle',
          wert: eintrag.quelle ?? 'nicht erkannt',
          korrektur: _hinweis(eintrag.korrektur('issuer')),
          bearbeiten: bearbeiten(() => _quelleKorrigieren(eintrag)),
        ),
        _FeldZeile(
          label: 'Importiert',
          wert: formatZeitpunktLokal(eintrag.importdatum),
          korrigierbar: false,
        ),
        const _Abschnitt('Erkannte Angaben'),
        if (eintrag.infos.isEmpty)
          const ListTile(title: Text('Keine Angaben erkannt.')),
        for (final info in eintrag.infos)
          _FeldZeile(
            label: info.name,
            wert: [info.wert, ?info.einheit].join(' '),
            korrektur: [
              ?_hinweis(
                eintrag.korrektur('facts.${info.id}.value'),
                null,
                'Wert',
              ),
              ?_hinweis(
                eintrag.korrektur('facts.${info.id}.unit'),
                null,
                'Einheit',
              ),
            ].join('\n').nullWennLeer,
            bearbeiten: bearbeiten(() => _infoKorrigieren(eintrag, info)),
          ),
        const _Abschnitt('Volltext'),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: SelectableText(eintrag.volltext),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
          child: Text(
            'Der Volltext kann im Prototyp nicht korrigiert werden.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ),
      ],
    );
  }

  /// Text für korrigierte Felder, z. B. „korrigiert, erkannt: 6,3“.
  /// [anzeige] wandelt den erkannten Rohwert für die Anzeige um.
  String? _hinweis(
    Korrektur? korrektur, [
    String Function(String)? anzeige,
    String? feldname,
  ]) {
    if (korrektur == null) return null;
    final erkannt = korrektur.erkannt;
    final text = erkannt == null
        ? 'nichts'
        : (anzeige == null ? erkannt : anzeige(erkannt));
    final praefix = feldname == null ? '' : '$feldname ';
    return '${praefix}korrigiert, erkannt: $text';
  }
}

extension on String {
  String? get nullWennLeer => isEmpty ? null : this;
}

/// Überschrift eines Abschnitts.
class _Abschnitt extends StatelessWidget {
  const _Abschnitt(this.titel);

  final String titel;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 20, 16, 4),
      child: Text(
        titel,
        style: Theme.of(context).textTheme.titleSmall
            ?.copyWith(color: Theme.of(context).colorScheme.primary),
      ),
    );
  }
}

/// Eine Zeile mit Bezeichnung, Wert, ggf. Korrektur-Hinweis und Stift.
class _FeldZeile extends StatelessWidget {
  const _FeldZeile({
    required this.label,
    required this.wert,
    this.korrektur,
    this.bearbeiten,
    this.korrigierbar = true,
  });

  final String label;
  final String wert;

  /// Hinweis wie „korrigiert, erkannt: 6,3“, sonst `null`.
  final String? korrektur;

  /// `null` sperrt den Stift (z. B. während gespeichert wird).
  final VoidCallback? bearbeiten;

  /// `false`: gar kein Stift (z. B. beim Importdatum).
  final bool korrigierbar;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final korrektur = this.korrektur;
    return ListTile(
      title: Text(
        label,
        style: theme.textTheme.labelMedium?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(wert, style: theme.textTheme.bodyLarge),
          if (korrektur != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    Icons.edit_note,
                    size: 16,
                    color: theme.colorScheme.tertiary,
                  ),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Text(
                      korrektur,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.tertiary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
      trailing: korrigierbar
          ? IconButton(
              icon: const Icon(Icons.edit_outlined),
              tooltip: '$label korrigieren',
              onPressed: bearbeiten,
            )
          : null,
    );
  }
}

/// Originaldatei: Bilder als Vorschau, bei PDFs Name und Größe. Darunter
/// der Knopf zum Öffnen; ein Tipp auf das Vorschaubild öffnet ebenfalls.
class _Originaldatei extends StatelessWidget {
  const _Originaldatei({
    required this.datei,
    required this.bild,
    required this.oeffnen,
  });

  final Datei datei;
  final Future<DateiInhalt>? bild;

  /// Öffnet die Datei; [vorhanden] ist das schon geladene Bild, falls da.
  final void Function(DateiInhalt? vorhanden) oeffnen;

  @override
  Widget build(BuildContext context) {
    final bild = this.bild;
    final (symbol, art) = datei.istPdf
        ? (Icons.picture_as_pdf_outlined, 'PDF')
        : datei.istText
        ? (Icons.article_outlined, 'Text')
        : (Icons.image_outlined, 'Bild');
    return Card(
      margin: const EdgeInsets.all(16),
      clipBehavior: Clip.antiAlias,
      child: Column(
        children: [
          if (bild != null)
            FutureBuilder<DateiInhalt>(
              future: bild,
              builder: (context, snapshot) {
                if (snapshot.hasError) {
                  return const _BildPlatzhalter(
                    kind: Text('Originaldatei konnte nicht geladen werden.'),
                  );
                }
                final inhalt = snapshot.data;
                if (inhalt == null) {
                  return const _BildPlatzhalter(
                    kind: CircularProgressIndicator(),
                  );
                }
                return InkWell(
                  onTap: () => oeffnen(inhalt),
                  child: Image.memory(
                    inhalt.bytes,
                    // Volle Breite: Die ganze Fläche ist antippbar, auch
                    // bevor das Bild dekodiert ist.
                    width: double.infinity,
                    height: 220,
                    fit: BoxFit.contain,
                    errorBuilder: (_, _, _) => const _BildPlatzhalter(
                      kind: Icon(Icons.broken_image_outlined, size: 48),
                    ),
                  ),
                );
              },
            ),
          ListTile(
            leading: Icon(symbol),
            title: Text(datei.dateiname),
            subtitle: Text('${formatGroesse(datei.groesse)} · $art'),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: SizedBox(
              width: double.infinity,
              child: FilledButton.tonalIcon(
                onPressed: () => oeffnen(null),
                icon: const Icon(Icons.open_in_full),
                label: const Text('Originaldatei öffnen'),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _BildPlatzhalter extends StatelessWidget {
  const _BildPlatzhalter({required this.kind});

  final Widget kind;

  @override
  Widget build(BuildContext context) =>
      SizedBox(height: 220, child: Center(child: kind));
}
