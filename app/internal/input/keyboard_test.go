package input

import "testing"

func TestKeyboardTextModifiersAndLegacyNavigation(t *testing.T) {
	k := newKeyboard()
	check := func(word uint32, text rune, key Key, repeat, release bool) {
		t.Helper()
		ev, ok := k.decode(word)
		if !ok || !ev.Keyboard || ev.Text != text || ev.Key != key || ev.Repeat != repeat || ev.Release != release {
			t.Fatalf("word %x: %+v", word, ev)
		}
	}
	check(0x21c, 'a', Left, false, false)
	check(0x21c, 'a', Left, true, false)
	check(0x01c, 'a', Left, false, true)
	k.decode(0x212) // left shift
	check(0x21c, 'A', Left, false, false)
	k.decode(0x01c)
	check(0x216, '!', None, false, false)
	k.decode(0x016)
	k.decode(0x012)
	k.decode(0x258) // caps lock; repeated make must not toggle it again
	k.decode(0x258)
	k.decode(0x058)
	check(0x21c, 'A', Left, false, false)
	k.decode(0x01c)
	k.decode(0x259) // right shift + caps -> lower case
	check(0x21c, 'a', Left, false, false)
	k.decode(0x01c)
	k.decode(0x059)
	check(0x229, ' ', Enter, false, false)
	check(0x266, 0, Back, false, false)
	check(0x375, 0, Up, false, false)
	check(0x35a, 0, Enter, false, false)
	check(0x20d, 0, None, false, false)
	check(0x371, 0, None, false, false)
}

func TestKeyboardPrintableLayout(t *testing.T) {
	k := newKeyboard()
	for code, want := range letterCodes {
		ev, ok := k.decode(uint32(code) | 0x200)
		if !ok || ev.Text != want {
			t.Fatalf("letter %x: %+v", code, ev)
		}
		k.decode(uint32(code))
	}
	for code, pair := range symbolCodes {
		ev, _ := k.decode(uint32(code) | 0x200)
		if ev.Text != pair[0] {
			t.Fatalf("symbol %x", code)
		}
		k.decode(uint32(code))
		k.decode(0x212)
		ev, _ = k.decode(uint32(code) | 0x200)
		if ev.Text != pair[1] {
			t.Fatalf("shift symbol %x", code)
		}
		k.decode(uint32(code))
		k.decode(0x012)
	}
	k.decode(0x214)
	if ev, _ := k.decode(0x221); ev.Text != 0 {
		t.Fatal("Ctrl+C inserted text")
	}
}
