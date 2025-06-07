package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dhowden/tag"
)

type FileGroup struct {
	MediaFile    string
	SidecarFiles []string
}

type Metadata struct {
	Title    string
	Artist   string
	Album    string
	Format   tag.Format
	FileType tag.FileType
	Genre    string
	Year     int

	Track int
	Disc  int
}

// CleanForPaths returns a new Metadata instance with fields cleaned for use in file paths.
// In Go, we use forward slashes on all architectures, no need to worry about OS-specific path separators.
func (m *Metadata) CleanForPaths() *Metadata {
	return &Metadata{
		Title:    strings.ReplaceAll(m.Title, "/", ""),
		Artist:   strings.ReplaceAll(m.Artist, "/", ""),
		Album:    strings.ReplaceAll(m.Album, "/", ""),
		Format:   m.Format,
		FileType: m.FileType,
		Genre:    strings.ReplaceAll(m.Genre, "/", ""),
		Year:     m.Year,
		Track:    m.Track,
		Disc:     m.Disc,
	}
}

type MetaDataReader struct {
	OutputWriter *OutputWriter
}

type NotAMediaFileError struct {
	srcPath string
}

func (m *NotAMediaFileError) Error() string {
	return fmt.Sprintf("'%s' is probably not a media file than can be parsed", m.srcPath)
}

func (m *MetaDataReader) ReadMetadata(srcPath string) (*Metadata, error) {
	// read metadata from file
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", srcPath, err)
	}
	defer f.Close()

	// Try to identify the file type, to avoid seek errors in tag.ReadFrom
	// The files might not be audio files but other metadata files or images
	_, _, err = tag.Identify(f)
	if err != nil {
		return nil, &NotAMediaFileError{srcPath}
	}

	// Use github.com/dhowden/tag for reading audio metadata
	rawMetadata, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	m.OutputWriter.Debug(fmt.Sprintf("Metadata for file %s - %v", srcPath, rawMetadata))

	track, _ := rawMetadata.Track()
	disc, _ := rawMetadata.Disc()

	metadata := &Metadata{
		Title:    rawMetadata.Title(),
		Artist:   rawMetadata.Artist(),
		Album:    rawMetadata.Album(),
		Format:   rawMetadata.Format(),
		FileType: rawMetadata.FileType(),
		Genre:    rawMetadata.Genre(),
		Year:     rawMetadata.Year(),
		Track:    track,
		Disc:     disc,
	}

	m.OutputWriter.Debug(fmt.Sprintf("Created Metadata: %v", metadata))
	return metadata, nil
}

func (m *MetaDataReader) GetFileGroup(fileCandidates []string) (*FileGroup, error) {
	if len(fileCandidates) == 0 {
		// This should not happen, but just in case
		return nil, fmt.Errorf("no files found in the group, skipping")
	}

	// Find the media file in the group
	var mediaFile string
	var sidecarFiles []string

	for _, file := range fileCandidates {
		// Try to identify if this is a media file
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("error opening file %s: %v", file, err)
		}
		defer f.Close()

		_, _, err = tag.Identify(f)

		if err == nil {
			// This is a media file
			if mediaFile == "" {
				mediaFile = file
			} else {
				// Multiple media files with same basename - treat others as sidecars
				sidecarFiles = append(sidecarFiles, file)
			}
		} else {
			// This is a sidecar file
			sidecarFiles = append(sidecarFiles, file)
		}
	}

	if mediaFile == "" {
		return nil, fmt.Errorf("no media file found in the group, skipping")
	}

	return &FileGroup{
		MediaFile:    mediaFile,
		SidecarFiles: sidecarFiles,
	}, nil
}
