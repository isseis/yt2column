//go:build test

package transcript

import (
	"errors"
	"testing"
)

func TestValidateVideoURL(t *testing.T) {
	const id = "dQw4w9WgXcQ"

	accepted := []struct {
		name   string
		url    string
		wantID string
	}{
		{"https watch", "https://www.youtube.com/watch?v=" + id, id},
		{"http watch", "http://www.youtube.com/watch?v=" + id, id},
		{"bare youtube host", "https://youtube.com/watch?v=" + id, id},
		{"mobile host", "https://m.youtube.com/watch?v=" + id, id},
		{"youtu.be", "https://youtu.be/" + id, id},
		{"embed", "https://www.youtube.com/embed/" + id, id},
		{"shorts", "https://www.youtube.com/shorts/" + id, id},
		{"live", "https://www.youtube.com/live/" + id, id},
		{"extra query params", "https://www.youtube.com/watch?v=" + id + "&t=30&list=PLxyz", id},
		{"fragment", "https://www.youtube.com/watch?v=" + id + "#t=30", id},
		{"path-like query value", "https://www.youtube.com/watch?v=" + id + "&list=../../../etc", id},
		{"hyphen and underscore id", "https://youtu.be/ab-CD_ef-12", "ab-CD_ef-12"},
	}
	for _, tc := range accepted {
		t.Run("accept/"+tc.name, func(t *testing.T) {
			gotID, gotURL, err := ValidateVideoURL(tc.url)
			if err != nil {
				t.Fatalf("ValidateVideoURL(%q) error = %v", tc.url, err)
			}
			if gotID != tc.wantID {
				t.Errorf("video ID = %q, want %q", gotID, tc.wantID)
			}
			wantURL := "https://www.youtube.com/watch?v=" + tc.wantID
			if gotURL != wantURL {
				t.Errorf("normalized URL = %q, want %q", gotURL, wantURL)
			}
		})
	}

	rejected := []struct {
		name string
		url  string
	}{
		{"id too short", "https://www.youtube.com/watch?v=short"},
		{"id too long", "https://www.youtube.com/watch?v=" + id + "x"},
		{"id invalid character", "https://www.youtube.com/watch?v=dQw4w9WgXc!"},
		{"youtu.be id too short", "https://youtu.be/dQw4w9WgX"},
		{"ftp scheme", "ftp://youtube.com/watch?v=" + id},
		{"file scheme", "file://" + id},
		{"missing scheme", "youtube.com/watch?v=" + id},
		{"other host", "https://vimeo.com/watch?v=" + id},
		{"host suffix", "https://youtube.com.evil.example/watch?v=" + id},
		{"host prefix", "https://evilyoutube.com/watch?v=" + id},
		{"no path", "https://www.youtube.com/"},
		{"watch without v", "https://www.youtube.com/watch"},
		{"watch with other query", "https://www.youtube.com/watch?foo=bar"},
		{"channel path", "https://www.youtube.com/channel/" + id},
		{"multiple v", "https://www.youtube.com/watch?v=" + id + "&v=abcdefghijk"},
		{"multiple v hidden by bad escape", "https://www.youtube.com/watch?v=" + id + "&v=%zz"},
		{"multiple v hidden by semicolon", "https://www.youtube.com/watch?v=" + id + "&v=abcdefghijk;"},
		{"bare id", id},
		{"leading space", " https://www.youtube.com/watch?v=" + id},
		{"trailing space", "https://www.youtube.com/watch?v=" + id + " "},
		{"trailing newline", "https://www.youtube.com/watch?v=" + id + "\n"},
		{"trailing space after fragment", "https://www.youtube.com/watch?v=" + id + "#t=30 "},
		{"trailing tab after fragment", "https://www.youtube.com/watch?v=" + id + "#t=30\t"},
		{"youtu.be extra path", "https://youtu.be/" + id + "/../../etc"},
		{"shorts extra path", "https://www.youtube.com/shorts/" + id + "/extra"},
		{"embed extra path", "https://www.youtube.com/embed/" + id + "/extra"},
		{"live extra path", "https://www.youtube.com/live/" + id + "/extra"},
		{"trailing slash", "https://youtu.be/" + id + "/"},
		{"userinfo", "https://user:pass@www.youtube.com/watch?v=" + id},
	}
	for _, tc := range rejected {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			_, _, err := ValidateVideoURL(tc.url)
			if !errors.Is(err, ErrInvalidVideoURL) {
				t.Fatalf("ValidateVideoURL(%q) error = %v, want ErrInvalidVideoURL", tc.url, err)
			}
			for _, other := range []error{ErrYtDlpExec, ErrParseSubtitles, ErrParseInfo, ErrNoSubtitles} {
				if errors.Is(err, other) {
					t.Errorf("ValidateVideoURL(%q) error %v also matches %v", tc.url, err, other)
				}
			}
		})
	}
}

func TestNormalizedVideoURL(t *testing.T) {
	accepted := []string{"dQw4w9WgXcQ", "ab-CD_ef-12", "AAAAAAAAAAA", "00000000000"}
	for _, id := range accepted {
		t.Run("accept/"+id, func(t *testing.T) {
			got, ok := NormalizedVideoURL(id)
			if !ok {
				t.Fatalf("NormalizedVideoURL(%q) ok = false, want true", id)
			}
			if want := "https://www.youtube.com/watch?v=" + id; got != want {
				t.Errorf("NormalizedVideoURL(%q) = %q, want %q", id, got, want)
			}
		})
	}

	rejected := []struct {
		name string
		id   string
	}{
		{"empty", ""},
		{"10 characters", "dQw4w9WgXc"},
		{"12 characters", "dQw4w9WgXcQx"},
		{"slash", "dQw4w9/gXcQ"},
		{"dot dot", "dQw4w9..XcQ"},
		{"disallowed symbol", "dQw4w9WgXc!"},
		{"query separator", "dQw4w9WgX&Q"},
		{"non-ASCII", "dQw4w9WgXcé"},
		{"trailing newline", "dQw4w9WgXcQ\n"},
		{"leading space", " dQw4w9WgXc"},
	}
	for _, tc := range rejected {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			got, ok := NormalizedVideoURL(tc.id)
			if ok {
				t.Fatalf("NormalizedVideoURL(%q) ok = true, want false", tc.id)
			}
			if got != "" {
				t.Errorf("NormalizedVideoURL(%q) = %q, want empty", tc.id, got)
			}
		})
	}
}
