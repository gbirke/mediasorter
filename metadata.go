package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/dhowden/tag"
)

// A path to a media file that was verified by the tag library to be an actual media file
type MediaFile string

type FileGroup struct {
	MediaFile    MediaFile
	SidecarFiles []string
}

type Metadata struct {
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	Format      tag.Format
	FileType    tag.FileType
	Genre       string
	Year        int

	Track int
	Disc  int
}

// CleanForPaths returns a new Metadata instance with fields cleaned for use in file paths.
// In Go, we use forward slashes on all architectures, no need to worry about OS-specific path separators.
func (m *Metadata) CleanForPaths() *Metadata {
	return &Metadata{
		Title:       strings.ReplaceAll(m.Title, "/", ""),
		Artist:      strings.ReplaceAll(m.Artist, "/", ""),
		AlbumArtist: strings.ReplaceAll(m.AlbumArtist, "/", ""),
		Album:       strings.ReplaceAll(m.Album, "/", ""),
		Format:      m.Format,
		FileType:    m.FileType,
		Genre:       strings.ReplaceAll(m.Genre, "/", ""),
		Year:        m.Year,
		Track:       m.Track,
		Disc:        m.Disc,
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

func (m *MetaDataReader) ReadMetadata(srcPath MediaFile) (*Metadata, error) {
	// read metadata from file
	f, err := os.Open(string(srcPath))
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", srcPath, err)
	}
	defer f.Close()

	// Use github.com/dhowden/tag for reading audio metadata
	rawMetadata, err := tag.ReadFrom(f)
	if err != nil {
		return nil, err
	}

	m.OutputWriter.Debug(fmt.Sprintf("Metadata for file %s - %v", srcPath, rawMetadata))

	// The seconds value is not an error, but the total
	// TODO: Add to our Metadata struct
	track, _ := rawMetadata.Track()
	disc, _ := rawMetadata.Disc()

	metadata := &Metadata{
		Title:       rawMetadata.Title(),
		Artist:      rawMetadata.Artist(),
		AlbumArtist: rawMetadata.AlbumArtist(),
		Album:       rawMetadata.Album(),
		Format:      rawMetadata.Format(),
		FileType:    rawMetadata.FileType(),
		Genre:       rawMetadata.Genre(),
		Year:        rawMetadata.Year(),
		Track:       track,
		Disc:        disc,
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
	var mediaFile MediaFile
	var sidecarFiles []string

	for _, file := range fileCandidates {
		// Try to identify if this is a media file
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("error opening file %s: %v", file, err)
		}
		defer f.Close()

		// Try to identify the file using the tag library
		// We ignore the format and filetype (they'll later be read by "ReadMetadata" anyway)
		// and are only interested in the error. If it is not nil, it means the tag library could
		// could not identify the file as a media file.
		_, _, err = tag.Identify(f)

		if err == nil {
			// This is a media file
			if mediaFile == "" {
				mediaFile = MediaFile(file)
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
