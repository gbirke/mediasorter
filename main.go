package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dhowden/tag"
	"github.com/urfave/cli/v3"
)

// TODO read template from file, explain purpose of whitespace trimming (allows for complex templates with logic)
var defaultPathTemplate = "{{- .Artist -}}/{{- .Album -}}/{{- .Title -}}"

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

func CopyFile(srcPath string, destPath string) error {
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

func MoveFile(srcPath string, destPath string) error {
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

	var pathBuffer bytes.Buffer
	if err := m.PathTemplate.Execute(&pathBuffer, metadata.CleanForPaths()); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}
	// remove newlines and tabs from pathStr in case the template is "bad"
	pathStr := cleanPath(pathBuffer.String())

	// Process the main media file
	mediaExt := filepath.Ext(string(group.MediaFile))
	destPath := filepath.Join(m.DestDir, pathStr+mediaExt)

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
			m.OutputWriter.Warn(fmt.Sprintf("No media file found for basename %s, skipping group", filepath.Base(files[0])))
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
		Usage:                  fmt.Sprintf("Copy or move media files into subdirectories, based on their metadata and a path template.\n\nDefault Template: %s", defaultPathTemplate),
		UseShortOptionHandling: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "override",
				Usage: "Override existing files",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"v"},
				Usage:   "Set verbosity",
				Config: cli.BoolConfig{
					Count: &verbosity,
				},
			},
			//&cli.BoolFlag{
			//	Name:  "move",
			//	Usage: "Move files instead of copying",
			//},
			//&cli.BoolFlag{
			//	Name:  "dry-run",
			//	Usage: "Do not move/copy files, just print the new file names",
			//},
			// TODO add flag for template and/or template file
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
				return cli.Exit("Source directory is required", 1)
			}

			var fileProcessor = DryRunFileProcessor
			var overrideChecker OverrideChecker = &NoOverrideChecker{}

			if cmd.Bool("override") {
				overrideChecker = &MemoryOverrideChecker{SeenFiles: make(map[string]struct{})}
			}

			outputWriter := &OutputWriter{Quiet}
			if Verbosity(verbosity) == Verbose {
				outputWriter.Verbosity = Verbose
			} else if Verbosity(verbosity) >= Debug {
				outputWriter.Verbosity = Debug
			}

			// TODO re-enable when the new architecture works
			// if !ctx.Bool("dry-run") {
			// 	if ctx.Bool("move") {
			// 		fileProcessors = append(fileProcessors, &MoveProcessor{overrideChecker: overrideChecker})
			// 	} else {
			// 		fileProcessors = append(fileProcessors, &CopyProcessor{overrideChecker: overrideChecker})
			// 	}
			// }

			pathTemplate, err := template.New("path").Parse(defaultPathTemplate)
			if err != nil {
				return fmt.Errorf("error parsing template: %v", err)
			}
			// TODO add custom functions for normalizing names - underscores instead of spaces, transform unicode, etc

			mediaSorter := &MediaSorter{
				DestDir:         destDir,
				PathTemplate:    pathTemplate,
				FileProcessor:   fileProcessor,
				MetadataReader:  &MetaDataReader{outputWriter},
				OverrideChecker: overrideChecker,
				OutputWriter:    outputWriter,
			}
			return mediaSorter.Sort(srcDir)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}
