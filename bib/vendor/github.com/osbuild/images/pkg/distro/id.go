package distro

import (
	"fmt"
	"strconv"
	"strings"
)

// ID represents a distro name and version
type ID struct {
	Name         string
	MajorVersion int
	// MinorVersion is -1 if not specified
	MinorVersion int
}

func (id ID) String() string {
	if id.MinorVersion == -1 {
		return fmt.Sprintf("%s-%d", id.Name, id.MajorVersion)
	}

	return fmt.Sprintf("%s-%d.%d", id.Name, id.MajorVersion, id.MinorVersion)
}

type ParseError struct {
	ToParse string
	Msg     string
	Inner   error
}

func (e ParseError) Error() string {
	msg := fmt.Sprintf("error when parsing distro name (%s): %v", e.ToParse, e.Msg)

	if e.Inner != nil {
		msg += fmt.Sprintf(", inner error:\n%v", e.Inner)
	}

	return msg
}

// ParseID parses a distro name and version from a Distro ID string.
// This is the generic parser, which is used by all distros as the base parser.
//
// Limitations:
// - the distro name must not contain a dash
func ParseID(idStr string) (*ID, error) {
	idParts := strings.Split(idStr, "-")

	if len(idParts) < 2 {
		return nil, ParseError{ToParse: idStr, Msg: "A dash is expected to separate distro name and version"}
	}

	name := strings.Join(idParts[:len(idParts)-1], "-")
	version := idParts[len(idParts)-1]

	versionParts := strings.Split(version, ".")

	if len(versionParts) > 2 {
		return nil, ParseError{ToParse: idStr, Msg: fmt.Sprintf("too many dots in the version (%d)", len(versionParts)-1)}
	}

	majorVersion, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return nil, ParseError{ToParse: idStr, Msg: "parsing major version failed", Inner: err}
	}

	minorVersion := -1

	if len(versionParts) > 1 {
		minorVersion, err = strconv.Atoi(versionParts[1])

		if err != nil {
			return nil, ParseError{ToParse: idStr, Msg: "parsing minor version failed", Inner: err}
		}
	}

	return &ID{
		Name:         name,
		MajorVersion: majorVersion,
		MinorVersion: minorVersion,
	}, nil
}
