# Test data

`NotoColorEmoji-Subset.ttf` holds U+1F600 and U+1F920 of the bitmap (CBDT, PNG) build of Noto Color
Emoji (Copyright Google Inc., SIL Open Font License 1.1, https://openfontlicense.org), the format of
Android's emoji font. It is cut from `opentype/bitmap/NotoColorEmoji.ttf` of
`github.com/go-text/typesetting-utils` v0.0.0-20260419141703-4ffe8874dabc (sha256
`959a9efd2bfca018c546ee952c6631dd38a179dd02f9d7385c0c97ab31f59b8e`) with fontTools:

```sh
pyftsubset NotoColorEmoji.ttf --unicodes=U+1F920,U+1F600 \
  --output-file=NotoColorEmoji-Subset.ttf --no-recalc-timestamp
```

It is used only by `textengine_test.go`, to draw emoji with the Android text engine (`textengine.go`)
without the fonts of a device.
