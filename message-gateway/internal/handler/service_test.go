package handler

import "testing"

func TestStreamingMessageUUIDIsShort(t *testing.T) {
	uuid := streamingMessageUUID("53ca512e414582ca531e910872b9cb3c")
	if len(uuid) > 32 {
		t.Fatalf("uuid too long: len=%d uuid=%s", len(uuid), uuid)
	}
	if uuid == "" {
		t.Fatalf("uuid should not be empty")
	}
}
