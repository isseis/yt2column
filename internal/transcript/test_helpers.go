//go:build test

package transcript

// Paths to the committed testdata fixtures, relative to this package directory.
const (
	testdataRealSubtitles        = "../../testdata/2tcCWM-sRBw.ja.json3"
	testdataRealInfo             = "../../testdata/2tcCWM-sRBw.info.json"
	testdataInvalidUTF8Subtitles = "../../testdata/invalid_utf8.json3"
	testdataSurrogateSubtitles   = "../../testdata/unpaired_surrogate.json3"
	testdataInvalidUTF8Info      = "../../testdata/invalid_utf8.info.json"
	testdataSurrogateInfo        = "../../testdata/unpaired_surrogate.info.json"
	testdataRealVideoID          = "2tcCWM-sRBw"
)
