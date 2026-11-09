package phon

import (
	"strings"
	"unicode"
)

// romanMap gives ASCII spellings for IPA base symbols (longest keys first in
// lookup). Unlisted letters with ASCII forms pass through.
var romanMap = map[string]string{
	"t͡ʃ": "ch", "d͡ʒ": "j", "t͡s": "ts", "d͡z": "dz", "t͡ɕ": "ch", "d͡ʑ": "j", "t̠͡ʃ": "ch", "d̠͡ʒ": "j",
	"ʃ": "sh", "ʒ": "zh", "θ": "th", "ð": "dh", "x": "kh", "χ": "kh", "ɣ": "gh", "ʁ": "r",
	"ŋ": "ng", "ɲ": "ny", "ʔ": "'", "ʕ": "'", "ħ": "h", "ɦ": "h", "ə": "e", "ɛ": "e",
	"ɔ": "o", "æ": "ae", "ø": "oe", "œ": "oe", "y": "ü", "ɨ": "y", "ɯ": "u", "ʉ": "u",
	"ɪ": "i", "ʊ": "u", "ɑ": "a", "ɒ": "o", "ʌ": "u", "ɐ": "a", "ɤ": "o", "β": "v",
	"ɸ": "f", "ç": "ch", "ɾ": "r", "ɹ": "r", "ɽ": "r", "ʀ": "r", "ɭ": "l", "ɫ": "l",
	"ʎ": "ly", "ɟ": "gy", "c": "ky", "j": "y", "ɡ": "g", "ʋ": "v", "ɕ": "sh", "ʑ": "zh",
	"ʂ": "sh", "ʐ": "zh", "ʈ": "t", "ɖ": "d", "ɳ": "n", "ɻ": "r", "ɓ": "b", "ɗ": "d",
	"ʄ": "j", "ɠ": "g", "q": "q", "ɢ": "g", "ɴ": "n", "ʝ": "y", "ɰ": "w", "ɥ": "w",
	"ʍ": "wh", "ɬ": "lh", "ɮ": "l", "ɺ": "r", "ʙ": "b", "ⱱ": "v", "ʜ": "h", "ʢ": "'",
}

// Romanize gives a plain ASCII-ish spelling of a word, for names.
func Romanize(w Word) string {
	var b strings.Builder
	for i, s := range w.Segs {
		ipa := RenderFeatures(s.F)
		if ipa == "ŋ" && i+1 < len(w.Segs) && w.Segs[i+1].F.Has(Dorsal) && !w.Segs[i+1].F.Has(Sonorant) {
			b.WriteString("n") // ŋɡ → ng, ŋk → nk
			continue
		}
		if r, ok := romanMap[ipa]; ok {
			b.WriteString(r)
			continue
		}
		// Strip diacritics and map the base letter.
		var base strings.Builder
		for _, r := range ipa {
			if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Lm, r) {
				continue
			}
			base.WriteRune(r)
		}
		bs := base.String()
		if r, ok := romanMap[bs]; ok {
			b.WriteString(r)
		} else {
			for _, r := range bs {
				if r < 128 {
					b.WriteRune(r)
				}
			}
		}
		if s.F.Has(Long) && s.IsVowel() {
			// Long vowels double their last letter (aː → aa).
			if rs := []rune(b.String()); len(rs) > 0 {
				b.WriteRune(rs[len(rs)-1])
			}
		}
	}
	return b.String()
}

// Capitalize upper-cases the first letter of a name. A leading apostrophe
// (a romanized ʔ or ʕ) is dropped, as English writes Ali, not 'Ali.
func Capitalize(s string) string {
	if t := strings.TrimLeft(s, "'"); t != "" {
		s = t
	}
	for i, r := range s {
		if unicode.IsLetter(r) {
			return s[:i] + string(unicode.ToUpper(r)) + s[i+len(string(r)):]
		}
	}
	return s
}
