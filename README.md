# Audio file mover

This command line tool moves/copies audio files in a directory according
to their metadata and a path template (using Go template syntax), creating
subdirectories as needed.

The tool sanitizes the file names to avoid path traversal, extra
directories and hard-to-escape file names on the shell.

If a file has "sidecar" files (files with the same name as the media file
but with a different suffix), the tool will rename them as well.

## Usage

    go run . srcPath destPath


## Future ideas

- Configurable templates, configure from config directory
- Optional Functions in the templates - case change, transliterate Unicode
    chars, etc.
- Improve verbose output - show when we move sidecar files
