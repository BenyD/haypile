package ingest

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMbox(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "inbox.mbox")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMboxMessagesBecomeSections(t *testing.T) {
	att := base64.StdEncoding.EncodeToString([]byte("SECRET-ATTACHMENT-BYTES"))
	mbox := "From alice@example.com Mon Jan  5 10:00:00 2026\n" +
		"From: Alice <alice@example.com>\n" +
		"Date: Mon, 5 Jan 2026 10:00:00 +0000\n" +
		"Subject: =?utf-8?q?Renewal_terms_for_the_M=C3=BCller_lease?=\n" +
		"\n" +
		"The notice period is ninety days.\n" +
		">From what I recall, that clause survived.\n" +
		"\n" +
		"From bob@example.com Mon Jan  5 11:00:00 2026\n" +
		"From: Bob <bob@example.com>\n" +
		"Subject: Invoice attached\n" +
		"MIME-Version: 1.0\n" +
		"Content-Type: multipart/mixed; boundary=XYZ\n" +
		"\n" +
		"--XYZ\n" +
		"Content-Type: text/plain\n" +
		"Content-Transfer-Encoding: quoted-printable\n" +
		"\n" +
		"Payment is due net=2D45.\n" +
		"--XYZ\n" +
		"Content-Type: application/pdf; name=invoice.pdf\n" +
		"Content-Disposition: attachment; filename=invoice.pdf\n" +
		"Content-Transfer-Encoding: base64\n" +
		"\n" +
		att + "\n" +
		"--XYZ--\n"

	secs, err := extractMbox(writeMbox(t, mbox))
	if err != nil {
		t.Fatalf("extractMbox: %v", err)
	}
	if len(secs) != 2 {
		t.Fatalf("got %d sections, want 2: %+v", len(secs), secs)
	}

	first, second := secs[0], secs[1]
	if first.Page != 1 || second.Page != 2 {
		t.Fatalf("ordinals %d,%d; want 1,2", first.Page, second.Page)
	}
	// RFC 2047 subject decoded, body present, mboxrd quoting undone.
	for _, want := range []string{"Müller lease", "ninety days", "From what I recall"} {
		if !strings.Contains(first.Text, want) {
			t.Errorf("message 1 missing %q:\n%s", want, first.Text)
		}
	}
	// Quoted-printable decoded, attachment bytes absent.
	if !strings.Contains(second.Text, "net-45") {
		t.Errorf("message 2 quoted-printable not decoded:\n%s", second.Text)
	}
	for _, absent := range []string{"SECRET-ATTACHMENT-BYTES", att} {
		if strings.Contains(second.Text, absent) {
			t.Errorf("message 2 leaked attachment content")
		}
	}
}

func TestMboxHTMLOnlyBodyIsStripped(t *testing.T) {
	mbox := "From carol@example.com Mon Jan  5 12:00:00 2026\n" +
		"From: Carol <carol@example.com>\n" +
		"Subject: Meeting moved\n" +
		"Content-Type: text/html\n" +
		"\n" +
		"<html><head><style>p{color:red}</style></head>" +
		"<body><p>The meeting moved to <b>Thursday</b>.</p></body></html>\n"

	secs, err := extractMbox(writeMbox(t, mbox))
	if err != nil {
		t.Fatalf("extractMbox: %v", err)
	}
	if len(secs) != 1 {
		t.Fatalf("got %d sections, want 1", len(secs))
	}
	if !strings.Contains(secs[0].Text, "meeting moved to Thursday") {
		t.Errorf("html body not stripped to text:\n%s", secs[0].Text)
	}
	if strings.Contains(secs[0].Text, "color:red") {
		t.Errorf("style content leaked:\n%s", secs[0].Text)
	}
}

func TestMboxRejectsNonMbox(t *testing.T) {
	if _, err := extractMbox(writeMbox(t, "just some text\nnot an mbox\n")); err == nil {
		t.Fatal("a non-mbox file must error, not index as noise")
	}
}
