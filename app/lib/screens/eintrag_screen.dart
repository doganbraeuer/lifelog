import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_fehler.dart';
import '../model/eintrag.dart';
import '../model/typ.dart';
import '../widgets/fehler_anzeige.dart';
import '../widgets/format.dart';
import '../widgets/mock_hinweis.dart';

/// Zeigt einen Eintrag. Vorerst nur lesend; Korrekturen und die
/// Originaldatei kommen in Etappe e und g.
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

  @override
  void initState() {
    super.initState();
    _laden();
  }

  Future<void> _laden() async {
    setState(() => _fehler = null);
    try {
      final eintrag = await widget.api.eintrag(widget.eintragId);
      if (mounted) setState(() => _eintrag = eintrag);
    } on ApiFehler catch (fehler) {
      if (mounted) setState(() => _fehler = fehler);
    }
  }

  @override
  Widget build(BuildContext context) {
    final eintrag = _eintrag;
    return Scaffold(
      appBar: AppBar(
        title: Text(eintrag == null ? 'Eintrag' : typAnzeige(eintrag.typ)),
        actions: [MockHinweis(sichtbar: widget.api.istMock)],
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

    final ueberschrift = Theme.of(context).textTheme.titleSmall;
    return ListView(
      padding: const EdgeInsets.symmetric(vertical: 8),
      children: [
        ListTile(
          title: const Text('Dokumentdatum'),
          subtitle: Text(eintrag.dokumentdatum?.anzeige ?? 'nicht erkannt'),
        ),
        ListTile(
          title: const Text('Quelle'),
          subtitle: Text(eintrag.quelle ?? 'nicht erkannt'),
        ),
        ListTile(
          title: const Text('Importiert'),
          subtitle: Text(formatZeitpunktLokal(eintrag.importdatum)),
        ),
        ListTile(
          title: const Text('Originaldatei'),
          subtitle: Text(eintrag.datei.dateiname),
        ),
        const Divider(),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
          child: Text('Erkannte Angaben', style: ueberschrift),
        ),
        for (final info in eintrag.infos)
          ListTile(
            title: Text(info.name),
            trailing: Text(
              [info.wert, ?info.einheit].join(' '),
              style: Theme.of(context).textTheme.bodyLarge,
            ),
          ),
        const Divider(),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 4),
          child: Text('Volltext', style: ueberschrift),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: SelectableText(eintrag.volltext),
        ),
        const SizedBox(height: 24),
      ],
    );
  }
}
