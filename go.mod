module github.com/jfreymuth/oggvorbis

go 1.15

require github.com/jfreymuth/vorbis v1.0.2

// PROTOTYPE (dvoyni/cog#514): the split API lives in the sibling vorbis clone
// on its proto/setup-split branch. Not for the cog branch.
replace github.com/jfreymuth/vorbis => ../vorbis
