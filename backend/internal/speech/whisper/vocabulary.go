package whisper

import "strings"

// Vocabulary is sent to whisper-server as "prompt" with every recording:
// typical words and spellings from German doctor's letters, lab reports
// and prescriptions. Whisper treats the prompt as text that came before
// the recording, so it prefers these spellings ("1-0-1" instead of "101",
// "mU/l", "Gonarthrose") when it hears something close.
//
// It is German on purpose, unlike our other prompts (CLAUDE.md): it is not
// an instruction but sample text in the language of the recording; an
// English prompt pulls Whisper towards English.
//
// Rules for editing (docs/speech/README.md):
//   - General terms of the domain only, never words taken from the test
//     sentences in demo/speech (the measurement would only show that
//     Whisper finds the list again). cmd/speechcheck reports which terms
//     happen to be in the test sentences.
//   - No example values: a number in the prompt could appear in a
//     transcript although nobody said it. The only digits are dose schemes
//     and names like HbA1c (a test checks this).
//   - Whisper uses at most 224 prompt tokens and drops the beginning of a
//     longer prompt. This list is about 205 tokens (counted with the
//     model's vocabulary, greedy, so rather too many); a test limits its
//     length.
//   - It only works without "--max-context 0" on the server; the start
//     scripts use "--carry-initial-prompt" instead, so it is used for every
//     30-second window of a long recording.
const Vocabulary = "Blutdruck, Puls, mmHg. " +
	"Hämoglobin, Kreatinin, HbA1c, Glukose, Cholesterin, LDL, HDL, Triglyceride, TSH, CRP, Ferritin, Vitamin D. " +
	"mg/dl, mmol/l, g/dl, ng/ml, mU/l. " +
	"Ibuprofen, Paracetamol, Pantoprazol, Ramipril, Metformin, L-Thyroxin. " +
	"1-0-1, 1-0-0, 0-0-1, 1-1-1, bei Bedarf. " +
	"Hypertonie, Diabetes mellitus, Hypothyreose, Arthrose, Gonarthrose, Bandscheibenvorfall, Erguss, Fraktur. " +
	"Ultraschall, Röntgen, MRT, EKG, Physiotherapie, FSME."

// VocabularyTerms splits Vocabulary into its terms ("Vitamin D", "1-0-1").
func VocabularyTerms() []string {
	var out []string
	for _, group := range strings.Split(Vocabulary, ". ") {
		for _, t := range strings.Split(strings.TrimSuffix(group, "."), ", ") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}
