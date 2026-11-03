package coveragecompare

import (
	"strings"

	"github.com/isseis/yt2column/internal/strictjson"
)

// parseRanges reads the range helper's standard output. files are the product
// files the helper was asked about, relative to the repository root; the helper
// names the same files relative to extension/, and the returned entries use the
// repository-relative paths. A value of another shape, a file that is missing,
// repeated, or out of the requested order, a line below 1, or a range that ends
// before it starts stops as unreadable.
func parseRanges(data []byte, point Point, files []string) ([]fileRanges, error) {
	object, err := strictjson.ParseObject(data)
	if err != nil {
		return nil, rangesStop(point, "the output is not one JSON object", data)
	}
	members, err := object.CollectOnly("files")
	if err != nil {
		return nil, rangesStop(point, "a member of the output is not accepted", data)
	}
	value, err := strictjson.Required(members, "files")
	if err != nil {
		return nil, rangesStop(point, "the output has no files member", data)
	}
	entries, err := value.AsArray()
	if err != nil {
		return nil, rangesStop(point, "files is not an array", data)
	}
	if len(entries) != len(files) {
		return nil, rangesStop(point, "the output lists a different number of files than were requested", data)
	}
	answer := make([]fileRanges, len(entries))
	for i, entry := range entries {
		ranges, err := parseFileRanges(entry, point, data)
		if err != nil {
			return nil, err
		}
		if ranges.Path != helperPath(files[i]) {
			return nil, rangesStop(point, "the output does not list the requested files once each in the requested order", data)
		}
		ranges.Path = files[i]
		answer[i] = ranges
	}
	return answer, nil
}

// parseFileRanges reads one entry of the helper's files array. Its path is kept
// as the helper named it until the caller checks it against the requested file.
func parseFileRanges(value strictjson.Value, point Point, data []byte) (fileRanges, error) {
	object, err := value.AsObject()
	if err != nil {
		return fileRanges{}, rangesStop(point, "a file entry is not an object", data)
	}
	members, err := object.CollectOnly("path", "functions", "fieldGroups")
	if err != nil {
		return fileRanges{}, rangesStop(point, "a member of a file entry is not accepted", data)
	}
	file, err := strictjson.RequiredString(members, "path")
	if err != nil {
		return fileRanges{}, rangesStop(point, "path is not a non-empty string", data)
	}
	functions, err := parseLineRanges(members, "functions", point, data)
	if err != nil {
		return fileRanges{}, err
	}
	fieldGroups, err := parseLineRanges(members, "fieldGroups", point, data)
	if err != nil {
		return fileRanges{}, err
	}
	return fileRanges{Path: file, Functions: functions, FieldGroups: fieldGroups}, nil
}

// parseLineRanges reads one array of the file entry: functions or fieldGroups.
func parseLineRanges(members map[string]strictjson.Value, key string, point Point, data []byte) ([]lineRange, error) {
	value, err := strictjson.Required(members, key)
	if err != nil {
		return nil, rangesStop(point, key+" is missing", data)
	}
	elements, err := value.AsArray()
	if err != nil {
		return nil, rangesStop(point, key+" is not an array", data)
	}
	ranges := make([]lineRange, 0, len(elements))
	for _, element := range elements {
		r, err := parseLineRange(element, point, data)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, r)
	}
	return ranges, nil
}

// parseLineRange reads one {start, end} object. Both lines are 1-based and
// inclusive, so start is at least 1 and end is not before start.
func parseLineRange(value strictjson.Value, point Point, data []byte) (lineRange, error) {
	object, err := value.AsObject()
	if err != nil {
		return lineRange{}, rangesStop(point, "a range is not an object", data)
	}
	members, err := object.CollectOnly("start", "end")
	if err != nil {
		return lineRange{}, rangesStop(point, "a member of a range is not accepted", data)
	}
	start, ok := rangeLine(members, "start")
	if !ok {
		return lineRange{}, rangesStop(point, "start is not a line number that fits int", data)
	}
	end, ok := rangeLine(members, "end")
	if !ok {
		return lineRange{}, rangesStop(point, "end is not a line number that fits int", data)
	}
	if start < 1 {
		return lineRange{}, rangesStop(point, "a range starts before line 1", data)
	}
	if end < start {
		return lineRange{}, rangesStop(point, "a range ends before it starts", data)
	}
	return lineRange{Start: start, End: end}, nil
}

// rangeLine returns a member as a line number: a non-negative decimal integer
// with no fraction or exponent that also fits int.
func rangeLine(members map[string]strictjson.Value, key string) (int, bool) {
	value, ok := members[key]
	if !ok {
		return 0, false
	}
	number, err := value.AsNonNegativeInt64()
	if err != nil || int64(int(number)) != number {
		return 0, false
	}
	return int(number), true
}

// helperPath is the path the range helper uses for a repository-relative file:
// the path relative to extension/, because extension/ is the helper's --root.
func helperPath(file string) string {
	path, found := strings.CutPrefix(file, "extension/")
	if !found {
		return file
	}
	return path
}

// rangesStop is the unreadable stop for the range helper's output. The output
// is one JSON document, so the line is 1.
func rangesStop(point Point, detail string, data []byte) *StopError {
	return stopError(point, InputRanges, 1, detail, string(data))
}
