import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/api_fehler.dart';
import '../api/mime_typ.dart';
import '../model/eintrag.dart';
import '../plattform/datei_auswahl.dart';
import '../widgets/format.dart';
import '../widgets/mock_hinweis.dart';
import '../widgets/navigation.dart';

/// Hinweis zum Datenschutz, steht immer oben im Screen.
const cloudHinweis =
    'Dokumente werden zur Auswertung an einen Cloud-Dienst gesendet. '
    'Bitte nur Testdokumente verwenden.';

/// Foto aufnehmen, Bild oder PDF wählen und hochladen.
///
/// Ablauf: Datei wählen → Vorschau prüfen → „Hochladen“. Die Vorschau
/// verhindert, dass versehentlich die falsche Datei in die Cloud geht.
class UploadScreen extends StatefulWidget {
  const UploadScreen({
    super.key,
    required this.api,
    required this.dateiAuswahl,
    required this.modulId,
    required this.hochgeladen,
    required this.eintragOeffnen,
  });

  final ApiClient api;
  final DateiAuswahl dateiAuswahl;

  /// Modul, in das hochgeladen wird (Feld `modul` beim Upload).
  final String modulId;

  /// Nach erfolgreichem Upload mit dem neuen Eintrag.
  final void Function(Eintrag eintrag) hochgeladen;

  /// Öffnet einen vorhandenen Eintrag (bei einem Duplikat).
  final void Function(String eintragId) eintragOeffnen;

  @override
  State<UploadScreen> createState() => _UploadScreenState();
}

class _UploadScreenState extends State<UploadScreen> {
  AusgewaehlteDatei? _datei;
  ApiFehler? _fehler;
  var _laedtHoch = false;

  /// Bei diesen Fehlern lohnt ein zweiter Versuch mit derselben Datei.
  static const _wiederholbar = {
    FehlerCode.netzwerkFehler,
    FehlerCode.internerFehler,
    FehlerCode.auslesenFehlgeschlagen,
    FehlerCode.modellNichtErreichbar,
    FehlerCode.unbekannteAntwort,
  };

  @override
  void initState() {
    super.initState();
    _verloreneDateiAbholen();
  }

  Future<void> _verloreneDateiAbholen() async {
    try {
      final datei = await widget.dateiAuswahl.verloreneDatei();
      if (datei != null && mounted) _uebernehmen(datei);
    } on PlatformException {
      // Nichts wiederherzustellen; kein Grund für eine Meldung.
    }
  }

  Future<void> _waehlen(Future<AusgewaehlteDatei?> Function() auswahl) async {
    try {
      final datei = await auswahl();
      if (datei != null && mounted) _uebernehmen(datei);
    } on DateiZuGross {
      _zeigeLokalenFehler();
    } on PlatformException catch (e) {
      if (!mounted) return;
      setState(() {
        _fehler = ApiFehler(
          code: 'AUSWAHL_FEHLGESCHLAGEN',
          nachricht: e.code.contains('access_denied')
              ? 'Kein Zugriff auf Kamera oder Fotos. Bitte in den '
                    'Einstellungen des Geräts erlauben.'
              : 'Die Auswahl hat nicht geklappt. Bitte noch einmal versuchen.',
        );
      });
    }
  }

  void _uebernehmen(AusgewaehlteDatei datei) {
    // Größe schon hier prüfen, damit nichts Unnötiges hochgeladen wird.
    // Das Backend prüft trotzdem und meldet ggf. 413.
    if (datei.bytes.length > maxDateiGroesse) {
      _zeigeLokalenFehler();
      return;
    }
    setState(() {
      _datei = datei;
      _fehler = null;
    });
  }

  void _zeigeLokalenFehler() {
    if (!mounted) return;
    setState(() {
      _datei = null;
      _fehler = const ApiFehler(
        code: FehlerCode.dateiZuGross,
        nachricht:
            'Die Datei ist größer als 10 MB. Bitte eine kleinere Datei wählen.',
      );
    });
  }

  Future<void> _hochladen() async {
    final datei = _datei;
    if (datei == null) return;
    setState(() {
      _laedtHoch = true;
      _fehler = null;
    });
    try {
      final eintrag = await widget.api.hochladen(
        datei.bytes,
        datei.name,
        modul: widget.modulId,
      );
      if (!mounted) return;
      setState(() {
        _laedtHoch = false;
        _datei = null;
      });
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Dokument gespeichert.')));
      widget.hochgeladen(eintrag);
    } on ApiFehler catch (fehler) {
      if (!mounted) return;
      setState(() {
        _laedtHoch = false;
        _fehler = fehler;
        // Ohne Aussicht auf Erfolg die Datei verwerfen, damit direkt eine
        // andere gewählt werden kann.
        if (!_wiederholbar.contains(fehler.code)) _datei = null;
      });
    }
  }

  void _verwerfen() => setState(() {
    _datei = null;
    _fehler = null;
  });

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: const BereicheKnopf(),
        title: const Text('Hochladen'),
        actions: [MockHinweis(sichtbar: widget.api.istMock)],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          const _Hinweis(),
          const SizedBox(height: 16),
          if (_fehler case final fehler?) ...[
            _FehlerKarte(fehler: fehler, eintragOeffnen: widget.eintragOeffnen),
            const SizedBox(height: 16),
          ],
          if (_datei case final datei?) _vorschau(datei) else _auswahlKnoepfe(),
        ],
      ),
    );
  }

  Widget _auswahlKnoepfe() {
    final auswahl = widget.dateiAuswahl;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        FilledButton.icon(
          onPressed: () => _waehlen(auswahl.fotoAufnehmen),
          icon: const Icon(Icons.photo_camera_outlined),
          label: const Text('Foto aufnehmen'),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: () => _waehlen(auswahl.bildAuswaehlen),
          icon: const Icon(Icons.photo_library_outlined),
          label: const Text('Bild auswählen'),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: () => _waehlen(auswahl.pdfAuswaehlen),
          icon: const Icon(Icons.picture_as_pdf_outlined),
          label: const Text('PDF auswählen'),
        ),
        const SizedBox(height: 12),
        Text(
          'Erlaubt sind JPEG, PNG, HEIC und PDF bis 10 MB.',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ],
    );
  }

  Widget _vorschau(AusgewaehlteDatei datei) {
    final mimeTyp = erkenneMimeTyp(datei.bytes);
    final istBild = mimeTyp != null && mimeTyp.startsWith('image/');
    final wiederholen = _fehler != null;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Card(
          clipBehavior: Clip.antiAlias,
          // Stack legt das X-Symbol über die Vorschau, oben rechts.
          child: Stack(
            children: [
              Column(
                children: [
                  if (istBild)
                    Image.memory(
                      datei.bytes,
                      height: 260,
                      fit: BoxFit.contain,
                      // HEIC kann Android evtl. nicht anzeigen; dann ein Symbol.
                      errorBuilder: (_, _, _) =>
                          const _DateiSymbol(Icons.image),
                    )
                  else
                    const _DateiSymbol(Icons.picture_as_pdf_outlined),
                  ListTile(
                    title: Text(datei.name),
                    subtitle: Text(formatGroesse(datei.bytes.length)),
                  ),
                ],
              ),
              // Während des Uploads ausgeblendet: Die Datei ist dann schon
              // unterwegs und soll nicht mitten im Vorgang verschwinden.
              if (!_laedtHoch)
                Positioned(
                  top: 4,
                  right: 4,
                  child: IconButton.filledTonal(
                    onPressed: _verwerfen,
                    icon: const Icon(Icons.close),
                    tooltip: 'Datei verwerfen',
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        if (_laedtHoch)
          const Column(
            children: [
              CircularProgressIndicator(),
              SizedBox(height: 12),
              Text(
                'Das Dokument wird ausgewertet.\n'
                'Das kann bis zu einer Minute dauern.',
                textAlign: TextAlign.center,
              ),
            ],
          )
        else ...[
          FilledButton.icon(
            onPressed: _hochladen,
            icon: Icon(wiederholen ? Icons.refresh : Icons.cloud_upload),
            label: Text(wiederholen ? 'Erneut versuchen' : 'Hochladen'),
          ),
          const SizedBox(height: 8),
          TextButton(
            onPressed: _verwerfen,
            child: const Text('Andere Datei wählen'),
          ),
        ],
      ],
    );
  }
}

class _Hinweis extends StatelessWidget {
  const _Hinweis();

  @override
  Widget build(BuildContext context) {
    final farben = Theme.of(context).colorScheme;
    return Card(
      color: farben.secondaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(Icons.info_outline, color: farben.onSecondaryContainer),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                cloudHinweis,
                style: TextStyle(color: farben.onSecondaryContainer),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _FehlerKarte extends StatelessWidget {
  const _FehlerKarte({required this.fehler, required this.eintragOeffnen});

  final ApiFehler fehler;
  final void Function(String eintragId) eintragOeffnen;

  @override
  Widget build(BuildContext context) {
    final farben = Theme.of(context).colorScheme;
    final vorhandeneId = fehler.code == FehlerCode.duplikat
        ? fehler.details['eintragId']
        : null;
    return Card(
      color: farben.errorContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.error_outline, color: farben.onErrorContainer),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    fehler.nachricht,
                    style: TextStyle(color: farben.onErrorContainer),
                  ),
                ),
              ],
            ),
            if (vorhandeneId is String)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: () => eintragOeffnen(vorhandeneId),
                  child: const Text('Vorhandenen Eintrag öffnen'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _DateiSymbol extends StatelessWidget {
  const _DateiSymbol(this.icon);

  final IconData icon;

  @override
  Widget build(BuildContext context) => SizedBox(
    height: 120,
    child: Center(
      child: Icon(
        icon,
        size: 64,
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
    ),
  );
}
