package cli

import "strings"

// emulate plays what a program wrote to a terminal onto a character grid and
// returns the rows. Bubble Tea v2 redraws only what changed, moving the cursor
// between pieces, so a phrase such as "Enter accept" is no longer one run of
// bytes on the wire; the tests ask what is on the screen, as a person would.
// It handles the sequences kb's interface produces (cursor movement, erase,
// repeat) and ignores colours, modes and operating-system commands.
func emulate(raw string, cols, rows int) string {
	grid := make([][]rune, rows)
	clear := func(r int) {
		grid[r] = []rune(strings.Repeat(" ", cols))
	}
	for r := range grid {
		clear(r)
	}
	x, y := 0, 0
	top, bottom := 0, rows-1
	scroll := func() { // the region moves up one row; its last row is blank
		copy(grid[top:bottom], grid[top+1:bottom+1])
		clear(bottom)
	}
	var last rune
	put := func(r rune) {
		if x >= cols {
			x = 0
			y++
		}
		if y > bottom {
			scroll()
			y = bottom
		}
		grid[y][x] = r
		x++
		last = r
	}
	in := []rune(raw)
	for i := 0; i < len(in); i++ {
		c := in[i]
		switch {
		case c == '\r':
			x = 0
		case c == '\n':
			if y == bottom {
				scroll()
			} else if y < rows-1 {
				y++
			}
		case c == '\b':
			if x > 0 {
				x--
			}
		case c == 0x1b && i+1 < len(in) && in[i+1] == ']': // OSC: to BEL or ST
			i += 2
			for i < len(in) && in[i] != 0x07 && !(in[i] == 0x1b && i+1 < len(in) && in[i+1] == '\\') {
				i++
			}
			if i < len(in) && in[i] == 0x1b {
				i++
			}
		case c == 0x1b && i+1 < len(in) && in[i+1] == '[':
			j := i + 2
			for j < len(in) && (in[j] < 0x40 || in[j] > 0x7e) {
				j++
			}
			if j >= len(in) {
				i = len(in)
				break
			}
			params, final := string(in[i+2:j]), in[j]
			i = j
			if strings.ContainsAny(params, "?<>=") {
				if final == 'h' && strings.HasPrefix(params, "?1049") {
					for r := range grid {
						clear(r)
					}
					x, y = 0, 0
				}
				break
			}
			n := func(k, def int) int {
				ps := strings.Split(params, ";")
				if k >= len(ps) || ps[k] == "" {
					return def
				}
				v := 0
				for _, d := range ps[k] {
					if d < '0' || d > '9' {
						return def
					}
					v = v*10 + int(d-'0')
				}
				return v
			}
			switch final {
			case 'H', 'f':
				y, x = n(0, 1)-1, n(1, 1)-1
			case 'A':
				y -= n(0, 1)
			case 'B':
				y += n(0, 1)
			case 'C':
				x += n(0, 1)
			case 'D':
				x -= n(0, 1)
			case 'G':
				x = n(0, 1) - 1
			case 'd':
				y = n(0, 1) - 1
			case 'K':
				switch n(0, 0) {
				case 0:
					for k := x; k < cols; k++ {
						grid[min(max(y, 0), rows-1)][k] = ' '
					}
				case 1:
					for k := 0; k <= x && k < cols; k++ {
						grid[min(max(y, 0), rows-1)][k] = ' '
					}
				default:
					clear(min(max(y, 0), rows-1))
				}
			case 'J':
				switch n(0, 0) {
				case 0:
					for k := x; k < cols; k++ {
						grid[min(max(y, 0), rows-1)][k] = ' '
					}
					for r := y + 1; r < rows; r++ {
						clear(r)
					}
				default:
					for r := range grid {
						clear(r)
					}
				}
			case 'X':
				for k := x; k < x+n(0, 1) && k < cols; k++ {
					grid[min(max(y, 0), rows-1)][k] = ' '
				}
			case 'r':
				top, bottom = n(0, 1)-1, n(1, rows)-1
				x, y = 0, 0
			case 'S':
				for k := n(0, 1); k > 0; k-- {
					scroll()
				}
			case 'b':
				for k := n(0, 1); k > 0; k-- {
					put(last)
				}
			}
			x, y = min(max(x, 0), cols), min(max(y, 0), rows-1)
			if y < 0 {
				y = 0
			}
		case c == 0x1b:
			i++ // a two-character escape
		case c >= ' ':
			put(c)
		}
	}
	var b strings.Builder
	for _, row := range grid {
		b.WriteString(strings.TrimRight(string(row), " "))
		b.WriteByte('\n')
	}
	return b.String()
}
