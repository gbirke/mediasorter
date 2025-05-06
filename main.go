package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/dhowden/tag"
)

// TODO read template from file, explain purpose of whitespace trimming (allows for complex templates with logic)
// var pathTemplate = "{{- .Artist -}}/{{- .Album -}}/{{- .Track -}} {{- .Title -}}.{{- .Ext -}}"
var pathTemplate = "{{- .Artist -}}/{{- .Album -}}/{{- .Title -}}"

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

// Options contains configuration parameters for file processing
type Options struct {
	DestDir  string
	Override bool
	Move     bool
	DryRun   bool
}

// TODO return processing result errors that indicate skipped files
func processFile(srcPath string, opts *Options) error {
	// read metadata from file
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", srcPath, err)
	}
	defer f.Close()

	// Try to identify the file type to avoid seek errors later
	// The files might not be audio files but other metadata files or images
	_, _, err = tag.Identify(f)
	if err != nil {
		// TODO return skip error instead
		return nil
	}

	// Use github.com/dhowden/tag for reading audio metadata
	rawMetadata, err := tag.ReadFrom(f)
	if err != nil {
		return err
	}

	fmt.Printf("Metadata for file %s:\n", srcPath)
	fmt.Printf("Raw: %+v\n", rawMetadata)

	track, _ := rawMetadata.Track()
	disc, _ := rawMetadata.Disc()

	// TODO clean up metadata - remove newlines, slashes, colons and tabs, to avoid problems with file names
	m := Metadata{
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

	// debug print metadata
	fmt.Printf("%v\n", m)

	// render metadata as text
	// TODO move template parsing to main and pass template to processFile
	templ, err := template.New("path").Parse(pathTemplate)
	if err != nil {
		return fmt.Errorf("error parsing template: %v", err)
	}
	var pathStr bytes.Buffer
	if err := templ.Execute(&pathStr, m); err != nil {
		return fmt.Errorf("error executing template: %v", err)
	}

	// TODO remove newlines and tabs from pathStr in case the template is "bad"

	// TODO check for path traversal attacks and skip if detected

	// parse text as path
	newFileName := filepath.Join(opts.DestDir, pathStr.String())

	// check if path exists, skip file if override is not set
	if !opts.Override {
		if _, err := os.Stat(newFileName); err == nil {
			// TODO return skip error instead
			fmt.Printf("File %s already exists, skipping\n", newFileName)
			return nil
		}
	}

	if opts.DryRun {
		fmt.Printf("Processing file %s -> %s\n", srcPath, newFileName)
		return nil
	}
	// create destination directory if it does not exist
	err = os.MkdirAll(filepath.Dir(newFileName), 0755)
	if err != nil {
		return fmt.Errorf("error creating directory %s: %v", filepath.Dir(newFileName), err)
	}

	// move/copy file to destination directory, delete original file if move is set
	if opts.Move {
		err = os.Rename(srcPath, newFileName)
		if err != nil {
			return fmt.Errorf("error moving file %s to %s: %v", srcPath, newFileName, err)
		}
	} else {
		destFile, err := os.Create(newFileName)
		if err != nil {
			return fmt.Errorf("error creating file %s: %v", newFileName, err)
		}
		defer destFile.Close()
		_, err = io.Copy(destFile, f)
		if err != nil {
			return fmt.Errorf("error copying file %s to %s: %v", srcPath, newFileName, err)
		}
	}

	return nil
}

func main() {
	// Define command line flags
	override := flag.Bool("override", false, "Override existing files")
	move := flag.Bool("move", false, "Move files instead of copying")
	dryRun := flag.Bool("dry-run", false, "Do not move/copy files, just print the new file names")
	// TODO add flag for template and/or template file
	// TODO add flag for verbosity and help

	flag.Parse()
	args := flag.Args()
	// TODO make destDir optional, path can also bet set in template
	if len(args) < 2 {
		flag.Usage()
		return
	}
	srcDir := args[0]
	destDir := args[1]

	// Create options struct with inline initialization
	opts := &Options{
		DestDir:  destDir,
		Override: *override,
		Move:     *move,
		DryRun:   *dryRun,
	}

	// iterate over all files in source directory
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		err = processFile(path, opts)

		if err == tag.ErrNoTagsFound {
			fmt.Printf("No tags found in file %s, skipping\n", path)
			return nil
		}

		// TODO handle skip errors

		return err
	})
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

