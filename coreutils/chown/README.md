**See https://codereview.appspot.com/101310044/ for more information about why
we need to install a patch**

### IMPORTANT NOTES:

- (1) `-R` could get stuck on looped symlinks (e.g. foo -> bar -> foo -> bar)
- (2) `--from=` doesn't work yet