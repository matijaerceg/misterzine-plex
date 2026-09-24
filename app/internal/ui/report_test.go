package ui

import "testing"

func TestParseReportOutput(t *testing.T) {
	r := parseReportOutput([]byte("saved: /media/fat/misterzine-plex/report.txt\ncode: K7M4\n"))
	if r.saved != "/media/fat/misterzine-plex/report.txt" || r.code != "K7M4" || r.problem != "" {
		t.Fatalf("sent: %+v", r)
	}
	r = parseReportOutput([]byte("saved: /media/fat/misterzine-plex/report.txt\nerror: The MiSTer seems to be offline.\n"))
	if r.code != "" || r.problem != "The MiSTer seems to be offline." {
		t.Fatalf("failed upload: %+v", r)
	}
	r = parseReportOutput([]byte("Traceback (most recent call last)\n"))
	if r.problem == "" || r.code != "" {
		t.Fatalf("crash: %+v", r)
	}
	if got := cardRelative("/media/fat/misterzine-plex/report.txt"); got != "misterzine-plex/report.txt" {
		t.Fatalf("cardRelative: %q", got)
	}
}
