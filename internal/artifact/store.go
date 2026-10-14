package artifact

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// Names and bounds of an artifact file and its temporary file.
const (
	// tempPrefix starts a temporary artifact file's name. The leading dot
	// hides it from a directory listing, and the video ID follows so the next
	// run can collect the same video's leftovers.
	tempPrefix = ".yt2column-artifact-"
	// tempSuffix ends a temporary artifact file's name.
	tempSuffix = ".tmp"
	// artifactSuffix ends a final artifact file's name.
	artifactSuffix = ".json"
	// maxTempAttempts bounds the retries when a random temporary name
	// already exists.
	maxTempAttempts = 8
	// tempRandomBytes is the size of a temporary file name's random part.
	tempRandomBytes = 8
)

// Store reads and writes one artifacts directory. NewStore opens the directory
// once and every later read and write goes through that handle, so replacing
// the path afterwards does not redirect them. A Store is used by a single
// writer at a time; it holds no lock.
type Store struct {
	root      *os.Root
	collected bool
	// beforeOpen runs after the type check and before the open of the
	// directory (NewStore) or of an artifact (Load). Production leaves it
	// nil; a same-package test sets it to race a replacement between the two.
	beforeOpen func(name string)
}

// Store implements the writer's artifact boundary.
var _ writer.ArtifactStore = (*Store)(nil)

// NewStore opens dir as an artifacts directory. It rejects a missing path, a
// path that is not a directory, and a symbolic link with ErrMissing, and it
// never creates the directory.
func NewStore(dir string) (*Store, error) {
	return newStore(dir, nil)
}

// newStore is NewStore with the test hook used to replace the path between the
// type check and the open.
func newStore(dir string, beforeOpen func(string)) (*Store, error) {
	clean := filepath.Clean(dir)
	info, err := os.Lstat(clean) //nolint:gosec // stats the directory the caller named, before opening it
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMissing, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, ErrMissing
	}
	if beforeOpen != nil {
		beforeOpen(clean)
	}
	root, err := os.OpenRoot(clean) //nolint:gosec // opens the directory the caller named after checking its type
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMissing, err)
	}
	rootInfo, err := root.Stat(".")
	if err != nil || !os.SameFile(info, rootInfo) {
		_ = root.Close()
		return nil, ErrMissing
	}
	return &Store{root: root, beforeOpen: beforeOpen}, nil
}

// Close releases the directory handle. The Store must not be used afterwards.
func (s *Store) Close() error {
	if s.root == nil {
		return nil
	}
	return s.root.Close()
}

// Save writes one completed stage's record for videoID and returns its
// identifier. It writes a temporary file and renames it over the final name,
// so an interrupted write never leaves a partly written artifact under the
// final name.
func (s *Store) Save(ctx context.Context, videoID string, record writer.StageRecord) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, ok := transcript.NormalizedVideoURL(videoID); !ok {
		return "", errInvalidVideoID
	}
	if !isKnownStage(record.Stage) {
		return "", errInvalidStage
	}
	s.collectTempFiles(videoID)
	data, err := Marshal(record)
	if err != nil {
		return "", err
	}
	if err := s.writeArtifact(videoID, record.Stage, data); err != nil {
		return "", err
	}
	return recordID(record), nil
}

// Load verifies the chain of videoID's artifacts that ends at stage against
// materialID and returns it. A missing artifact is ErrMissing, a broken one is
// ErrMalformed, and one that belongs to another run is ErrMismatch.
func (s *Store) Load(ctx context.Context, videoID string, stage writer.Stage, materialID string) (writer.ArtifactChain, error) {
	if err := ctx.Err(); err != nil {
		return writer.ArtifactChain{}, err
	}
	if _, ok := transcript.NormalizedVideoURL(videoID); !ok {
		return writer.ArtifactChain{}, errInvalidVideoID
	}
	s.collectTempFiles(videoID)
	switch stage {
	case writer.StageGeneration:
		generation, err := s.loadGeneration(videoID, materialID)
		if err != nil {
			return writer.ArtifactChain{}, err
		}
		return writer.ArtifactChain{Generation: generation}, nil
	case writer.StageShorten:
		shorten, err := s.loadRecord(videoID, writer.StageShorten)
		if err != nil {
			return writer.ArtifactChain{}, err
		}
		generation, err := s.loadGeneration(videoID, materialID)
		if err != nil {
			return writer.ArtifactChain{}, err
		}
		if shorten.Input != generation.ID {
			return writer.ArtifactChain{}, mismatchf("the shorten artifact's input is %s, want the generation artifact's id %s", shorten.Input, generation.ID)
		}
		return writer.ArtifactChain{Generation: generation, Shorten: &shorten}, nil
	case writer.StageReview:
		return s.loadReview(videoID, materialID)
	default:
		return writer.ArtifactChain{}, errInvalidStage
	}
}

// loadGeneration loads the generation artifact and checks that its input is
// the current material.
func (s *Store) loadGeneration(videoID, materialID string) (writer.StageRecord, error) {
	generation, err := s.loadRecord(videoID, writer.StageGeneration)
	if err != nil {
		return writer.StageRecord{}, err
	}
	if generation.Input != materialID {
		return writer.StageRecord{}, mismatchf("the material's identifier is %s, the generation artifact's input is %s", materialID, generation.Input)
	}
	return generation, nil
}

// loadReview loads the review artifact and verifies the chain below it. The
// review's input names either the shorten artifact or, when the shorten step
// did not run, the generation artifact. A missing shorten artifact is the
// normal state of a run without the shorten step; a broken one is rejected
// because the previous stage cannot be decided.
func (s *Store) loadReview(videoID, materialID string) (writer.ArtifactChain, error) {
	review, err := s.loadRecord(videoID, writer.StageReview)
	if err != nil {
		return writer.ArtifactChain{}, err
	}
	shorten, shortenErr := s.loadRecord(videoID, writer.StageShorten)
	if shortenErr == nil {
		if review.Input == shorten.ID {
			generation, err := s.loadGeneration(videoID, materialID)
			if err != nil {
				return writer.ArtifactChain{}, err
			}
			if shorten.Input != generation.ID {
				return writer.ArtifactChain{}, mismatchf("the shorten artifact's input is %s, want the generation artifact's id %s", shorten.Input, generation.ID)
			}
			return writer.ArtifactChain{Generation: generation, Shorten: &shorten, Review: &review}, nil
		}
	} else if !errors.Is(shortenErr, ErrMissing) {
		return writer.ArtifactChain{}, shortenErr
	}
	generation, err := s.loadGeneration(videoID, materialID)
	if err != nil {
		return writer.ArtifactChain{}, err
	}
	if review.Input != generation.ID {
		return writer.ArtifactChain{}, mismatchf("the review artifact's input is %s, want the generation artifact's id %s", review.Input, generation.ID)
	}
	return writer.ArtifactChain{Generation: generation, Review: &review}, nil
}

// loadRecord reads one artifact file. It checks the name's type with Lstat,
// opens it without blocking, and checks the opened descriptor: a regular file,
// the same file Lstat described, and within the size limit. Any name that is
// not a regular file, and any change between the check and the open, is
// ErrMalformed.
func (s *Store) loadRecord(videoID string, stage writer.Stage) (writer.StageRecord, error) {
	name := artifactName(videoID, stage)
	info, err := s.root.Lstat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return writer.StageRecord{}, fmt.Errorf("%w: %w", ErrMissing, err)
		}
		return writer.StageRecord{}, malformedf("artifact %s could not be examined", stage)
	}
	if !info.Mode().IsRegular() {
		return writer.StageRecord{}, malformedf("artifact %s is not a regular file", stage)
	}
	if s.beforeOpen != nil {
		s.beforeOpen(name)
	}
	file, err := s.root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // opens the fixed artifact name under the directory handle, after the type check
	if err != nil {
		return writer.StageRecord{}, malformedf("artifact %s changed between the check and the open", stage)
	}
	defer func() { _ = file.Close() }()
	openedInfo, err := file.Stat()
	if err != nil {
		return writer.StageRecord{}, malformedf("artifact %s could not be examined", stage)
	}
	if !openedInfo.Mode().IsRegular() {
		return writer.StageRecord{}, malformedf("artifact %s is not a regular file", stage)
	}
	if !os.SameFile(info, openedInfo) {
		return writer.StageRecord{}, malformedf("artifact %s changed between the check and the open", stage)
	}
	if openedInfo.Size() > maxArtifactBytes {
		return writer.StageRecord{}, malformedf("artifact exceeds the limit of %d bytes", maxArtifactBytes)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxArtifactBytes+1))
	if err != nil {
		return writer.StageRecord{}, malformedf("artifact %s could not be read", stage)
	}
	if len(data) > maxArtifactBytes {
		return writer.StageRecord{}, malformedf("artifact exceeds the limit of %d bytes", maxArtifactBytes)
	}
	record, err := Parse(data)
	if err != nil {
		return writer.StageRecord{}, err
	}
	if record.Stage != stage {
		return writer.StageRecord{}, mismatchf("the artifact's stage is %s, want %s", record.Stage, stage)
	}
	return record, nil
}

// writeArtifact writes data to videoID's stage file through a temporary file
// and a rename. A failure removes the temporary file.
func (s *Store) writeArtifact(videoID string, stage writer.Stage, data []byte) error {
	file, tempName, err := s.createTemp(videoID, stage)
	if err != nil {
		return err
	}
	writeErr := writeAll(file, data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = s.root.Remove(tempName)
		return errors.Join(writeErr, closeErr)
	}
	if err := s.root.Rename(tempName, artifactName(videoID, stage)); err != nil {
		_ = s.root.Remove(tempName)
		return fmt.Errorf("%w: %w", ErrMissing, err)
	}
	s.syncDir()
	return nil
}

// createTemp creates a fresh temporary file in the artifacts directory.
// O_CREATE and O_EXCL reject an existing name, a symbolic link included.
func (s *Store) createTemp(videoID string, stage writer.Stage) (*os.File, string, error) {
	for range maxTempAttempts {
		var suffix [tempRandomBytes]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, "", err
		}
		name := tempPrefix + videoID + "." + stage.String() + "." + hex.EncodeToString(suffix[:]) + tempSuffix
		file, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, "", fmt.Errorf("%w: %w", ErrMissing, err)
		}
		return file, name, nil
	}
	return nil, "", fmt.Errorf("%w: could not create a temporary artifact file", ErrMissing)
}

// writeAll writes data to file, looping over short writes.
func writeAll(file *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := file.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// syncDir best-effort syncs the artifacts directory after a rename. A failure
// is ignored: the rename already happened, so reporting it as a save failure
// would be wrong.
func (s *Store) syncDir() {
	dir, err := s.root.Open(".")
	if err != nil {
		return
	}
	_ = dir.Sync()
	_ = dir.Close()
}

// collectTempFiles removes the temporary files a prior interrupted run of the
// same video left behind. It runs once per Store, at the first Load or Save,
// when the video ID is known.
func (s *Store) collectTempFiles(videoID string) {
	if s.collected {
		return
	}
	s.collected = true
	entries, err := fs.ReadDir(s.root.FS(), ".")
	if err != nil {
		return
	}
	prefix := tempPrefix + videoID + "."
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, tempSuffix) {
			_ = s.root.Remove(name)
		}
	}
}

// artifactName is the fixed name of one video's stage artifact.
func artifactName(videoID string, stage writer.Stage) string {
	return videoID + "." + stage.String() + artifactSuffix
}
