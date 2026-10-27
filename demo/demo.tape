# Record with: vhs demo/demo.tape   (run from the repository root)
# Everything shown comes from demo/setup.sh's generated machine: a fake HOME
# on a small disk image, so free space and the Trash are the image's own.
Output demo/demo.gif
Set Shell bash
Set FontSize 13
Set Width 1160
Set Height 620
Set Padding 14
Set TypingSpeed 45ms
Set Framerate 12

Hide
Type "source demo/setup.sh" Enter
Sleep 45s
Type "clear" Enter
Sleep 500ms
Show

Type "decruft" Sleep 300ms Enter
Sleep 5s

# Xcode: every item with its safety label and the exact command or action.
Enter Sleep 2s
Down Sleep 1.6s
Type "G" Sleep 2.2s
Escape Sleep 700ms

# Flutter: items found by flutterkill, handed back to it to clean.
Down Down Down Down Down Sleep 600ms
Enter Sleep 2.4s
Escape Sleep 600ms
Up Up Up Up Up Sleep 500ms

# The same space as a treemap; zoom into the biggest category.
Type "t" Sleep 2.2s
Enter Sleep 2.4s
Escape Sleep 500ms
Escape Sleep 800ms

# Select every safe item, review the plan, run it.
Type "s" Sleep 1.5s
Type "c" Sleep 4s
Type "y" Sleep 7s

# Back to the overview: cleaned categories are struck through.
Type "x" Sleep 2.8s
Type "q" Sleep 4s

Hide
Type "cd / && hdiutil detach -quiet /tmp/decruft-demo/home; rm -rf /tmp/decruft-demo" Enter
Sleep 3s
