# Hero demo: index a folder, search by meaning, verify zero outbound connections.
# Render with: vhs docs/social/hero.tape

Output docs/social/hero.gif

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 640
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "PS1='$ '; export PATH=/Users/benydishon/Developer/haypile/dist/hay_darwin_arm64_v8.0:$PATH; hay remove ~/haypile-demo >/dev/null 2>&1; clear"
Enter
Show

Type "hay add ~/haypile-demo"
Sleep 500ms
Enter
Sleep 8s

Type 'hay search "agreement cancellation" --limit 3'
Sleep 500ms
Enter
Sleep 6s

Type "hay status"
Sleep 500ms
Enter
Sleep 6s
