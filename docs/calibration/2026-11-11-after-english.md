# Calibration after the English rename

Measured: 2026-11-11 with `go run ./cmd/calibrate` (threshold core.MinScore 0.82),
three runs with identical scores.
Code: branch refactor-english at bef2d29 (English names, codes, MongoDB
names and prompts).
Database: "lifelog" on Atlas, deleted and refilled with `demo/upload.sh`:
4 seed entries and the same 11 files as in the
[baseline](2026-11-11-baseline.md). All 11 uploads were read again by the
vision LLM with the English read prompt (gemini-3.6-flash).

## Comparison with the baseline

- Seed entries (control group, questions 1–10): identical scores.
- 41 of 45 questions: identical scores and the same best hit.
- 4 questions differ by at most 0.002 (the re-read full texts of the uploads
  changed slightly):

  | No. | Group | Baseline | After | Question |
  |---|---|---|---|---|
  | 16 | unrelated | 0.765 | 0.764 | Wie koche ich Spaghetti Carbonara? |
  | 23 | relevant | 0.858 | 0.859 | Warum war ich beim Orthopäden? |
  | 24 | relevant | 0.853 | 0.855 | Was hat das Röntgen meines Knies ergeben? |
  | 39 | near | 0.865 | 0.864 | Wie hoch war mein Vitamin-D-Wert? |

- Ranges: relevant 0.830–0.902, near 0.823–0.864 (was 0.865), unrelated
  0.758–0.808. Separating thresholds still lie in (0.808; 0.830]; 0.82
  stays.
- The read model assigned the same types and document dates as before.
  The full texts stayed German (no translation).

Second layer, checked by hand via `POST /api/v1/questions` (3 relevant,
3 near, 3 unrelated): answers are German with references `[n]` and German
dates; "Allergien" and "Covid-Impfung" get the fixed sentence; unrelated
questions get the fixed sentence without a model call (0.3–0.4 s).
"Vitamin-D-Wert" is answered from the lab report of 2026-10-28 (L1, uploaded
on 2026-11-09), so it is no longer a "near" question.

## Change after this measurement: question 39 is relevant

Decision (Dogan, 2026-11-11): question 39 „Wie hoch war mein Vitamin-D-Wert?“
moves from near to relevant, target `L1_lab_report_2026-10-28.pdf`. The
scores do not change; only the groups do. The measurement below and the
baseline still count it as near, so compare near and relevant with these
counts in mind. Re-evaluated with the same database (embeddings only):

```
  relevant   30 questions, score 0.830 … 0.902, 30/30 sources get through (should: all)
    multiple  6 questions, score 0.850 … 0.866,  6/6 of them with several entries, all get through
  near        7 questions, score 0.823 … 0.861,  7/7 go to the language model (allowed, it answers with the fixed sentence)
  unrelated   8 questions, score 0.758 … 0.808,  0/8 go to the language model (should: none)
Separable: every threshold above 0.808 up to and including 0.830 separates relevant from unrelated.
```

## Measurement (before the change of question 39)

```
45 questions, 15 entries, embedding model gemini-embedding-2, database "lifelog", top 5

 No  Group      Best    Hit                 Right          Question
  1  relevant   0.863   lab_report 10-14    ✓              Welchen Hämoglobinwert hatte ich im Oktober?
  2  relevant   0.857   lab_report 10-14    ✓              Wie hoch waren meine Leukozyten?
  3  relevant   0.858   lab_report 10-14    ✓              Was stand beim CRP?
  4  relevant   0.884   lab_report 10-14    ✓              Welches Labor hat mein Blutbild gemacht?
  5  relevant   0.879   other 09-23         ✗ 0.870        Wie hoch war mein Blutdruck bei der Routinekontrolle?
  6  relevant   0.873   doctor_letter 09-02 ✓              Wann soll ich wieder zur Kontrolle bei Dr. Beispiel?
  7  relevant   0.874   prescription 10-14  ✓              Welches Medikament wurde mir verschrieben?
  8  relevant   0.865   prescription 10-14  ✓              Wie oft am Tag soll ich Beispielin nehmen?
  9  relevant   0.868   prescription 10-14  ✓              Wie viele Tabletten stehen auf dem Rezept?
 10  relevant   0.902   other no date       ✓              Wo liegt mein Impfpass?
 11  relevant   0.868   lab_report 07-22    ✓ 0.866/0.868  Wie hoch war mein Cholesterin?
 12  near       0.856   other no date                      Wann hatte ich meine letzte Tetanus-Impfung?
 13  near       0.823   lab_report 10-14                   Welche Allergien habe ich?
 14  unrelated  0.798   doctor_letter 09-02                Wie wird das Wetter morgen in Salzburg?
 15  unrelated  0.766   other 08-12                        Wer hat die Fußball-WM 2014 gewonnen?
 16  unrelated  0.764   prescription 05-18                 Wie koche ich Spaghetti Carbonara?
 17  unrelated  0.791   lab_report 10-14                   Was kostet eine Zugfahrt von Salzburg nach Wien?
 18  near       0.836   other 08-12                        Wie viele Kilometer bin ich letzte Woche gelaufen?
 19  relevant   0.871   lab_report 01-20    ✓              Wie hoch war mein LDL-Cholesterin im Januar?
 20  relevant   0.842   lab_report 01-20    ✓              Wie hoch war mein Nüchternblutzucker im Januar?
 21  relevant   0.880   lab_report 07-22    ✓              Wie hoch war mein HDL-Cholesterin bei der Kontrolle im Juli?
 22  relevant   0.868   lab_report 07-22    ✓              Wie hoch waren meine Triglyceride im Juli?
 23  relevant   0.859   doctor_letter 03-05 ✓              Warum war ich beim Orthopäden?
 24  relevant   0.855   doctor_letter 03-05 ✓              Was hat das Röntgen meines Knies ergeben?
 25  relevant   0.853   doctor_letter 04-30 ✗ 0.844        Wie lange sollte ich mit dem Laufen pausieren?
 26  relevant   0.887   prescription 03-05  ✓              An welche Physiotherapie-Praxis wurde ich überwiesen?
 27  relevant   0.853   other 08-12         ✗ 0.830        Wie oft pro Woche soll ich zur Physiotherapie?
 28  relevant   0.866   other 06-10         ✓              Wann muss ich die FSME-Impfung auffrischen?
 29  relevant   0.878   other 06-10         ✓              Welcher Impfstoff wurde bei der FSME-Impfung verwendet?
 30  relevant   0.892   other 09-23         ✓              Wie hoch war mein Puls bei der Messung zu Hause?
 31  relevant   0.878   other 08-12         ✓              Wann habe ich die Physiotherapie abgeschlossen?
 32  relevant   0.862   lab_report 07-22    ✓ 0.855/0.862  Wie hat sich mein LDL-Cholesterin verändert?
 33  relevant   0.856   lab_report 07-22    ✓ 0.850/0.856  Wie hat sich mein Gesamtcholesterin zwischen Januar und Juli entwickelt?
 34  relevant   0.861   prescription 03-05  ✓ 0.852/0.861  Warum wurde mir Physiotherapie verordnet und wie oft pro Woche?
 35  relevant   0.887   other 08-12         ✓ 0.861/0.887  Wie lange hat es von der Überweisung bis zum Abschluss der Physiotherapie gedauert?
 36  relevant   0.878   other 09-23         ✓ 0.853/0.878  Wie hat sich mein Blutdruck seit der Routinekontrolle verändert?
 37  near       0.832   doctor_letter 04-30                Wurde bei mir schon ein MRT gemacht?
 38  near       0.837   lab_report 10-14                   Wie hoch war mein HbA1c-Wert?
 39  near       0.864   lab_report 10-28                   Wie hoch war mein Vitamin-D-Wert?
 40  near       0.861   other no date                      Wann hatte ich meine letzte Covid-Impfung?
 41  near       0.831   lab_report 10-14                   Welche Blutgruppe habe ich?
 42  unrelated  0.808   prescription 10-14                 Hast du ein Rezept für Apfelkuchen?
 43  unrelated  0.800   other no date                      Bis wann muss ich meine Steuererklärung abgeben?
 44  unrelated  0.774   other no date                      Was kostet ein Flug nach Rom?
 45  unrelated  0.758   other 09-23                        Wie wechsle ich einen Fahrradschlauch?

Threshold 0.820 (cos 0.640):
  relevant   29 questions, score 0.830 … 0.902, 29/29 sources get through (should: all)
    multiple  6 questions, score 0.850 … 0.866,  6/6 of them with several entries, all get through
  near        8 questions, score 0.823 … 0.864,  8/8 go to the language model (allowed, it answers with the fixed sentence)
  unrelated   8 questions, score 0.758 … 0.808,  0/8 go to the language model (should: none)
  3 relevant questions: best hit from another entry
Separable: every threshold above 0.808 up to and including 0.830 separates relevant from unrelated.
```
