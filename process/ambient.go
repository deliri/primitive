package process

import (
	"os"
)

// AmbientArguments reports the calling process's arguments after argv[0].
// The executable identity has its own Executable door; keeping it out of this
// projection makes the result exactly the arguments owned by the command.
func AmbientArguments() ([]Argument, error) {
	if len(os.Args) == 0 {
		return ParseArguments(nil)
	}
	return ParseArguments(os.Args[1:])
}

// StandardStreams returns the calling process's three standard byte streams
// through the same Streams capability shape used for direct children.
func StandardStreams() (Streams, error) {
	streams := Streams{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	if err := streams.Validate(); err != nil {
		return Streams{}, err
	}
	return streams, nil
}
