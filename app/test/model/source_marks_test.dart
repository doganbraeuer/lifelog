import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/answer.dart';
import 'package:lifelog/model/excerpt_location.dart';
import 'package:lifelog/model/source_marks.dart';

/// Synthetic lab report as a short document: one chunk is the whole text.
const ferritinReport =
    'Labor Dr. Muster, Salzburg\n'
    'Laborbefund vom 14.10.2026\n'
    'Patientin: Erika Beispiel, geb. 03.04.1985\n'
    '\n'
    'Hämoglobin 13,1 g/dl (Ref. 12,0-16,0)\n'
    'Ferritin 84 ng/ml (Ref. 15-150)\n'
    'Eisen 92 µg/dl (Ref. 60-180)\n'
    'Transferrinsättigung 24 % (Ref. 16-45)\n'
    'CRP 0,3 mg/dl (Ref. <0,5)\n'
    '\n'
    'Befund vidiert: Dr. Max Muster';

/// The texts [markPassage] marks for source [number]. The passage is
/// [excerpt], or the whole text (a short document is one chunk).
List<String> marked(
  String fullText,
  String answer, {
  int number = 1,
  String question = '',
  String? excerpt,
}) {
  final passage = locateExcerpt(fullText, excerpt ?? fullText)!;
  return [
    for (final location in markPassage(
      fullText: fullText,
      passage: passage,
      answer: answer,
      number: number,
      question: question,
    ))
      location.textIn(fullText),
  ];
}

Source source(int number, String excerpt) => Source(
  number: number,
  entryId: '6727a1f3c2b4e5d6f7a8b9c0',
  module: 'health',
  type: 'lab_report',
  issuer: 'Labor Dr. Muster, Salzburg',
  documentDate: null,
  excerpt: excerpt,
);

void main() {
  group('markPassage', () {
    test('a lab report: only the line of the value in the answer', () {
      expect(marked(ferritinReport, 'Ferritin lag bei 84 ng/ml [1]'), [
        'Ferritin 84 ng/ml (Ref. 15-150)',
      ]);
    });

    test('a date in the answer does not mark the header with the date', () {
      expect(
        marked(ferritinReport, 'Am 14.10.2026 lag Ferritin bei 84 ng/ml [1]'),
        ['Ferritin 84 ng/ml (Ref. 15-150)'],
      );
    });

    test('a date alone marks only if the anchor has nothing else', () {
      // No other number and no rare word: the date is all there is.
      expect(marked(ferritinReport, 'Der Befund ist vom 14.10.2026 [1].'), [
        'Laborbefund vom 14.10.2026',
      ]);
      // With a rare word the date no longer qualifies a line; one word
      // alone does not either.
      expect(
        marked(ferritinReport, 'Ferritin wurde am 14.10.2026 bestimmt [1].'),
        isEmpty,
      );
    });

    test('the same unit on every line counts hardly', () {
      const lipids =
          'Laborbefund vom 02.09.2026\n'
          'Cholesterin gesamt 212 mg/dl\n'
          'HDL-Cholesterin 58 mg/dl\n'
          'LDL-Cholesterin 131 mg/dl\n'
          'Triglyceride 115 mg/dl\n'
          'Glukose nüchtern 94 mg/dl';
      expect(
        marked(lipids, 'Das Cholesterin gesamt lag bei 212 mg/dl [1].'),
        ['Cholesterin gesamt 212 mg/dl'],
      );
      // Only the unit: no line qualifies.
      expect(marked(lipids, 'Alle Werte sind in mg/dl angegeben [1].'), []);
    });

    test('a short document that is one chunk is never marked as a whole', () {
      const prescription =
          'Rezept\n'
          'Ibuprofen 400 mg\n'
          '1-1-1 nach dem Essen\n'
          'Dr. Erika Beispiel';
      expect(marked(prescription, 'Verordnet ist Ibuprofen 400 mg [1].'), [
        'Ibuprofen 400 mg',
      ]);
      // The answer repeats nearly everything: the marks would cover more
      // than 35 % of the text, so nothing is marked.
      expect(
        marked(
          prescription,
          'Das Rezept über Ibuprofen 400 mg, 1-1-1 nach dem Essen, stammt '
          'von Dr. Erika Beispiel [1].',
        ),
        isEmpty,
      );
    });

    test('an unspecific question with a general answer marks nothing', () {
      expect(
        marked(
          ferritinReport,
          'In deinen Befunden stehen Laborwerte vom Oktober und ein '
          'Arztbrief [1].',
          question: 'Was steht in meinen Befunden?',
        ),
        isEmpty,
      );
    });

    test('an answer without a reference marks nothing', () {
      expect(marked(ferritinReport, 'Dazu habe ich nichts gefunden.'), []);
      expect(
        marked(ferritinReport, 'Dazu finde ich nichts in deinen Dokumenten.'),
        isEmpty,
      );
    });

    test('decimal comma in the answer, point in the text, and back', () {
      const withPoint =
          'Blutbild vom 14.10.2026\n'
          'Hämoglobin 13.1 g/dL\n'
          'Leukozyten 6.8 G/l\n'
          'Thrombozyten 245 G/l';
      expect(marked(withPoint, 'Hämoglobin lag bei 13,1 g/dl [1].'), [
        'Hämoglobin 13.1 g/dL',
      ]);
      expect(marked(ferritinReport, 'CRP lag bei 0.3 mg/dl [1].'), [
        'CRP 0,3 mg/dl (Ref. <0,5)',
      ]);
    });

    test('umlauts and ß are tolerant, 130/85 is one number', () {
      expect(marked(ferritinReport, 'Haemoglobin 13,1 [1]'), [
        'Hämoglobin 13,1 g/dl (Ref. 12,0-16,0)',
      ]);
      const pressure =
          'Blutdruckprotokoll\n'
          'Montag 130/85 mmHg\n'
          'Dienstag 128/80 mmHg\n'
          'Mittwoch 135/88 mmHg\n'
          'Donnerstag 126/79 mmHg';
      expect(marked(pressure, 'Der Blutdruck lag bei 130/85 mmHg [1].'), [
        'Montag 130/85 mmHg',
      ]);
      // 130 alone is not 130/85.
      expect(marked(pressure, 'Der obere Wert lag bei 130 [1].'), isEmpty);
    });

    test('a note in running text: at most one sentence', () {
      const note =
          'Rezept Marmorkuchen\n'
          'Notiz zum Kuchen: Den Ofen auf 180 °C vorheizen. Butter und '
          'Zucker schaumig rühren, dann die Eier einzeln unterrühren. Mehl '
          'mit Backpulver mischen und unterheben. Den Teig in die Form '
          'füllen und 45 Minuten backen. Vor dem Anschneiden mindestens eine '
          'Stunde auskühlen lassen.';
      expect(marked(note, 'Der Kuchen muss 45 Minuten backen [1].'), [
        'Den Teig in die Form füllen und 45 Minuten backen.',
      ]);
    });

    test('the number of the reference itself counts as no number', () {
      // Without the reference the anchor has nothing; [1] is no „1“.
      const text =
          'Laborbefund\nNatrium 1 mmol/l\nKalium 4,1 mmol/l\nChlorid 102 '
          'mmol/l\nCalcium 2,3 mmol/l';
      expect(marked(text, 'Siehe Befund [1].'), isEmpty);
    });

    test('question terms add to the score but qualify no line', () {
      expect(
        marked(
          ferritinReport,
          'Ferritin lag bei 84 ng/ml [1].',
          question: 'Wie hoch waren Ferritin und Eisen?',
        ),
        ['Ferritin 84 ng/ml (Ref. 15-150)'],
      );
    });

    test('at most three lines, in text order', () {
      const electrolytes =
          'Labor Dr. Muster, Salzburg\n'
          'Laborbefund vom 14.10.2026, Elektrolyte im Serum\n'
          'Natrium 140 mmol/l\n'
          'Kalium 4,1 mmol/l\n'
          'Chlorid 102 mmol/l\n'
          'Calcium 2,3 mmol/l\n'
          'Probe hämolysefrei, Abnahme nüchtern um 8:15 Uhr\n'
          'Befund vidiert: Dr. Max Muster';
      expect(
        marked(
          electrolytes,
          'Natrium 140, Kalium 4,1, Chlorid 102 und Calcium 2,3 mmol/l [1].',
        ),
        ['Natrium 140 mmol/l', 'Kalium 4,1 mmol/l', 'Chlorid 102 mmol/l'],
      );
    });

    test('a candidate below 60 % of the best stays unmarked', () {
      // 140 is on two lines, so alone it weighs only half.
      const text =
          'Labor Dr. Muster, Salzburg\n'
          'Laborbefund vom 14.10.2026, Blutbild und Elektrolyte\n'
          'Natrium 140 mmol/l\n'
          'Thrombozyten 140 G/l\n'
          'Kalium 4,1 mmol/l\n'
          'Befund vidiert: Dr. Max Muster';
      expect(marked(text, 'Natrium lag bei 140 mmol/l [1].'), [
        'Natrium 140 mmol/l',
      ]);
    });

    test('only lines inside the passage, by the sentences citing it', () {
      const answer = 'Ferritin lag bei 84 ng/ml [1]. Eisen lag bei 92 µg/dl [2].';
      expect(marked(ferritinReport, answer, number: 2), [
        'Eisen 92 µg/dl (Ref. 60-180)',
      ]);
      // A passage without the Ferritin line marks nothing for source 1.
      expect(
        marked(
          ferritinReport,
          answer,
          excerpt: 'Eisen 92 µg/dl (Ref. 60-180)\nTransferrinsättigung 24 %',
        ),
        isEmpty,
      );
      // [1, 3]: the sentence belongs to both sources.
      expect(
        marked(
          ferritinReport,
          'Ferritin lag bei 84 ng/ml und Eisen bei 92 µg/dl [1, 3].',
          number: 3,
        ),
        ['Ferritin 84 ng/ml (Ref. 15-150)', 'Eisen 92 µg/dl (Ref. 60-180)'],
      );
    });
  });

  group('markSources', () {
    const answer = 'Ferritin lag bei 84 ng/ml [1]. Eisen lag bei 92 µg/dl [2].';
    final one = source(1, ferritinReport);
    final two = source(2, ferritinReport);

    test('own lines strong, those of the other source light', () {
      final marks = markSources(
        fullText: ferritinReport,
        source: one,
        sameEntry: [one, two],
        answer: answer,
        question: 'Wie waren meine Eisenwerte?',
      )!;
      expect(marks.current.map((l) => l.textIn(ferritinReport)), [
        'Ferritin 84 ng/ml (Ref. 15-150)',
      ]);
      expect(marks.others.map((l) => l.textIn(ferritinReport)), [
        'Eisen 92 µg/dl (Ref. 60-180)',
      ]);
      expect(identical(marks.scrollTo, marks.current.first), isTrue);
    });

    test('nothing marked: scroll to the start of the passage', () {
      final excerpt = 'Hämoglobin 13,1 g/dl (Ref. 12,0-16,0)\nFerritin 84 ng/ml';
      final marks = markSources(
        fullText: ferritinReport,
        source: source(1, excerpt),
        sameEntry: [source(1, excerpt)],
        answer: 'In deinen Befunden stehen Laborwerte [1].',
        question: 'Was steht in meinen Befunden?',
      )!;
      expect(marks.current, isEmpty);
      expect(marks.others, isEmpty);
      expect(marks.scrollTo.textIn(ferritinReport), excerpt);
    });

    test('a light mark that overlaps an own one is left out', () {
      final marks = markSources(
        fullText: ferritinReport,
        source: one,
        sameEntry: [one, two],
        answer: 'Ferritin lag bei 84 ng/ml [1, 2].',
        question: '',
      )!;
      expect(marks.current, hasLength(1));
      expect(marks.others, isEmpty);
    });

    test('light marks only while all marks stay below 35 %', () {
      const short =
          'Rezept\nIbuprofen 400 mg\n1-1-1 nach dem Essen\n'
          'Dr. Erika Beispiel, Allgemeinmedizin, Salzburg';
      final a = source(1, short);
      final b = source(2, short);
      final marks = markSources(
        fullText: short,
        source: a,
        sameEntry: [a, b],
        answer: 'Ibuprofen 400 mg [1]. Einnahme 1-1-1 nach dem Essen [2].',
        question: '',
      )!;
      // Each alone stays below 35 % of the text, both together would not.
      expect(marks.current.map((l) => l.textIn(short)), ['Ibuprofen 400 mg']);
      expect(marks.others, isEmpty);
    });

    test('passage not found: null', () {
      expect(
        markSources(
          fullText: ferritinReport,
          source: source(1, 'Ganz anderer Text'),
          sameEntry: const [],
          answer: answer,
          question: '',
        ),
        isNull,
      );
    });
  });

  group('splitSentences', () {
    List<String> sentences(String text) => [
      for (final (start, end) in splitSentences(text))
        text.substring(start, end),
    ];

    test('after punctuation, citations stay with their sentence', () {
      expect(
        sentences(
          'Ferritin lag bei 84 ng/ml [1]. Eisen war 92 µg/dl. [2] '
          'Am 14. Oktober 2026 kam Dr. Muster, z. B. mit 2.1 mg!\n'
          'Neue Zeile',
        ),
        [
          'Ferritin lag bei 84 ng/ml [1].',
          'Eisen war 92 µg/dl. [2]',
          'Am 14. Oktober 2026 kam Dr. Muster, z. B. mit 2.1 mg!',
          'Neue Zeile',
        ],
      );
    });

    test('dates and a lowercase word after a dot', () {
      expect(sentences('Am 14.10.2026 ca. 5 mg. danach weniger.'), [
        'Am 14.10.2026 ca. 5 mg. danach weniger.',
      ]);
      expect(sentences('  '), isEmpty);
    });
  });

  group('anchorText', () {
    test('only the sentences citing the source, without references', () {
      const answer = 'Ferritin 84 ng/ml [1]. Eisen 92 µg/dl [2]. Beides [1, 2].';
      expect(findTerms(anchorText(answer, 1)), {
        const Term(TermKind.word, 'ferritin'),
        const Term(TermKind.number, '84'),
        const Term(TermKind.unit, 'ng/ml'),
        const Term(TermKind.word, 'beides'),
      });
      expect(anchorText(answer, 3), '');
      expect(anchorText('Dazu habe ich nichts gefunden.', 1), '');
    });
  });

  group('findTerms', () {
    test('numbers, dates, units and words, normalized', () {
      expect(
        findTerms(
          'Hämoglobin 14,2 g/dl, HbA1c 5,9 %, RR 130/85 mmHg am '
          '2. September 2026 bzw. 2026-09-02, Straße',
        ),
        {
          const Term(TermKind.word, 'haemoglobin'),
          const Term(TermKind.number, '14.2'),
          const Term(TermKind.unit, 'g/dl'),
          const Term(TermKind.word, 'hba1c'),
          const Term(TermKind.number, '5.9'),
          const Term(TermKind.unit, '%'),
          const Term(TermKind.number, '130/85'),
          const Term(TermKind.unit, 'mmhg'),
          const Term(TermKind.date, '2.9.2026'),
          const Term(TermKind.word, 'strasse'),
        },
      );
    });

    test('equal spellings give equal terms', () {
      expect(findTerms('02.09.2026'), {const Term(TermKind.date, '2.9.2026')});
      expect(findTerms('2. Jänner 2027'), {
        const Term(TermKind.date, '2.1.2027'),
      });
      expect(findTerms('5 Prozent'), findTerms('5 %'));
      expect(findTerms('12 umol/l'), findTerms('12 µmol/l'));
      expect(findTerms('TSH 2,1 mU/l'), findTerms('tsh 2.1 MU/L'));
      expect(findTerms('Puls 72/min'), {
        const Term(TermKind.word, 'puls'),
        const Term(TermKind.number, '72'),
        const Term(TermKind.unit, '/min'),
      });
    });

    test('filler, general and short words are ignored', () {
      expect(
        findTerms('Der Befund, die Werte und das Datum des Patienten: ok'),
        isEmpty,
      );
      // B12 is a word, not the number 12.
      expect(findTerms('Vitamin B12'), {
        const Term(TermKind.word, 'vitamin'),
        const Term(TermKind.word, 'b12'),
      });
    });
  });

  group('splitSegments', () {
    test('lines, trimmed; a long line in sentences', () {
      final long = '${'Wort ' * 40}Ende. Zweiter Satz hier.';
      final text = '  Kopf  \n\n$long\nKurz. Nicht geteilt.';
      expect(
        [for (final (s, e) in splitSegments(text)) text.substring(s, e)],
        [
          'Kopf',
          '${'Wort ' * 40}Ende.',
          'Zweiter Satz hier.',
          'Kurz. Nicht geteilt.',
        ],
      );
    });
  });
}
