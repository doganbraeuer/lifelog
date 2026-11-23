package gemini

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"lifelog/backend/internal/core"
)

// readPromptSHA256 is the SHA-256 of the read instruction and schema for
// photos and PDFs. The read prompt determines the full text of uploaded files
// and with it their chunks, vectors and the calibration of the threshold
// (docs/API.md, section 4, "Recalibrate the threshold"). A change must be
// deliberate: run cmd/calibrate again, then update this value.
const readPromptSHA256 = "f79895809d5bd1fd333c4bbb41cc41e6bc76d485d58efb86350b4edb30a9c2df"

func TestReadPrompt_Unchanged(t *testing.T) {
	types := core.AllowedTypes("health")
	schema, err := json.Marshal(readSchema(types))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append([]byte(readInstruction(types)+"\n"), schema...))
	if got := hex.EncodeToString(sum[:]); got != readPromptSHA256 {
		t.Errorf("the read prompt for files changed (SHA-256 %s).\n"+
			"Run cmd/calibrate again before changing it, then update readPromptSHA256.", got)
	}
}
