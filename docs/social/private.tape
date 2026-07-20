# Privacy loop: outbound connections is a number you can check, not a promise.
# Render with: vhs docs/social/private.tape

Output docs/social/private.gif

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 480
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "PS1='$ '; export PATH=/Users/benydishon/Developer/haypile/dist/hay_darwin_arm64_v8.0:$PATH; clear"
Enter
Show

Type "# does any of this leave my machine?"
Sleep 300ms
Enter
Sleep 500ms
Type "hay status"
Sleep 500ms
Enter
Sleep 7s
