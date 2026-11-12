// Display of the verification in the entry: a calm summary at the top and
// an explanation behind an info button. The marks per value are drawn by
// the rows of the entry view (entry_screen.dart).
//
// Important (Dogan): "not found" means the value could not be found in the
// original, not that it is wrong. So no warning colors.

import 'package:flutter/material.dart';

import '../model/entry.dart';
import '../model/verification.dart';
import '../widgets/format.dart';

/// Mark next to a confirmed value, also read out by screen readers.
const foundLabel = 'Im Original gefunden';

/// Line under an unconfirmed value.
const notFoundLabel = 'Nicht im Original gefunden';

/// Tooltip of the info button.
const explanationTooltip = 'Was bedeutet die Prüfung?';

/// Headline of the summary, e.g. „5 von 6 Werten im Original gefunden“.
String verificationTitle(Verification verification) {
  if (!verification.isChecked) return 'Nicht geprüft';
  final total = verification.total;
  if (total == 0) return 'Keine Werte zum Prüfen';
  final values = total == 1 ? 'Wert' : 'Werten';
  return '${verification.confirmed} von $total $values im Original gefunden';
}

/// Second line of the summary: what was compared against (plus the number
/// of corrected values), or why the entry was not checked.
String verificationDetail(Verification verification, File file) {
  if (!verification.isChecked) return reasonText(verification.reason);
  final corrected = verification.corrected;
  return [
    sourceText(verification.source, file),
    if (corrected > 0)
      '$corrected ${corrected == 1 ? 'Wert' : 'Werte'} korrigiert',
  ].join(' · ');
}

/// What the values were compared against, in plain German.
String sourceText(VerificationSource? source, File file) => switch (source) {
  VerificationSource.pdfText => 'Abgleich mit der Textebene des PDF',
  VerificationSource.ocr when file.isPdf =>
    'Abgleich per Texterkennung im gescannten PDF',
  VerificationSource.ocr when file.isImage =>
    'Abgleich per Texterkennung im Foto',
  VerificationSource.ocr => 'Abgleich per Texterkennung',
  VerificationSource.text => 'Abgleich mit dem eingegebenen Text',
  VerificationSource.mixed => 'Abgleich mit Textebene und Texterkennung im PDF',
  VerificationSource.unknown || null => 'Abgleich mit dem Original',
};

/// Why an entry was not checked. Neutral, not an error.
String reasonText(VerificationReason? reason) => switch (reason) {
  VerificationReason.ocrUnavailable =>
    'Die Prüfung ist auf diesem Server nicht eingerichtet.',
  VerificationReason.ocrFailed =>
    'Das Original konnte beim Prüfen nicht gelesen werden.',
  VerificationReason.unsupportedFileType =>
    'Dieser Dateityp kann nicht geprüft werden.',
  VerificationReason.tooManyPages =>
    'Das PDF hat mehr als 10 Seiten und wird deshalb nicht geprüft.',
  VerificationReason.imageTooLarge => 'Das Bild ist zu groß, um es zu prüfen.',
  VerificationReason.noText => 'Im Original wurde kein lesbarer Text gefunden.',
  VerificationReason.notRun ||
  VerificationReason.unknown ||
  null => 'Dieser Eintrag wurde noch nicht geprüft.',
};

/// Summary at the top of the entry, with the info button for the
/// explanation.
class VerificationSummary extends StatelessWidget {
  const VerificationSummary({
    super.key,
    required this.verification,
    required this.file,
  });

  final Verification verification;
  final File file;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final checked = verification.isChecked;
    return Card(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 4),
      elevation: 0,
      color: colors.surfaceContainerHigh,
      child: ListTile(
        contentPadding: const EdgeInsets.only(left: 16, right: 4),
        leading: Icon(
          checked ? Icons.fact_check_outlined : Icons.remove_done,
          color: checked ? colors.primary : colors.onSurfaceVariant,
        ),
        title: Text(verificationTitle(verification)),
        subtitle: Text(verificationDetail(verification, file)),
        trailing: IconButton(
          icon: const Icon(Icons.info_outline),
          tooltip: explanationTooltip,
          onPressed: () => showVerificationExplanation(context, verification),
        ),
      ),
    );
  }
}

/// A short explanation of the check, as a bottom sheet.
Future<void> showVerificationExplanation(
  BuildContext context,
  Verification verification,
) {
  return showModalBottomSheet<void>(
    context: context,
    showDragHandle: true,
    isScrollControlled: true,
    builder: (sheet) {
      final theme = Theme.of(sheet);
      final small = theme.textTheme.bodySmall?.copyWith(
        color: theme.colorScheme.onSurfaceVariant,
      );
      final engine = verification.engine;
      final checkedAt = verification.checkedAt;
      return SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.fromLTRB(24, 0, 24, 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                'So prüft Lifelog die Werte',
                style: theme.textTheme.titleLarge,
              ),
              const SizedBox(height: 12),
              const Text(
                'Lifelog liest das Original ein zweites Mal, unabhängig von '
                'der Auswertung: bei PDFs die Textebene, bei Fotos und Scans '
                'mit Texterkennung, bei eingegebenem Text den Text selbst. '
                'Dann sucht es Dokumentdatum, Quelle und jede erkannte Angabe '
                'darin.',
              ),
              const SizedBox(height: 16),
              _Meaning(
                icon: Icons.check_circle_outline,
                color: theme.colorScheme.primary,
                title: foundLabel,
                text: 'Der Wert steht so im Original.',
              ),
              _Meaning(
                icon: Icons.search_off,
                color: theme.colorScheme.onSurfaceVariant,
                title: notFoundLabel,
                text:
                    'Der Wert ließ sich im Original nicht wiederfinden. Das '
                    'heißt nicht, dass er falsch ist, zum Beispiel bei einem '
                    'unscharfen Foto oder einer anders gelesenen Einheit. '
                    'Vergleiche ihn mit dem Original und korrigiere ihn bei '
                    'Bedarf.',
              ),
              const SizedBox(height: 8),
              const Text(
                'Nach einer Korrektur wird der neue Wert im Original gesucht. '
                'Typ und Importdatum werden nicht geprüft.',
              ),
              const SizedBox(height: 8),
              const Text(
                'Die Prüfung sagt nichts darüber aus, ob ein Wert medizinisch '
                'in Ordnung ist.',
              ),
              if (engine != null || checkedAt != null) ...[
                const SizedBox(height: 16),
                Text(
                  [
                    if (engine != null) 'Programm: $engine',
                    if (checkedAt != null)
                      'Geprüft: ${formatLocalTimestamp(checkedAt)}',
                  ].join(' · '),
                  style: small,
                ),
              ],
            ],
          ),
        ),
      );
    },
  );
}

class _Meaning extends StatelessWidget {
  const _Meaning({
    required this.icon,
    required this.color,
    required this.title,
    required this.text,
  });

  final IconData icon;
  final Color color;
  final String title;
  final String text;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 20, color: color),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: theme.textTheme.titleSmall),
                const SizedBox(height: 2),
                Text(text),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
