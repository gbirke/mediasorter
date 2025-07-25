# Audio file mover

This command line tool moves/copies audio files in a directory according
to their metadata and a path template (using [Go template
syntax](https://pkg.go.dev/text/template)), creating subdirectories as
needed.

The tool sanitizes the file names coming from the template, to avoid path
traversal, extra directories and hard-to-escape file names on the shell.

If a file has "sidecar" files (files with the same name as the media file
but with a different suffix), the tool will rename them as well.

**Supported Audio Formats**: Using the Go library
[dhowden/tag](https://github.com/dhowden/tag), `mediamover` supports
metadata from  MP3 (ID3v1,2.{2,3,4}) and MP4 (ACC, M4A, ALAC), OGG and
FLAC.

## Usage

```shell
medisorter srcPath destPath
```

`srcPath` can be a directory or a single file.

`destPath` must either not exist or be a directory.

### Command line flags

    -d, --dry-run   Show old and new name without overriding
    -m, --move      Move files instead of copying them
    --override      Override existing files
    -t, --template  Specify a custom template file.
    -v, --verbose   show verbose output
    -h, --help      show this help message and exit

## Template syntax

The custom template files follow the regular Go template syntax. See
[https://golang.org/pkg/text/template/](https://golang.org/pkg/text/template/)
for more information.

Enclose placeholders for file metadata in two curly brackets.

Put slashes ("/") or `{{ pathSep }}` between sections to create a subdirectories.

Have a look at the file `example.tmpl` to see an example.

### Available metadata placeholders

- `.Title`
- `.Artist`
- `.AlbumArtist`
- `.Album`
- `.Format`
- `.FileType`
- `.Genre`
- `.Year`
- `.Track`
- `.Disc`

### Custom template functions

#### pathSep

A function with no arguments that creates a path separator in the file name.
You can use this function to make path separators more visible than using
a slash ("/").

#### removeBrackets

Use this for removing qualifiers in brackets in song and album names.
For example "(Parental Advisory)" or "(Remastered)".
Give it a parameter to specify which brackets to remove. The following example
will remove all content in square and round brackets:

```
{{ .Title | removeBrackets "([])" }}
```

If you want to target specific content in brackets, you can put the words that
should trigger the removal inside the brackets. If you have more than one
"match word", separate the words with comma:

```
{{ .Album | removeBrackets "[(remastered)]" }}
{{ .Title | removeBrackets "(parental,explicit)" }}
```

The software will match case-insensitive and the whole text inside the brackets,
so the above example would match "(Parental Advisory)" and "(Explicit Lyrics)"
as well.

#### replaceInBrackets

Similar to `removeBrackets`, but allows you to specify a replacement:

```
{{ .Title | replaceInBrackets "(extended version)" "XXL" }}
```

## Future ideas

- I have to come up with better handling with songs from *compilation albums* where
   `AlbumArtist` is empty or different for every track.
  - Can I have a complex template that checks for compilation albums and sorts them differently?
  - Is there a flag to mark compilation albums?
- Configuration directory, with templates. Template parameter will search for a
    template file in the configuration directory if the software can't find the
    template relative to the current working directory.
- Add more functions in the templates - case change, transliterate Unicode
    characters to ASCII, etc.
- Improve verbose output - show when we move sidecar files

## Alternatives to this software

I wrote this software because I wanted two features that I could not find in other tools:

- Metadata from the files, not external media databases
- Creating artist- and album-centered directory structures from a flat file list

The software I tried may or may not have these features, but I could not
easily find them. But of course, they have lots of other cool features,
that `mediamover` does not.

- [filebot](https://www.filebot.net/) A cross-platform media file organization
    tool with GUI support, scripting capabilities, media database integration, and more.
- [musicbrainz picard](https://picard.musicbrainz.org/) Another cross-platform 
     media file organization tool with GUI support, using the open source music database MusicBrainz.
- [beets](https://beets.io/) A music library manager that can organize your music 
     collection, tag files, and more.
