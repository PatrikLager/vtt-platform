// Package artlib reads art out of a campaign's flat art/ directory.
//
// THE FILENAME IS THE IDENTITY. Nothing declares an id, so nothing can
// disagree with one (design spec §3.2). That is why Lookup does no ReadDir:
// knowing the id IS knowing the path, and a scan would only be a slower way to
// arrive at the same filename — while quietly becoming the boot-time load this
// design exists to delete.
//
// AN ART ID IS KEBAB-CASE, and that is enforced here rather than assumed. Spec
// §3.2 says stems are kebab-case; leaving it advisory made "nothing can
// disagree with the filename" false on macOS, where Lookup(dir, "MASONRY-1")
// opens masonry-1.json and hands back a Piece whose ID and File echo the
// REQUEST. The same campaign then fails on a case-sensitive Linux server.
//
// The ID half is closed on every platform: restricted to lowercase letters,
// digits and single hyphens, an id has no case for a filesystem to fold.
// The DISK half is closed by Validate, which refuses a name no map could ever
// spell — but only when Validate runs. Between two runs, a Masonry-1.png
// dropped in by hand still resolves for "masonry-1" on macOS and hands the
// renderer a filename Linux does not have. Closing that inside Lookup needs a
// ReadDir, which is the boot-time load this design exists to delete, so the
// residual stands: it is the same cp-after-boot window spec §5 already accepts,
// and install or the next boot is what catches it.
//
// UNIQUENESS IS THE FILESYSTEM'S (spec §3.3), which is why art/ is flat.
// art/a/x.png and art/b/x.png coexist happily, and the moment they can,
// something has to decide what "x" means. Validate is what refuses a
// subdirectory — and a symlink, which is the same thing wearing a disguise:
// fs.DirEntry.IsDir() reads the entry's own type, so a symlink to a directory
// answers false. Lookup never sees either, because it never lists anything.
//
// EVERY FILE IS OPENED THROUGH os.OpenRoot, never a plain filepath.Join.
// Validate lists art/ with os.ReadDir, which follows nothing and opens no
// entry; every byte either function reads comes through a root. Art is
// community-authored content read from a path a stranger controls — a campaign
// directory today, an adventure bundle's own <adventure>/art/ under this
// sub-project's pre-flight ruling — and cmd/vtt/maps.go records what a plain
// join costs: "A community-authored pack containing a symlink at, say,
// tiles/evil.png pointing at the campaign's own SQLite file would have served
// that file's bytes to any authenticated participant." os.Root is the
// primitive that closes it: "Methods on Root will follow symbolic links, but
// symbolic links may not reference a location outside the root" (go doc
// os.Root).
//
// ABSENT ART DEGRADES; BROKEN ART REFUSES. Lookup answers ErrNotFound when
// nothing is installed under an id — including when the id is one no file
// could carry, which is what a typo in a map looks like from here — so the
// caller drops that one square to the built-in vocabulary and warns (spec §4).
// Everything else is art that exists and cannot be read, and that is a defect
// to fix rather than a square to draw plain.
//
// THE ART ROOT ITSELF IS A THIRD ANSWER, added 2026-09-03: art/ present and
// unopenable is not one piece failing, it is every piece failing, and the two
// callers want opposite verdicts on it — a boot walk refuses, a load at the
// table degrades. ErrArtDirUnreadable is how they tell it apart, and its own
// doc comment carries the ruling and the path-disclosure rule that shapes its
// message.
package artlib

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FormatVersion is the sidecar format this package understands. A sidecar
// declaring a different version is refused, not degraded (spec §3.4).
const FormatVersion int32 = 1

const (
	sidecarExt = ".json"
	pictureExt = ".png"

	// kindDoor is the one kind whose pictures are not its own name: a door has
	// an open one and a closed one (spec §3.4) and no third.
	kindDoor = "door"

	// maxArtIDLen is the longest id that can name a file: NAME_MAX is 255 on
	// the filesystems this server runs on, and the sidecar suffix spends five
	// of them. Bounding the id here rather than letting the syscall answer
	// keeps an over-long name ABSENT — a warning, per spec §4 — where a raw
	// ENAMETOOLONG refuses the whole map. A filesystem with a SHORTER limit
	// would still answer through the syscall; this bound makes the ordinary
	// case uniform, and does not claim to cover every filesystem there is.
	maxArtIDLen = 255 - len(sidecarExt)
)

// ErrNotFound is the sentinel a caller checks with errors.Is to tell absent
// art (degrade the square) from malformed art (refuse the map). Only notFound
// wraps it.
var ErrNotFound = errors.New("artlib: no such art")

// ErrArtDirUnreadable marks the one failure that is about the art ROOT rather
// than about any piece in it: art/ exists, and this process cannot open it —
// a plain file sitting where the directory should be, or a mode that forbids
// it. Every lookup against that root fails identically, so it is not "this
// piece is missing" and must not be reported as one.
//
// SEPARATE FROM ErrNotFound BECAUSE THE TWO CALLERS WANT OPPOSITE ANSWERS
// (Patrik's ruling, 2026-09-03). A boot walk should refuse loudly: an
// operator is at a terminal and can chmod the directory. A load_map at the
// table should degrade: a DM cannot act on a filesystem path from a browser,
// and a campaign that worked five minutes ago should not stop working. Giving
// each its own sentinel lets the caller decide instead of this package.
//
// NEITHER THIS ERROR NOR ANYTHING IT WRAPS NAMES A PATH. Lookup's errors
// travel verbatim to whoever issued load_map or load_adventure — an agent
// seat included (internal/gateway/authz.go grants load_adventure to RoleAgent
// as well as RoleDM) — and mapdef.LoadInstalled promises in writing that no
// error it returns carries the server's layout. os.OpenRoot's own error is an
// *fs.PathError holding the absolute path, so it is NOT wrapped; only its
// inner syscall error is, which keeps errors.Is(err, fs.ErrPermission) working
// while saying only "permission denied". Round 1 of art-is-a-flat-library
// Task 3 wrapped the whole thing, and the operator's directory layout reached
// any seat that could load an adventure whose art/ was unreadable.
var ErrArtDirUnreadable = errors.New("artlib: the art directory cannot be read")

// Piece is one art entry.
//
// HasSidecar records whether an <id>.json was read, and it exists because Kind
// cannot carry that fact: a sidecar is OPTIONAL for object art (spec §3.4), so
// a sidecar declaring no kind and no sidecar at all both leave Kind empty.
// Exit criterion 6 — "tile art without a sidecar is refused" — is decided on
// this bit by the caller that knows a square from an object.
//
// File is the picture the renderer asks for, and it is EMPTY for a door: a
// door has the two pictures its sidecar names, Open and Closed, and no third.
type Piece struct {
	ID         string
	Kind       string
	Material   string
	File       string
	Open       string
	Closed     string
	HasSidecar bool
}

// sidecar is the on-disk shape of <id>.json.
type sidecar struct {
	FormatVersion int32  `json:"format_version"`
	Kind          string `json:"kind"`
	Material      string `json:"material"`
	Open          string `json:"open"`
	Closed        string `json:"closed"`
}

// notFound is the ONLY place ErrNotFound is wrapped. why says which kind of
// absence it is, because "not installed", "no art directory" and "not a name
// any art could have" all degrade the same square and a DM fixes them
// differently.
func notFound(id, why string) error {
	return fmt.Errorf("artlib: art %q %s: %w", id, why, ErrNotFound)
}

// bareCause strips the path out of an os error, keeping only the syscall
// failure underneath: "permission denied", "is a directory", "not a
// directory". errors.Is against fs.ErrPermission and friends still answers,
// because those sentinels live on the inner error rather than on the
// *fs.PathError wrapper.
//
// EVERY OS ERROR THIS PACKAGE INTERPOLATES ON A CLIENT-REACHABLE PATH GOES
// THROUGH HERE, and the rule is
// per SYSCALL PHASE rather than per call site, because that is the axis the
// leak actually varies along. os.Root's methods do not agree about what their
// *fs.PathError.Path holds:
//
//   - openat failures (Root.OpenRoot, a path escape, a permission denial on
//     the open) set Path to the RELATIVE name.
//   - statat failures (Root.Stat, Root.Lstat) set Path to the RELATIVE name.
//   - READ failures do not. Root.Open hands back an *os.File whose Name() is
//     the joined ABSOLUTE path, so any error raised after the descriptor
//     exists — read(2) on a directory, EIO, a short read — carries the whole
//     campaign path. Root.ReadFile opens and then reads, so its error is
//     absolute exactly when the open succeeded and the read did not.
//
// That third case is why this is a helper rather than three careful format
// strings. It cost three review rounds on art-is-a-flat-library Task 3: round
// 1 checked one caller, round 2 edited a format string and left the wrapped
// *fs.PathError, round 3 covered the open phase and left the read phase. The
// reproduction was a DIRECTORY named <id>.json in art/ — openat succeeds,
// read fails EISDIR — which spec §3.1 anticipates as a thing that turns up in
// an art directory. A new os.Root call added here without this wrapper is the
// same defect a fourth time.
//
// The paths matter because these errors reach a client verbatim: mapdef's
// LoadInstalled promises no error of its own names the path it opened, and
// internal/gateway forwards load_map and load_adventure failures as-is to
// seats that include RoleAgent.
func bareCause(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// artDirUnreadable turns os.OpenRoot's failure into ErrArtDirUnreadable
// WITHOUT the path it carries. See ErrArtDirUnreadable's own doc comment for
// why the path may not survive, and bareCause above for how.
func artDirUnreadable(err error) error {
	return fmt.Errorf("%w: %w", ErrArtDirUnreadable, bareCause(err))
}

// isArtID reports whether id is a name an art file can carry: kebab-case, so
// lowercase ASCII letters and digits joined by single interior hyphens, and
// short enough to be a filename.
//
// It does the job mapdef's idIsAFilename does for maps — an id arriving from a
// map file becomes a path, so it must be one path SEGMENT and nothing else —
// and more, because art has no declared id to check the filename against. A
// separator, "." and "..", and a NUL are excluded by the character rule rather
// than by cases of their own. So is the EMPTY STRING, and it needs no clause:
// the loop never runs, prevHyphen is still true at the end, and the
// trailing-hyphen rule refuses it. Round 1 carried an `id != ""` that mapdef's
// own guard comment already called dead, and a first draft of this function
// brought it back — dropping it changes no answer, which is the test.
// Uppercase is excluded for the reason the package doc gives, and so
// is every non-ASCII letter — which would invite the same cross-platform trap
// in another alphabet, since macOS and Linux disagree about Unicode
// normalisation in filenames.
func isArtID(id string) bool {
	if len(id) > maxArtIDLen {
		return false
	}
	// A leading hyphen fails by the same rule as a doubled one.
	prevHyphen := true
	// if/else rather than a switch on purpose: gremlins scores a switch case's
	// CONDITION as NOT COVERED, so every character boundary in this guard would
	// go unevaluated by the mutation gate. Nine mutants inside this one
	// function, measured on this package's first gate run; rewriting this loop
	// took the package from 14 NOT COVERED to 5, and Validate's symlink guard
	// the rest of the way to 4. See tools/mutation-scope.md.
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			prevHyphen = false
		} else if c == '-' {
			if prevHyphen {
				return false
			}
			prevHyphen = true
		} else {
			return false
		}
	}
	// A trailing hyphen is left over in prevHyphen.
	return !prevHyphen
}

// isPictureName reports whether name is a picture in art/: an art id with the
// picture extension on it. A door's "open" and "closed" are held to this
// because they are exactly as much a path as an id is — spec §6 turns each
// into GET /api/art/{file}.
func isPictureName(name string) bool {
	stem, ok := strings.CutSuffix(name, pictureExt)
	return ok && isArtID(stem)
}

// Lookup resolves id to a Piece. It reads <dir>/<id>.json when there is one
// and confirms every picture the piece names is installed; it does no ReadDir,
// and it opens nothing outside dir. See the package doc for why both are
// load-bearing rather than oversights.
func Lookup(dir, id string) (Piece, error) {
	if !isArtID(id) {
		return Piece{}, notFound(id, "is not an art id: an art id is one kebab-case "+
			"filename in art/ — lowercase letters and digits joined by single hyphens "+
			"(design spec §3.2)")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// A campaign that has installed no art at all is ordinary, and its
			// maps still load and draw plain.
			return Piece{}, notFound(id, "is not installed: there is no art directory")
		}
		return Piece{}, artDirUnreadable(err)
	}
	defer root.Close()
	return lookupIn(root, id)
}

// lookupIn is Lookup's body with the root already open — the guard and the
// root read as one step, the resolution as another.
//
// Validate reaches this logic through the public Lookup, not through here, so
// it pays one os.OpenRoot per sidecar. That is deliberate: Validate is the rare
// path (install and boot), and the property worth having is that install and a
// map load run the SAME function rather than two that could disagree — the
// divergence hazard mapdef.LoadInstalled's own doc comment was written about.
// A shared root would save an openat per entry and buy nothing else.
func lookupIn(root *os.Root, id string) (Piece, error) {
	raw, err := root.ReadFile(id + sidecarExt)
	switch {
	case err == nil:
		return pieceFromSidecar(root, id, raw)
	case errors.Is(err, fs.ErrNotExist):
		// Two different facts arrive as ErrNotExist and must not collapse:
		// nothing is there (object art, if the picture is), versus an entry
		// that IS there and does not resolve to a file — a broken link, which
		// read as "no sidecar" and silently turned tile art into furniture.
		if _, lstatErr := root.Lstat(id + sidecarExt); lstatErr == nil {
			return Piece{}, fmt.Errorf(
				"artlib: art/%s%s: exists but does not resolve to a file", id, sidecarExt)
		}
		file := id + pictureExt
		if err := statPicture(root, id, file); err != nil {
			return Piece{}, err
		}
		return Piece{ID: id, File: file}, nil
	default:
		// bareCause, not err: this is the READ phase, and Root.ReadFile's error
		// after a successful open carries the ABSOLUTE path. The relative name
		// this message already builds is the only one a client may see.
		return Piece{}, fmt.Errorf("artlib: art/%s%s: %w", id, sidecarExt, bareCause(err))
	}
}

// pieceFromSidecar turns a sidecar's bytes into a Piece, refusing anything it
// does not fully understand.
func pieceFromSidecar(root *os.Root, id string, raw []byte) (Piece, error) {
	var sc sidecar
	dec := json.NewDecoder(bytes.NewReader(raw))
	// Unknown fields are refused for the reason mapdef's decodeStrict gives:
	// so an author gets a "you misspelled a field" error rather than silence.
	// Without it {"knd":"wall"} decodes — measured — to an empty Kind, a wall
	// that no longer says it is one, and nothing downstream can tell that from
	// a sidecar genuinely declaring no kind. It is also what refuses a sidecar
	// carrying "pack", the word spec §7 refuses in a map file.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&sc); err != nil {
		return Piece{}, fmt.Errorf("artlib: art/%s%s: %w", id, sidecarExt, err)
	}
	if sc.FormatVersion == 0 {
		return Piece{}, fmt.Errorf(
			"artlib: art/%s%s: field \"format_version\": required: this server understands "+
				"%d, and an undeclared format is not assumed to be any of them",
			id, sidecarExt, FormatVersion)
	}
	if sc.FormatVersion != FormatVersion {
		return Piece{}, fmt.Errorf(
			"artlib: art/%s%s: field \"format_version\": declares %d; this server understands %d",
			id, sidecarExt, sc.FormatVersion, FormatVersion)
	}

	p := Piece{ID: id, Kind: sc.Kind, Material: sc.Material, HasSidecar: true}
	if sc.Kind != kindDoor {
		if sc.Open != "" || sc.Closed != "" {
			return Piece{}, fmt.Errorf(
				"artlib: art/%s%s: fields \"open\" and \"closed\" belong to a door, and this "+
					"declares kind %q — two pictures are what a door has",
				id, sidecarExt, sc.Kind)
		}
		p.File = id + pictureExt
		if err := statPicture(root, id, p.File); err != nil {
			return Piece{}, err
		}
		return p, nil
	}

	if sc.Open == "" || sc.Closed == "" {
		return Piece{}, fmt.Errorf(
			"artlib: art/%s%s: a door declares both \"open\" and \"closed\"", id, sidecarExt)
	}
	// A door has TWO pictures and no third: File stays empty. Spec §3.1's own
	// listing carries cellar-door.json, cellar-door-open.png and
	// cellar-door-closed.png, and no cellar-door.png.
	for _, named := range []struct{ field, name string }{
		{"open", sc.Open}, {"closed", sc.Closed},
	} {
		if !isPictureName(named.name) {
			return Piece{}, fmt.Errorf(
				"artlib: art/%s%s: field %q: %q is not a picture in art/ — one kebab-case "+
					"name ending %s, because this string becomes a path (spec §6 serves it "+
					"as GET /api/art/{file})",
				id, sidecarExt, named.field, named.name, pictureExt)
		}
		if err := statPicture(root, id, named.name); err != nil {
			return Piece{}, err
		}
	}
	p.Open, p.Closed = sc.Open, sc.Closed
	return p, nil
}

// statPicture confirms name is an installed picture inside root. An absent one
// is absence — the whole piece degrades, because half-installed art draws a
// square nothing can serve. Any OTHER stat failure is NOT absence: discarding
// it made a symlink loop indistinguishable from a missing file.
func statPicture(root *os.Root, id, name string) error {
	info, err := root.Stat(name)
	switch {
	case err == nil:
		if info.IsDir() {
			return fmt.Errorf("artlib: art/%s: picture %s is a directory", id, name)
		}
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return notFound(id, fmt.Sprintf("has no picture %q installed", name))
	default:
		// Root.Stat's own error is already relative; bareCause is applied
		// anyway so the rule is "every os error in this package", which is one
		// a reader can check by grepping rather than one that needs the
		// per-method table in bareCause's doc comment to be re-derived.
		return fmt.Errorf("artlib: art/%s: picture %s cannot be read: %w", id, name, bareCause(err))
	}
}

// Validate is the directory-shaped check: it walks art/ once and refuses what
// no lookup can ever see — a subdirectory, a symlink, a filename no map could
// spell — then resolves every sidecar it finds through Lookup itself, so a
// piece that install accepts is a piece a map load accepts. It is meant for
// `vtt art install` and server start, the rare paths (spec §5); a map load
// uses Lookup and never this.
//
// An absent art/ passes, because a campaign with no art yet is ordinary —
// refusing it would reproduce sub-project 15's boot-order defect, where a
// guard on one directory silently gated the loading of another. A path that
// exists and is not a directory is a different fact and is refused.
//
// Errors from the per-sidecar resolution may wrap ErrNotFound: a sidecar with
// no picture beside it (spec §3.4's error) IS absent art, seen from the
// directory rather than from a map. Validate's own contract is only err or
// nil.
func Validate(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		// The ONE exemption from bareCause's rule, stated here so the rule
		// survives its own grep. os.ReadDir is not an os.Root method and has
		// no phase table; more to the point Validate is OPERATOR-facing by
		// design — every message it returns names dir on purpose, because the
		// only caller is a boot check with an operator at a terminal. If
		// Validate ever answers a request, this line becomes a leak.
		return fmt.Errorf("artlib: read art dir %s: %w", dir, err)
	}
	for _, e := range entries {
		name := e.Name()
		// Type() is the entry's OWN type, not its target's, which is the point:
		// a symlink to a directory answers IsDir() == false, so `ln -s` walked
		// straight past a subdirectory check and re-created the pack. A symlink
		// to a file is refused by the same rule, because it is a second name
		// for one piece and §3.3 says a piece has exactly one.
		//
		// This runs on EVERY entry, before the "is it art at all" filter below,
		// so a symlinked .DS_Store is refused where a real one is ignored. That
		// asymmetry is deliberate: the filter asks what a file IS to this
		// package, and the link rule asks whether art/ owns it at all.
		//
		// if rather than a switch, for the reason isArtID gives: gremlins
		// scores a switch case's condition NOT COVERED, and this comparison is
		// the whole of the symlink refusal.
		typ := e.Type()
		if typ&fs.ModeSymlink != 0 {
			return fmt.Errorf(
				"artlib: art dir %s contains a symlink %q; art/ holds files — a link is a "+
					"second name for one piece, and a link to a directory is a subfolder "+
					"in disguise", dir, name)
		}
		if typ.IsDir() {
			return fmt.Errorf(
				"artlib: art dir %s contains a subdirectory %q; art/ is flat — "+
					"a subfolder is a namespace, and a namespace is a pack", dir, name)
		}

		// Art is a picture and an optional sidecar. Anything else in the
		// directory belongs to whoever put it there — a .DS_Store must not
		// stop a campaign booting — but a name that only LOOKS like art is
		// refused, because a case-insensitive filesystem hands y.JSON to a
		// lookup that a case-sensitive one never would.
		ext := filepath.Ext(name)
		if !strings.EqualFold(ext, sidecarExt) && !strings.EqualFold(ext, pictureExt) {
			continue
		}
		stem := strings.TrimSuffix(name, ext)
		if ext != strings.ToLower(ext) || !isArtID(stem) {
			return fmt.Errorf(
				"artlib: art dir %s: %q is not an art filename: a kebab-case stem and a "+
					"lowercase %s or %s, so the stem IS the id a map names (design spec §3.2)",
				dir, name, pictureExt, sidecarExt)
		}
		if ext == sidecarExt {
			// Resolved through Lookup itself, not a second check that could
			// disagree with it: what install accepts, a map load accepts.
			if _, lookupErr := Lookup(dir, stem); lookupErr != nil {
				return fmt.Errorf("artlib: art dir %s: %w", dir, lookupErr)
			}
		}
	}
	return nil
}
