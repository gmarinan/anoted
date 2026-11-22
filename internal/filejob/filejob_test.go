package filejob

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func touch(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestParseDroppedFormats(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX paths")
	}
	dir := t.TempDir()
	spaced := touch(t, filepath.Join(dir, "voice note 1.m4a"))
	plain := touch(t, filepath.Join(dir, "call.mp3"))

	cases := map[string][]string{
		"alacritty bare path with spaces": {spaced + " "},
		"single quoted (gnome, kitty)":    {"'" + spaced + "'"},
		"double quoted":                   {`"` + spaced + `"`},
		"backslash escaped":               {filepath.Join(dir, `voice\ note\ 1.m4a`)},
		"file uri":                        {"file://" + filepath.ToSlash(dir) + "/voice%20note%201.m4a"},
	}
	for name, in := range cases {
		got := ParseDropped(in[0], IsFile)
		if !reflect.DeepEqual(got, []string{spaced}) {
			t.Errorf("%s: got %q", name, got)
		}
	}

	got := ParseDropped("'"+spaced+"' '"+plain+"'", IsFile)
	if !reflect.DeepEqual(got, []string{spaced, plain}) {
		t.Errorf("two quoted files: %q", got)
	}
	got = ParseDropped(plain+"\n"+spaced+"\n"+plain, IsFile)
	if !reflect.DeepEqual(got, []string{plain, spaced}) {
		t.Errorf("one per line, deduplicated: %q", got)
	}
	got = ParseDropped(filepath.Join(dir, "missing.wav"), IsFile)
	if len(got) != 1 || IsFile(got[0]) {
		t.Errorf("a missing file is still reported so the caller can name it: %q", got)
	}
}

func TestCreateListRemove(t *testing.T) {
	src := touch(t, filepath.Join(t.TempDir(), "Entrevista Final!.mp3"))
	root := filepath.Join(t.TempDir(), "files")
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)

	a, err := Create(root, src, "es", now)
	if err != nil {
		t.Fatal(err)
	}
	// Same name, same second: must not share a folder.
	b, err := Create(root, src, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if a.Dir == b.Dir {
		t.Fatal("two jobs share a folder")
	}
	if filepath.Base(a.Dir) != "20261122-100000-entrevista-final" {
		t.Fatalf("folder name %q", filepath.Base(a.Dir))
	}

	later, err := Create(root, src, "en", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(later.TranscriptPath(), []byte("hola mundo\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	jobs, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 || jobs[0].Dir != later.Dir || !jobs[0].Done || jobs[1].Done {
		t.Fatalf("list = %+v", jobs)
	}
	if jobs[0].Source != src || jobs[0].Language != "en" || jobs[0].Name != "Entrevista Final!.mp3" {
		t.Fatalf("metadata not round-tripped: %+v", jobs[0])
	}
	if text, err := ReadTranscript(later.Dir); err != nil || text != "hola mundo" {
		t.Fatalf("transcript %q, %v", text, err)
	}

	if err := Remove(root, later.Dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatal("removing a job deleted the source file")
	}
	if jobs, _ := List(root); len(jobs) != 2 {
		t.Fatalf("after remove: %d jobs", len(jobs))
	}
}

func TestRemoveRefusesAnythingButAJob(t *testing.T) {
	root := t.TempDir()
	stray := filepath.Join(root, "not-a-job")
	touch(t, filepath.Join(stray, "keep.txt"))

	for _, dir := range []string{"", root, filepath.Dir(root), stray} {
		if err := Remove(root, dir); err == nil {
			t.Errorf("Remove(%q) succeeded", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(stray, "keep.txt")); err != nil {
		t.Fatal("a refused remove deleted files")
	}
}

func TestListMissingRootIsEmpty(t *testing.T) {
	jobs, err := List(filepath.Join(t.TempDir(), "nope"))
	if err != nil || len(jobs) != 0 {
		t.Fatalf("got %v, %v", jobs, err)
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Reunión con Ana.m4a": "reuni-n-con-ana",
		"___.wav":             "audio",
		"WhatsApp Audio 2026-11-01 at 10.15.32.opus": "whatsapp-audio-2026-11-01-at-10-15-32",
	} {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLooksLikeMedia(t *testing.T) {
	if !LooksLikeMedia("/a/B.MP3") || !LooksLikeMedia("x.webm") || LooksLikeMedia("notes.txt") || LooksLikeMedia("Makefile") {
		t.Fatal("extension check wrong")
	}
}
