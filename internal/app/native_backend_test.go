package app

import (
	"strings"
	"testing"
)

func TestNativePageWindowUsesRemoteTotalForDone(t *testing.T) {
	start, end, done := nativePageWindow(120, 1, 50)
	if start != 0 || end != 50 || done {
		t.Fatalf("page 1 = start %d end %d done %v, want 0 50 false", start, end, done)
	}

	start, end, done = nativePageWindow(120, 3, 50)
	if start != 100 || end != 120 || !done {
		t.Fatalf("page 3 = start %d end %d done %v, want 100 120 true", start, end, done)
	}

	start, end, done = nativePageWindow(120, 4, 50)
	if start != 0 || end != 0 || !done {
		t.Fatalf("page beyond total = start %d end %d done %v, want 0 0 true", start, end, done)
	}
}

func TestNativeEnvelopeSeqRangeUsesRemoteTotalForDone(t *testing.T) {
	from, to, done, ok := nativeEnvelopeSeqRange(120, 1, 50)
	if from != 71 || to != 120 || done || !ok {
		t.Fatalf("page 1 = from %d to %d done %v ok %v, want 71 120 false true", from, to, done, ok)
	}

	from, to, done, ok = nativeEnvelopeSeqRange(120, 3, 50)
	if from != 1 || to != 20 || !done || !ok {
		t.Fatalf("page 3 = from %d to %d done %v ok %v, want 1 20 true true", from, to, done, ok)
	}

	_, _, done, ok = nativeEnvelopeSeqRange(120, 4, 50)
	if !done || ok {
		t.Fatalf("page beyond total = done %v ok %v, want true false", done, ok)
	}
}

func TestExtractReadableMessageContentIncludesImages(t *testing.T) {
	raw := strings.Join([]string{
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="clibox-test"`,
		"",
		"--clibox-test",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello with image.",
		"--clibox-test",
		"Content-Type: image/png",
		`Content-Disposition: inline; filename="pixel.png"`,
		"Content-Transfer-Encoding: base64",
		"",
		"aGVsbG8=",
		"--clibox-test--",
		"",
	}, "\r\n")

	content := extractReadableMessageContent([]byte(raw))
	if content.Body != "Hello with image." {
		t.Fatalf("unexpected body: %q", content.Body)
	}
	if len(content.Images) != 1 {
		t.Fatalf("expected one image, got %+v", content.Images)
	}
	image := content.Images[0]
	if image.Name != "pixel.png" || image.ContentType != "image/png" || string(image.Data) != "hello" {
		t.Fatalf("unexpected image: %+v", image)
	}
}
