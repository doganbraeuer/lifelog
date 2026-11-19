package config

import (
	"strings"
	"testing"
)

func TestLoad_LimitDefaults(t *testing.T) {
	c, err := Load(env(complete()))
	if err != nil {
		t.Fatal(err)
	}
	if c.LimitUploadsPerDay != 30 || c.LimitQuestionsPerDay != 60 || c.LimitTranscriptionsPerDay != 30 {
		t.Errorf("limits = %d/%d/%d, want 30/60/30", c.LimitUploadsPerDay, c.LimitQuestionsPerDay, c.LimitTranscriptionsPerDay)
	}
}

func TestLoad_LimitsOverride(t *testing.T) {
	e := complete()
	e["LIMIT_UPLOADS_PER_DAY"] = " 5 "
	e["LIMIT_QUESTIONS_PER_DAY"] = "100"
	e["LIMIT_TRANSCRIPTIONS_PER_DAY"] = "1"
	c, err := Load(env(e))
	if err != nil {
		t.Fatal(err)
	}
	if c.LimitUploadsPerDay != 5 || c.LimitQuestionsPerDay != 100 || c.LimitTranscriptionsPerDay != 1 {
		t.Errorf("limits = %d/%d/%d", c.LimitUploadsPerDay, c.LimitQuestionsPerDay, c.LimitTranscriptionsPerDay)
	}
}

func TestLoad_LimitsInvalid(t *testing.T) {
	for _, name := range []string{"LIMIT_UPLOADS_PER_DAY", "LIMIT_QUESTIONS_PER_DAY", "LIMIT_TRANSCRIPTIONS_PER_DAY"} {
		for _, value := range []string{"0", "-3", "zehn", "2.5"} {
			e := complete()
			e[name] = value
			_, err := Load(env(e))
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("%s=%q: err = %v, want an error naming the variable", name, value, err)
			}
		}
	}
}
