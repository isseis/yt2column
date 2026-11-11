// Package ytdlpsource implements transcript.TranscriptSource by invoking the
// external yt-dlp command. It fetches subtitles and info.json, parses the json3
// subtitle format, and keeps a per-video cache. It is the development path of
// the CLI; the copied-transcript path lives in package transcript and does not
// depend on this package.
package ytdlpsource
