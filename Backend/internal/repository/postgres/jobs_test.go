package postgres

import (
	"encoding/json"
	"testing"

	"beeba.org/internal/domain/jobs"
)

func TestRedactAdminJobHidesEmailQueueTokens(t *testing.T) {
	inputPayload := json.RawMessage(`{"user_id":"u1","token_id":"t1","token":"bb_pc_secret","template":"password_change_confirmation"}`)
	job := redactAdminJob(jobs.AdminJob{
		QueueName: "email_queue",
		JobType:   "send_password_change_confirmation",
		Payload:   inputPayload,
	})

	var payload map[string]string
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		t.Fatalf("decode redacted payload: %v", err)
	}
	if payload["token"] != "[redacted]" {
		t.Fatalf("token = %q, want redacted", payload["token"])
	}
}
