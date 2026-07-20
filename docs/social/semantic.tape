# Semantic search closeup: paraphrases match by meaning, not keywords.
# Render with: vhs docs/social/semantic.tape

Output docs/social/semantic.gif

Set Shell "bash"
Set FontSize 20
Set Width 1100
Set Height 620
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms

Hide
Type "PS1='$ '; export PATH=/Users/benydishon/Developer/haypile/dist/hay_darwin_arm64_v8.0:$PATH; clear"
Enter
Show

Type '# none of my files contain the word "cancellation"'
Sleep 300ms
Enter
Type 'hay search "agreement cancellation" --limit 2'
Sleep 500ms
Enter
Sleep 5s

Type '# plain questions work too'
Sleep 300ms
Enter
Type 'hay search "can we get out of the office lease early" --limit 1'
Sleep 500ms
Enter
Sleep 6s
