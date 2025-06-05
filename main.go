package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dhowden/tag"
)

// TODO read template from file, explain purpose of whitespace trimming (allows for complex templates with logic)
// var defaultPathTemplate = "{{- .Artist -}}/{{- .Album -}}/{{- .Track -}} {{- .Title -}}.{{- .Ext -}}"
var defaultPathTemplate = "{{- .Artist -}}/{{- .Album -}}/{{- .Title -}}"

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

type MetaDataReader struct {
	logger *slog.Logger
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

	m.logger.Debug("Metadata for file", "file", srcPath, "rawMetadata", slog.AnyValue(rawMetadata))

	track, _ := rawMetadata.Track()
	disc, _ := rawMetadata.Disc()

	// TODO clean up metadata - remove newlines, slashes, colons and tabs, to avoid problems with file names
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

	m.logger.Debug("Read Metadata", "metadata", slog.AnyValue(metadata))
	return metadata, nil
}

func (m *MetaDataReader) GetFileGroup(fileCandidates []string) (*FileGroup, error) {
	if len(fileCandidates) == 0 {
		return nil, fmt.Errorf("No files found in the group, skipping.")
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
		return nil, fmt.Errorf("No media file found in the group, skipping")
	}

	return &FileGroup{
		MediaFile:    mediaFile,
		SidecarFiles: sidecarFiles,
	}, nil
}

type OverrideChecker interface {
	DestinationFileExists(destPath string) bool
}

type NoOverrideChecker struct {
}

func (n *NoOverrideChecker) DestinationFileExists(destPath string) bool {
	return false
}

type FilesystemOverrideChecker struct{}

func (f *FilesystemOverrideChecker) DestinationFileExists(destPath string) bool {
	_, err := os.Stat(destPath)
	return err == nil
}

type MemoryOverrideChecker struct {
	SeenFiles map[string]struct{}
}

func (m *MemoryOverrideChecker) DestinationFileExists(destPath string) bool {
	if _, exists := m.SeenFiles[destPath]; exists {
		return true
	}
	m.SeenFiles[destPath] = struct{}{}
	return false
}

type ChainingOverrideChecker struct {
	overrideCheckers []OverrideChecker
}

func (c *ChainingOverrideChecker) DestinationFileExists(destPath string) bool {
	for _, checker := range c.overrideCheckers {
		if checker.DestinationFileExists(destPath) {
			return true
		}
	}
	return false
}

type FileExistsError struct {
	srcPath  string
	destPath string
}

func (err *FileExistsError) Error() string {
	return fmt.Sprintf("File %s already exists, skipping %s\n", err.destPath, err.srcPath)
}

type FileProcessor interface {
	ProcessFile(srcPath string, destPath string) error
}

type OutputProcessor struct {
	overrideChecker OverrideChecker
}

func (o *OutputProcessor) ProcessFile(srcPath string, destPath string) error {
	if o.overrideChecker.DestinationFileExists(destPath) {
		return &FileExistsError{destPath, srcPath}
	}
	fmt.Printf("Processing file %s -> %s\n", srcPath, destPath)
	return nil
}

type CopyProcessor struct {
	overrideChecker OverrideChecker
}

func (c *CopyProcessor) ProcessFile(srcPath string, destPath string) error {
	if c.overrideChecker.DestinationFileExists(destPath) {
		return &FileExistsError{destPath, srcPath}
	}

	// create destination directory if it does not exist
	err := os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %v", filepath.Dir(destPath), err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", destPath, err)
	}
	defer destFile.Close()
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", srcPath, err)
	}
	defer f.Close()
	_, err = io.Copy(destFile, f)
	if err != nil {
		return fmt.Errorf("error copying file %s to %s: %v", srcPath, destPath, err)
	}
	return nil
}

type MoveProcessor struct {
	overrideChecker OverrideChecker
}

func (m *MoveProcessor) ProcessFile(srcPath string, destPath string) error {
	if m.overrideChecker.DestinationFileExists(destPath) {
		return &FileExistsError{destPath, srcPath}
	}

	// create destination directory if it does not exist
	err := os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %v", filepath.Dir(destPath), err)
	}

	err = os.Rename(srcPath, destPath)
	if err != nil {
		return fmt.Errorf("error moving file %s to %s: %v", srcPath, destPath, err)
	}

	return nil
}

type FileGroup struct {
	MediaFile    string
	SidecarFiles []string
}

type MediaSorter struct {
	DestDir        string
	PathTemplate   *template.Template
	MetadataReader *MetaDataReader
	FileProcessors []FileProcessor
}

func (m *MediaSorter) ProcessFileGroup(group *FileGroup) error {
	metadata, err := m.MetadataReader.ReadMetadata(group.MediaFile)

	if err != nil {
		re, ok := err.(*NotAMediaFileError)
		if ok {
			fmt.Println(re.Error())
			// TODO return result with error msg (skipped) instead of printing, leaving the printing to the output
			return nil
		}
		return err
	}

	var pathStr bytes.Buffer
	if err := m.PathTemplate.Execute(&pathStr, metadata); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}
	// TODO remove newlines and tabs from pathStr in case the template is "bad"

	// TODO check for path traversal attacks and skip if detected

	// Process the main media file
	mediaExt := filepath.Ext(group.MediaFile)
	destPath := filepath.Join(m.DestDir, pathStr.String()+mediaExt)

	for _, processor := range m.FileProcessors {
		err := processor.ProcessFile(group.MediaFile, destPath)
		if err != nil {
			return err
		}
	}

	// Process sidecar files
	for _, sidecarFile := range group.SidecarFiles {
		sidecarExt := filepath.Ext(sidecarFile)
		sidecarDestPath := filepath.Join(m.DestDir, pathStr.String()+sidecarExt)

		for _, processor := range m.FileProcessors {
			err := processor.ProcessFile(sidecarFile, sidecarDestPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// TODO use custom output class instead of fmt.Printf to implement verbosity and colors
func (m *MediaSorter) Sort(srcDir string) error {
	// First pass: collect all files and group by basename
	fileGroups := make(map[string][]string)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// TODO recursive processing?
		if info.IsDir() {
			return nil
		}

		// Group files by basename (filename without extension)
		basename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		fileGroups[basename] = append(fileGroups[basename], path)

		return nil
	})

	if err != nil {
		return err
	}

	// Second pass: process each group
	for _, files := range fileGroups {

		group, err := m.MetadataReader.GetFileGroup(files)

		if err != nil {
			fmt.Printf("No media file found for basename %s, skipping group\n", filepath.Base(files[0]))
			continue
		}

		err = m.ProcessFileGroup(group)

		if err == tag.ErrNoTagsFound {
			fmt.Printf("No tags found in file %s, skipping\n", group.MediaFile)
			continue
		}

		switch err.(type) {
		case *FileExistsError:
			fmt.Print(err.Error())
		case *NotAMediaFileError:
			fmt.Print(err.Error())
		case nil:
			// Success, continue
		default:
			return err
		}
	}

	return nil
}

func main() {
	// Define command line flags
	override := flag.Bool("override", false, "Override existing files")
	//move := flag.Bool("move", false, "Move files instead of copying")
	//dryRun := flag.Bool("dry-run", false, "Do not move/copy files, just print the new file names")
	// TODO add flag for template and/or template file
	// TODO add flag for verbosity and help
	// TODO add flag for progress bar (mutually exclusive with verbosity)

	flag.Parse()
	args := flag.Args()
	// TODO make destDir optional, path can also bet set in template
	if len(args) < 2 {
		// TODO add required arguments when printing usage
		flag.Usage()
		return
	}
	srcDir := args[0]
	destDir := args[1]

	var fileProcessors []FileProcessor
	var overrideChecker OverrideChecker = &NoOverrideChecker{}

	if *override {
		overrideChecker = &FilesystemOverrideChecker{}
	}

	fileProcessors = append(fileProcessors, &OutputProcessor{overrideChecker: overrideChecker})

	// TODO re-enable when the new architecture works
	// if !*dryRun {
	// 	if *move {
	// 		fileProcessors = append(fileProcessors, &MoveProcessor{overrideChecker: overrideChecker})
	// 	} else {
	// 		fileProcessors = append(fileProcessors, &CopyProcessor{overrideChecker: overrideChecker})
	// 	}
	// }

	pathTemplate, err := template.New("path").Parse(defaultPathTemplate)
	if err != nil {
		panic(fmt.Sprintf("Error parsing template: %v", err))
	}
	// TODO add custom functions for normalizing names - underscores instead of spaces, transform unicode, etc

	mediaSorter := &MediaSorter{
		DestDir:        destDir,
		PathTemplate:   pathTemplate,
		FileProcessors: fileProcessors,
		MetadataReader: &MetaDataReader{slog.Default()},
	}
	err = mediaSorter.Sort(srcDir)

	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
