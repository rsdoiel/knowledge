package knowledge

import (
	"strings"
)

const (
    // Version number of release
    Version = "0.0.3"

    // ReleaseDate, the date version.go was generated
    ReleaseDate = "2026-08-08"

    // ReleaseHash, the Git hash when version.go was generated
<<<<<<< HEAD
    ReleaseHash = "a21f85b"
=======
    ReleaseHash = "193fa97"
>>>>>>> 6f81c33478fb839db70903bb8800aa73c795ab0b
    LicenseText = `
knowledge is a SQLite3-backed knowledge base module for tracking projects, observations, and concepts
Copyright (C) 2026 R. S. Doiel

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.

`
)

// FmtHelp lets you process a text block with simple curly brace markup.
func FmtHelp(src string, appName string, version string, releaseDate string, releaseHash string) string {
	m := map[string]string {
		"{app_name}": appName,
		"{version}": version,
		"{release_date}": releaseDate,
		"{release_hash}": releaseHash,
	}
	for k, v := range m {
		if strings.Contains(src, k) {
			src = strings.ReplaceAll(src, k, v)
		}
	}
	return src
}

