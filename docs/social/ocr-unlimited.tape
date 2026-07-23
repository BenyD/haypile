# Unlimited-OCR demo: scanned image-only PDF, transcribed by a local model, searchable.
# Render with: vhs docs/social/ocr-unlimited.tape

Output docs/social/ocr-unlimited.gif

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 500
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "PS1='$ '; export PATH=/Users/benydishon/Developer/haypile/dist/hay_darwin_arm64_v8.0:$PATH; clear"
Enter
Show

Type "# a scanned 1987 memo: image only, no text layer, OCR by Unlimited-OCR running locally"
Sleep 300ms
Enter
Type 'hay search "worn conveyor belt needs replacing" --limit 1'
Sleep 500ms
Enter
Sleep 5s

Type '# exact identifiers still match exactly'
Sleep 300ms
Enter
Type 'hay search "B-7" --limit 1'
Sleep 500ms
Enter
Sleep 7s
