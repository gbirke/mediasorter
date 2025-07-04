package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dhowden/tag"
	"github.com/urfave/cli/v3"
)

// TODO read template from file, explain whitespace trimming and placeholders in README
var defaultPathTemplate = `
	{{- or .AlbumArtist .Artist -}}
	{{- pathSep -}}
	{{- .Album -}}
	{{- pathSep -}}
	{{- if .Track }}{{ printf "%02d" .Track }}. {{ end -}}
	{{- .Title -}}
`

type OverrideChecker interface {
	DestinationFileExists(destPath string) bool
}

type NoOverrideChecker struct {
}

func (n *NoOverrideChecker) DestinationFileExists(destPath string) bool {
	return false
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

type FileExistsError struct {
	srcPath  string
	destPath string
}

func (err *FileExistsError) Error() string {
	return fmt.Sprintf("File %s already exists, skipping %s\n", err.destPath, err.srcPath)
}

type FileProcessor func(srcPath string, destPath string) error

func DryRunFileProcessor(srcPath string, destPath string) error {
	return nil
}

func CopyFile(srcPath string, destPath string) (err error) {
	// create destination directory if it does not exist
	err = os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %v", filepath.Dir(destPath), err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating file %s: %v", destPath, err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("error closing file %s: %v", destPath, closeErr)
		}
	}()
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

func MoveFile(srcPath string, destPath string) (err error) {
	// create destination directory if it does not exist
	err = os.MkdirAll(filepath.Dir(destPath), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %v", filepath.Dir(destPath), err)
	}

	err = os.Rename(srcPath, destPath)
	if err != nil {
		return fmt.Errorf("error moving file %s to %s: %v", srcPath, destPath, err)
	}

	return nil
}

type MediaSorter struct {
	DestDir         string
	PathTemplate    *template.Template
	MetadataReader  *MetaDataReader
	FileProcessor   FileProcessor
	OverrideChecker OverrideChecker
	OutputWriter    *OutputWriter
}

func (m *MediaSorter) ProcessFileGroup(group *FileGroup) error {
	metadata, err := m.MetadataReader.ReadMetadata(group.MediaFile)

	if err != nil {
		re, ok := err.(*NotAMediaFileError)
		if ok {
			m.OutputWriter.Info(re.Error())
			return nil
		}
		return err
	}

	// Generate the destination path and `destPath` for sidecar files, using the template
	var pathBuffer bytes.Buffer
	if err := m.PathTemplate.Execute(&pathBuffer, metadata.CleanForPaths()); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}
	pathStr := cleanPath(pathBuffer.String())
	mediaExt := filepath.Ext(string(group.MediaFile))
	destPath := filepath.Join(m.DestDir, pathStr+mediaExt)

	if string(group.MediaFile) == destPath {
		return fmt.Errorf("destination path %s is the same as source path, skipping", destPath)
	}

	m.OutputWriter.Info(fmt.Sprintf("Processing file %s -> %s", group.MediaFile, destPath))

	if m.OverrideChecker.DestinationFileExists(destPath) {
		m.OutputWriter.Warn(fmt.Sprintf("File %s already exists, skipping %s", destPath, group.MediaFile))
		return nil
	}

	err = m.FileProcessor(string(group.MediaFile), destPath)
	if err != nil {
		return err
	}

	// Process sidecar files
	for _, sidecarFile := range group.SidecarFiles {
		sidecarExt := filepath.Ext(sidecarFile)
		sidecarDestPath := filepath.Join(m.DestDir, pathStr+sidecarExt)

		err := m.FileProcessor(sidecarFile, sidecarDestPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MediaSorter) Sort(srcDir string) error {
	// First pass: collect all files and group by path without suffix
	fileGroups := make(map[string][]string)
	// Walk recursively through the source directory
	err := filepath.WalkDir(srcDir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// We don't do anything with directories, filepath.WalkDir will recursively walk them anyway
		if info.IsDir() {
			return nil
		}

		// Skip hidden files on Unix-like systems
		isHidenOnUnix := strings.HasPrefix(info.Name(), ".")
		if isHidenOnUnix {
			return nil
		}

		basename := strings.TrimSuffix(path, filepath.Ext(path))
		fileGroups[basename] = append(fileGroups[basename], path)

		return nil
	})

	if err != nil {
		return err
	}

	// Second pass: process each group
	for basename, files := range fileGroups {

		group, err := m.MetadataReader.GetFileGroup(files)

		if err != nil {
			switch len(files) {
			case 0:
				m.OutputWriter.Warn(fmt.Sprintf("Strange error: No files found in group '%s'. This should never happen. Please contact program author", basename))
			case 1:
				m.OutputWriter.Warn(fmt.Sprintf("%s is not a media file, skipping", files[0]))
			default:
				m.OutputWriter.Warn(fmt.Sprintf("No media file found for %d files starting with %s, skipping", len(files), basename))
			}
			continue
		}

		err = m.ProcessFileGroup(group)

		if err == tag.ErrNoTagsFound {
			m.OutputWriter.Warn(fmt.Sprintf("No tags found in file %s, skipping", group.MediaFile))
			continue
		}

		switch err.(type) {
		case *FileExistsError:
			m.OutputWriter.Warn(err.Error())
		case *NotAMediaFileError:
			m.OutputWriter.Warn(err.Error())
		case nil:
			// Success, continue
		default:
			return err
		}
	}

	return nil
}

func main() {
	var verbosity int
	app := &cli.Command{
		Name:                   "media-sorter",
		Usage:                  "Copy or move media files into subdirectories, based on their metadata and a path template.",
		UseShortOptionHandling: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "dry-run",
				Aliases: []string{"d"},
				Usage:   "Do not move/copy files, just print the new file names",
			},
			&cli.BoolFlag{
				Name:    "move",
				Aliases: []string{"m"},
				Usage:   "Move files instead of copying",
			},
			&cli.BoolFlag{
				Name:  "override",
				Usage: "Override existing files",
			},
			&cli.StringFlag{
				Name:    "template",
				Aliases: []string{"t"},
				Usage:   "Path to a Go template for new file names, with placeholders for metadata",
			},

			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Display every file action",
				Config: cli.BoolConfig{
					Count: &verbosity,
				},
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArg{
				Name: "srcDir",
			},
			&cli.StringArg{
				Name: "destDir",
			},
		},
		ArgsUsage: "<source directory> [destination directory]",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			srcDir := cmd.StringArg("srcDir")
			destDir := cmd.StringArg("destDir")

			if srcDir == "" {
				// TODO show usage here, after the error message
				return cli.Exit("Source directory is required", 1)
			}

			outputWriter := &OutputWriter{Quiet}
			if Verbosity(verbosity) == Verbose {
				outputWriter.Verbosity = Verbose
			} else if Verbosity(verbosity) >= Debug {
				outputWriter.Verbosity = Debug
			}

			var fileProcessor = CopyFile
			if cmd.Bool("move") {
				if cmd.Bool("dry-run") {
					outputWriter.Warn("Dry run mode is not compatible with move operation, no files will be moved")
				}
				fileProcessor = MoveFile
			}
			if cmd.Bool("dry-run") {
				fileProcessor = DryRunFileProcessor
				// Dry run mode should always be verbose to show what would happen
				if Verbosity(verbosity) < Verbose {
					outputWriter.Verbosity = Verbose
				}
			}

			var overrideChecker OverrideChecker = &NoOverrideChecker{}
			if cmd.Bool("override") {
				overrideChecker = &MemoryOverrideChecker{SeenFiles: make(map[string]struct{})}
			}

			var templateStr = defaultPathTemplate
			if cmd.String("template") != "" {
				templateFilePath := cmd.String("template")
				templateFileContents, err := os.ReadFile(templateFilePath)
				if err != nil {
					return fmt.Errorf("error reading template file %s: %v", templateFilePath, err)
				}
				templateStr = string(templateFileContents)
			}

			pathTemplate, err := template.New("path").Funcs(template.FuncMap{
				// Path separator function to make the separator more visible in templates than a simple "/"
				"pathSep":           func() string { return "/" },
				"replaceInBrackets": ReplaceInBrackets,
				"removeBrackets":    RemoveBrackets,
				// TODO add more custom functions for normalizing names:
				// - underscores instead of spaces
				// - transform unicode
				// - etc
			}).Parse(templateStr)
			if err != nil {
				return fmt.Errorf("error parsing template: %v", err)
			}
			// Check if template is valid by executing it with a dummy Metadata struct
			if err := pathTemplate.Execute(io.Discard, &Metadata{}); err != nil {
				return fmt.Errorf("error executing template: %v", err)
			}

			metadataReader := &MetaDataReader{outputWriter}
			mediaSorter := &MediaSorter{
				DestDir:         destDir,
				PathTemplate:    pathTemplate,
				FileProcessor:   fileProcessor,
				MetadataReader:  metadataReader,
				OverrideChecker: overrideChecker,
				OutputWriter:    outputWriter,
			}

			fi, err := os.Stat(srcDir)
			if err != nil {
				return fmt.Errorf("error getting file info for %s: %v", srcDir, err)
			}

			// Check if source directory is a file or directory
			if fi.IsDir() {
				return mediaSorter.Sort(srcDir)
			}

			// Process single file
			fg, err := metadataReader.GetFileGroup([]string{srcDir})
			if err != nil {
				return err
			}
			return mediaSorter.ProcessFileGroup(fg)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
