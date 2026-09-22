package input

// MiSTer forwards physical set-2 keys. Translate the printable US layout while
// retaining legacy navigation aliases for screens that do not accept text.
type keyboard struct {
	down map[int]bool
	caps bool
}

func newKeyboard() *keyboard { return &keyboard{down: make(map[int]bool)} }

var letterCodes = map[int]rune{
	0x1c: 'a', 0x32: 'b', 0x21: 'c', 0x23: 'd', 0x24: 'e', 0x2b: 'f', 0x34: 'g',
	0x33: 'h', 0x43: 'i', 0x3b: 'j', 0x42: 'k', 0x4b: 'l', 0x3a: 'm', 0x31: 'n',
	0x44: 'o', 0x4d: 'p', 0x15: 'q', 0x2d: 'r', 0x1b: 's', 0x2c: 't', 0x3c: 'u',
	0x2a: 'v', 0x1d: 'w', 0x22: 'x', 0x35: 'y', 0x1a: 'z',
}

var symbolCodes = map[int][2]rune{
	0x16: {'1', '!'}, 0x1e: {'2', '@'}, 0x26: {'3', '#'}, 0x25: {'4', '$'}, 0x2e: {'5', '%'},
	0x36: {'6', '^'}, 0x3d: {'7', '&'}, 0x3e: {'8', '*'}, 0x46: {'9', '('}, 0x45: {'0', ')'},
	0x0e: {'`', '~'}, 0x4e: {'-', '_'}, 0x55: {'=', '+'}, 0x54: {'[', '{'}, 0x5b: {']', '}'},
	0x5d: {'\\', '|'}, 0x4c: {';', ':'}, 0x52: {'\'', '"'}, 0x41: {',', '<'}, 0x49: {'.', '>'},
	0x4a: {'/', '?'}, 0x29: {' ', ' '},
}

func (k *keyboard) decode(word uint32) (Event, bool) {
	code := int(word & 0x1ff)
	down := word&0x200 != 0
	repeat := down && k.down[code]
	k.down[code] = down
	switch code {
	case 0x12, 0x59, 0x14, 0x114, 0x11, 0x111:
		return Event{}, false
	case 0x58:
		if down && !repeat {
			k.caps = !k.caps
		}
		return Event{}, false
	}
	ev := Event{Key: None, Keyboard: true, ScanCode: code, Repeat: repeat, Release: !down}
	if key, ok := ps2map[code]; ok {
		ev.Key = key
	}
	if code == 0x15a {
		ev.Key = Enter
	}
	shift := k.down[0x12] || k.down[0x59]
	if !k.down[0x14] && !k.down[0x114] && !k.down[0x11] && !k.down[0x111] {
		if letter, ok := letterCodes[code]; ok {
			ev.Text = letter
			if shift != k.caps {
				ev.Text -= 'a' - 'A'
			}
		} else if pair, ok := symbolCodes[code]; ok {
			ev.Text = pair[0]
			if shift {
				ev.Text = pair[1]
			}
		}
	}
	return ev, ev.Key != None || ev.Text != 0 || code == 0x0d || code == 0x171
}
