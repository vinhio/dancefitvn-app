package kit_test

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyOp is one call the keyboard of iOS makes on the view it types into, as the
// field gets it: text entered (insertText:), a range selected
// (setSelectedTextRange:, from sel[0] to sel[1] in UTF-16 units), or a deletion
// (deleteBackward).
type keyOp struct {
	text   string
	sel    [2]int32
	choose bool
	del    bool
}

// The calls.
func ins(s string) keyOp       { return keyOp{text: s} }
func sel(a, b int32) keyOp     { return keyOp{sel: [2]int32{a, b}, choose: true} }
func del() keyOp               { return keyOp{del: true} }
func key(ops ...keyOp) []keyOp { return ops }

// telexKeys is what the Vietnamese Telex keyboard of iOS did at each key of a
// code, recorded on a simulator of iOS 27 with the keyboard's own engine
// (UIKeyboardImpl, B-code3), in the three ways it types:
//
//   - lent: into a field that lends it its text (a ui.TextDocument). To put a
//     mark on a letter it selects the letters from there on, deletes them and
//     types them again;
//   - blind: into a field that lends it no text. It deletes as many letters as it
//     remembers typing and types them again;
//   - told: into a field that lends it no text and tells it after every key that
//     the text changed. It has nothing to combine a key with: every key is one
//     text, in the case its Shift key happens to be in, the W key alone the U with
//     a horn.
type telexKeys struct {
	keys              string
	lent, blind, told [][]keyOp
}

// telex are the codes recorded. The first has no letter the keyboard combines.
var telex = []telexKeys{
	{
		keys:  "7k4m2p",
		lent:  [][]keyOp{key(ins("7")), key(ins("k")), key(ins("4")), key(ins("m")), key(ins("2")), key(ins("p"))},
		blind: [][]keyOp{key(ins("7")), key(ins("k")), key(ins("4")), key(ins("m")), key(ins("2")), key(ins("p"))},
		told:  [][]keyOp{key(ins("7")), key(ins("K")), key(ins("4")), key(ins("M")), key(ins("2")), key(ins("P"))},
	},
	{
		keys: "vieetj",
		lent: [][]keyOp{
			key(ins("V")), key(ins("i")), key(ins("e")),
			key(sel(1, 3), del(), ins("i"), ins("ê")),
			key(ins("t")),
			key(sel(2, 4), del(), ins("ê"), sel(1, 3), del(), ins("i"), ins("ệt")),
		},
		blind: [][]keyOp{
			key(ins("V")), key(ins("i")), key(ins("e")),
			key(del(), ins("ê")),
			key(ins("t")),
			key(del(), del(), ins("ệt")),
		},
		told: [][]keyOp{key(ins("V")), key(ins("I")), key(ins("E")), key(ins("E")), key(ins("T")), key(ins("J"))},
	},
	{
		keys: "7as4rx",
		lent: [][]keyOp{
			key(ins("7")), key(ins("a")),
			key(sel(0, 2), del(), ins("7"), ins("á")),
			key(ins("4")), key(ins("r")), key(ins("x")),
		},
		blind: [][]keyOp{
			key(ins("7")), key(ins("a")),
			key(del(), ins("á")),
			key(ins("4")), key(ins("r")), key(ins("x")),
		},
		told: [][]keyOp{key(ins("7")), key(ins("A")), key(ins("S")), key(ins("4")), key(ins("R")), key(ins("X"))},
	},
	{
		keys: "ddawuw",
		lent: [][]keyOp{
			key(ins("D")),
			key(sel(0, 1), del(), ins("Đ")),
			key(ins("a")),
			key(sel(0, 2), del(), ins("Đ"), ins("ă")),
			key(ins("u")), key(ins("w")),
		},
		blind: [][]keyOp{
			key(ins("D")),
			key(del(), ins("Đ")),
			key(ins("a")),
			key(del(), ins("ă")),
			key(ins("u")), key(ins("w")),
		},
		told: [][]keyOp{key(ins("D")), key(ins("d")), key(ins("a")), key(ins("ư")), key(ins("U")), key(ins("Ư"))},
	},
	{
		keys: "tans9f",
		lent: [][]keyOp{
			key(ins("T")), key(ins("a")), key(ins("n")),
			key(sel(1, 3), del(), ins("a"), sel(0, 2), del(), ins("T"), ins("án")),
			key(ins("9")), key(ins("f")),
		},
		blind: [][]keyOp{
			key(ins("T")), key(ins("a")), key(ins("n")),
			key(del(), del(), ins("án")),
			key(ins("9")), key(ins("f")),
		},
		told: [][]keyOp{key(ins("T")), key(ins("A")), key(ins("N")), key(ins("S")), key(ins("9")), key(ins("F"))},
	},
	{
		keys: "huyrwj",
		lent: [][]keyOp{
			key(ins("H")), key(ins("u")), key(ins("y")),
			key(sel(1, 3), del(), ins("u"), ins("ỷ")),
			key(ins("w")), key(ins("j")),
		},
		blind: [][]keyOp{
			key(ins("H")), key(ins("u")), key(ins("y")),
			key(del(), ins("ỷ")),
			key(ins("w")), key(ins("j")),
		},
		told: [][]keyOp{key(ins("H")), key(ins("U")), key(ins("Y")), key(ins("R")), key(ins("Ư")), key(ins("J"))},
	},
}

// press makes the calls of one key on the focused field, in one turn of the UI
// thread as the keyboard does.
func press(tt *ui.Tester, ops []keyOp) {
	for _, op := range ops {
		switch {
		case op.choose:
			tt.TextSource().SelectText(platform.TextRange{Start: op.sel[0], End: op.sel[1]})
		case op.del:
			tt.Key(keys.KeyBackspace, 0)
		default:
			tt.Type(op.text)
		}
	}
}

// typeTelex types the keys of k into the focused field as the keyboard of iOS
// does for the field it finds: lent its text or not, told after the key before or
// not. gDesk's UIKit driver tells it on the turn after each key of a field that
// asks for the keyboard of a code (platform.KeyboardCode, proposal G15), and lends
// such a field no text. A turn passes after each key.
func typeTelex(tt *ui.Tester, k telexKeys) {
	for i := range k.keys {
		in := tt.TextInput() // read at every key: the keyboard forgets only what came before
		switch {
		case tt.TextSource().TextLength() >= 0 && in.Keyboard != platform.KeyboardCode:
			press(tt, k.lent[i])
		case in.Keyboard == platform.KeyboardCode || i == 0:
			press(tt, k.told[i])
		default:
			press(tt, k.blind[i])
		}
		tt.Elapse(kittest.Frame)
	}
}

// TestCodeBoxesVietnameseKeyboard is the device bug (iPhone 15, the Vietnamese
// keyboard of iOS): "after the first three characters, the characters in the six
// boxes jump around". The field lent the keyboard its text, the keyboard selected
// the letters to put a mark on, deleted them and typed them again, and the field,
// which pinned its caret at the end and dropped every letter with a mark, lost
// one letter, doubled another and shifted the rest (`7`, `a`, `s` showed `77`;
// `v`, `i`, `e`, `e`, `t`, `j` showed `VIIIT`). Typed into as the keyboard types
// into the field it finds, the boxes show the keys pressed, in their order.
func TestCodeBoxesVietnameseKeyboard(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, k := range telex {
			row, tt, _ := codeRig(t, ph)
			typeTelex(tt, k)
			assert.Equal(t, strings.ToUpper(k.keys), row.Value, "the keys %q", k.keys)
			for i, ch := range strings.ToUpper(k.keys) {
				assert.Truef(t, tt.Shows(string(ch)), "%q: box %d shows %c", k.keys, i, ch)
			}
		}
	})
}

// TestCodeBoxesLendTheKeyboardNoText pins what the fix stands on: the field asks
// for the keyboard of a code, which gDesk serves lending no text and having the
// keyboard forget each key, and is no text document either, so that the keyboard
// reads no text and has no range to select or replace.
func TestCodeBoxesLendTheKeyboardNoText(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row, tt, _ := codeRig(t, ph)
		_, doc := row.Field().(ui.TextDocument)
		require.False(t, doc, "the field of a code must stay no ui.TextDocument (B-code3)")
		tt.Type("ABC")
		src := tt.TextSource()
		assert.Equal(t, -1, src.TextLength(), "the keyboard is lent no text")
		assert.Empty(t, src.TextIn(platform.TextRange{Start: 0, End: 3}))
		_, marked := src.MarkedText()
		assert.False(t, marked)
		src.SelectText(platform.TextRange{Start: 0, End: 2}) // nothing to select: a deletion takes the last character
		backspace(t, row, tt)
		assert.Equal(t, "AB", row.Value)

		in := tt.TextInput()
		assert.True(t, in.Enabled, "the keyboard is up")
		assert.Equal(t, platform.KeyboardCode, in.Keyboard)
		assert.False(t, in.Multiline)
		tt.Elapse(kittest.Frame)
		box := frameOf(codeBoxesOf(row)[2])
		assert.Equal(t, box, tt.TextInput().Caret, "the caret the keyboard is told is the box the next character goes to")
	})
}

// TestCodeBoxesKeyboardNotTold types as the keyboard does when nothing tells it to
// forget (two keys within one turn of the run loop): it combines the keys it
// remembers, deleting and typing again. No character is lost, doubled or moved
// besides the key the keyboard made a mark of: the boxes show the letters a text
// field would, without their marks.
func TestCodeBoxesKeyboardNotTold(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for typed, want := range map[string]string{
			"7k4m2p": "7K4M2P",
			"vieetj": "VIET",  // Việt
			"7as4rx": "7A4RX", // 7á4rx
			"ddawuw": "DAUW",  // Đăuw
			"tans9f": "TAN9F", // Tán9f
			"huyrwj": "HUYWJ", // Huỷwj
		} {
			row, tt, told := codeRig(t, ph)
			for _, k := range telex {
				if k.keys != typed {
					continue
				}
				for _, ops := range k.blind {
					press(tt, ops)
					tt.Elapse(kittest.Frame)
				}
			}
			assert.Equal(t, want, row.Value, "the keys %q", typed)
			assert.Equal(t, want, (*told)[len(*told)-1])
		}
	})
}

// TestCodeBoxesComposingKeyboard types as a keyboard that composes every word
// does (a Telex keyboard of Android that pays no heed to the input type gDesk asks
// for a code):
// the composition grows a key at a time, and a key the keyboard makes a mark of
// changes the letter before it. The letter stays in its box, as the letter under
// the mark; nothing vanishes or moves.
func TestCodeBoxesComposingKeyboard(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		row, tt, told := codeRig(t, ph)
		tt.Type("7") // a digit is committed at once
		for _, step := range []struct{ composing, value string }{
			{"a", "7A"},
			{"á", "7A"}, // the S key
			{"ás", "7AS"},
			{"á", "7A"}, // the delete key
			{"ám", "7AM"},
			{"ấm", "7AM"}, // the A key
			{"ấmư", "7AMW"},
		} {
			tt.Compose(step.composing, len([]rune(step.composing)))
			assert.Equal(t, step.value, row.Value, "composing %q", step.composing)
		}
		tt.Type("ấmư") // the next digit commits the word, then is committed
		tt.Type("9")
		assert.Equal(t, "7AMW9", row.Value)
		assert.Equal(t, []string{"7", "7A", "7AS", "7A", "7AM", "7AMW", "7AMW9"}, *told, "OnChange hears every change of the code, once")
		backspace(t, row, tt)
		backspace(t, row, tt)
		assert.Equal(t, "7AM", row.Value)
	})
}
