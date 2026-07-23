# Live Unlimited-OCR flow: index a scanned image-only PDF, then search it.
# Prep (fresh page variant so OCR truly runs) happens hidden via prep-scan.sh.
# Render with: vhs docs/social/ocr-live.tape

Output docs/social/ocr-live.gif

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 560
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set PlaybackSpeed 1.5

Hide
Type "PS1='$ '; export PATH=/private/tmp/claude-501/-Users-benydishon-Developer-haypile/dc50ed42-0694-4e4e-bef9-699ff5f494e3/scratchpad/bin:$PATH; bash /private/tmp/claude-501/-Users-benydishon-Developer-haypile/dc50ed42-0694-4e4e-bef9-699ff5f494e3/scratchpad/prep-scan.sh; clear"
Enter
Show

Type "# a scanned 1987 memo: one page image, no text layer"
Sleep 300ms
Enter
Type "# OCR: Baidu's Unlimited-OCR via Ollama, fully local"
Sleep 300ms
Enter
Type "hay add ~/haypile-demo/scans"
Sleep 500ms
Enter
Sleep 20s

Type 'hay search "worn conveyor belt needs replacing" --limit 1'
Sleep 500ms
Enter
Sleep 7s

Type 'hay search "B-7" --limit 1'
Sleep 500ms
Enter
Sleep 8s
