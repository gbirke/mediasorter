package main

import (
	"bytes"
	"flag"
	"fmt"
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

// TODO return processing result errors that indicate skipped files
func processFile(srcPath string, destDir string, override bool, move bool) error {
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

	// parse text as path
	newFileName := filepath.Join(destDir, pathStr.String())
	// newDir := filepath.Dir(newFileName)

	// check if path exists, skip file if override is not set
	if !override {
		if _, err := os.Stat(newFileName); err == nil {
			// TODO return skip error instead
			fmt.Printf("File %s already exists, skipping\n", newFileName)
			return nil
		}
	}

	fmt.Printf("Processing file %s -> %s\n", srcPath, newFileName)
	// create destination directory if it does not exist
	// move/copy file to destination directory, delete original file if move is set
	return nil
}

func main() {

	// Define command line flags

	override := flag.Bool("override", false, "Override existing files")
	move := flag.Bool("move", false, "Move files instead of copying")
	// TODO add flag for template and/or template file
	// TODO add flag for verbosity and help

	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
		return
	}
	srcDir := args[0]
	destDir := args[1]

	// iterate over all files in source directory
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		err = processFile(path, destDir, *override, *move)

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
