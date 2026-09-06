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
// and `vtt art install` is what catches it.
//
// THE NEXT BOOT NO LONGER CATCHES IT, and this sentence said it did until
// 2026-09-03 (review finding F1). Since art-is-a-flat-library Task 4,
// composeServer runs Validate at start and REPORTS what it finds rather than
// refusing (Patrik's severity ruling that day), so a Masonry-1.png dropped in
// by hand is now NAMED in the boot log and then renders anyway on this machine
// — handing the renderer masonry-1.png, which is not on disk — while the same
// campaign draws that square plain on a case-sensitive server. A boot that
// mentions a defect is not a boot that closes it.
//
// UNIQUENESS IS THE FILESYSTEM'S (spec §3.3), which is why art/ is flat.
// art/a/x.png and art/b/x.png coexist happily, and the moment they can,
// something has to decide what "x" means. Validate is what refuses a
// subdirectory — and a symlink, which is the same thing wearing a disguise:
// fs.DirEntry.IsDir() reads the entry's own type, so a symlink to a directory
// answers false. Lookup never sees either, because it never lists anything.
//
// MAPTOOL SOLVES THIS SAME PROBLEM AND ARRIVES AT THE SAME SHAPE FROM THE
// OTHER END (read 2026-09-05, CLAUDE.md rule 9, from
// net/rptools/maptool/model/AssetManager.java). Its asset cache is ONE FLAT
// DIRECTORY: getAssetCacheFile is cacheDir/<id> and getAssetInfoFile is
// cacheDir/<id>.info, a properties file beside the picture holding its name and
// type — a picture plus a sidecar, keyed by stem, with no manifest listing what
// is installed and no subdirectory anywhere. searchForImageReferences walks
// whatever directory tree a user points it at and imports what it finds INTO
// that flat cache, so folders are a browsing convenience and never part of an
// identity. sanitizeAssetId canonicalises the resolved path and refuses
// anything outside the cache, which is what os.OpenRoot does for us.
//
// THE ONE DIVERGENCE IS THE ID ITSELF, and it is deliberate: theirs is the MD5
// of the picture's bytes and the human name is a field in the sidecar, where
// ours is the filename and nothing declares a name at all. Content-addressing
// buys them de-duplication of identical bytes and rename-safety for free, and
// costs what this platform cannot pay: a map file here is JSON that a DM or an
// LLM writes by hand, and `"overrides": {"0,1": "masonry-1"}` is a sentence
// either can read where a 32-character digest is not. It also means retouching
// a picture would change its id and orphan every reference to it, which is the
// opposite of spec §3.3's "cp over it and reload". What we give up is real and
// worth naming: two copies of the same picture under two names are two pieces
// here, and renaming a file breaks every map that names it.
//
// A campaign's assets travel INSIDE the .cmpgn zip there, which is the same
// answer this sub-project's pre-flight ruling reached for an adventure bundle's
// own <adventure>/art/ — art that ships with the thing you hand someone.
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
// ALMOST EVERYTHING DEGRADES; ONE THING REFUSES. This package returns an error
// for every failure and decides nothing; three sentinels are what let the
// caller decide, and since Patrik's ruling of 2026-09-04 the split runs
// one-against-the-rest rather than absent-against-broken:
//
//   - ErrNotFound: nothing is installed under an id — including an id no file
//     could carry, which is what a typo in a map looks like from here. The
//     caller drops that one square to the built-in vocabulary and warns
//     (spec §4).
//   - ErrArtDirUnreadable: art/ present and unopenable. Not one piece failing,
//     every piece failing, and the two callers want opposite verdicts — a boot
//     walk refuses, a load at the table degrades.
//   - ErrFormatVersion: the sidecar declares a LATER format than this server
//     understands. The one art failure that still refuses a map, because it is
//     a fact about the SERVER rather than about the file.
//   - EVERYTHING ELSE — a missing brace, a truncated copy, a wrongly typed
//     field, a format_version that is not a version number or is BELOW the one
//     this server understands, a door naming one picture, a picture that is a
//     directory — is a broken file, and the caller degrades that one square
//     with a warning naming the piece and the cause.
//
// THE LAST BULLET USED TO SAY THE OPPOSITE, and the sentence it replaces —
// "art that exists and cannot be read is a defect to fix rather than a square
// to draw plain" — was true of this package and catastrophic at the caller:
// composeServer turns any map-load error into a refusal to start, so one
// truncated sidecar named by one committed map stopped the server booting with
// every other map fine (measured 2026-09-03, exit status 1). Each sentinel's
// own doc comment carries its ruling, and ErrArtDirUnreadable's carries the
// path-disclosure rule that shapes every message here.
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
// DECLARING A LATER NUMBER is the single art failure that is still refused
// rather than degraded (spec §4, Patrik 2026-09-04) — see ErrFormatVersion.
// The other three things the field can be — absent, holding something that is
// not a version number at all, or declaring a number BELOW this one — are
// broken files and degrade with them; see pieceFromSidecar, which is where all
// four are told apart. It said "a different number" until
// art-is-a-flat-library Task 8, and that spelling is what gave a typo'd 0 the
// refusal reserved for the future.
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

// ErrNotFound is the sentinel a caller checks with errors.Is to tell absent art
// from art that is installed and does not resolve. BOTH degrade the square
// since 2026-09-04; what differs is the sentence the DM is told, and that is
// the whole reason the distinction is still worth a sentinel — "not installed"
// sends someone hunting for a file that is sitting in art/. Only notFound
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

// ErrFormatVersion marks the ONE sidecar failure that still refuses a map
// (Patrik's ruling, 2026-09-04): the file declares a format_version LATER than
// the one this server understands.
//
// "LATER", NOT "DIFFERENT", and the difference is a whole campaign. The rule
// was written as `!= FormatVersion` until art-is-a-flat-library Task 8, so a
// typo'd `"format_version": 0` took this sentinel, mapdef.Resolve refused the
// map, and composeServer refused the BOOT for every seat — over a digit, with
// the message telling an operator that their file is from the future. Nothing
// below this server's own version can be content this server is too old for;
// pieceFromSidecar degrades those with the broken files.
//
// IT IS NOT A CLAIM THAT THE FILE IS BROKEN. Every other unreadable sidecar —
// a missing brace, a truncated copy, a door that names one picture — degrades
// that one square with a warning, because the remedy is to fix the file and a
// campaign must not lose its server over one of them. This says the opposite
// thing: the content is NEWER THAN THIS SERVER, and the remedy is a newer
// server. Degrading it would turn a v2 art set into hundreds of plain squares
// and a wall of warnings reading "my art is broken", when the diagnosis is
// "this server is too old". One refusal naming both versions says that; ninety
// warnings do not.
//
// THREE OTHER THINGS THE FIELD CAN BE ARE NOT THIS, and all three degrade. A
// sidecar with NO format_version at all: an absent field does not assert that
// the content is newer, it asserts that somebody hand-wrote a file and left a
// line out — the same class as a missing brace, and refusing it would leave the
// measured defect this ruling exists to remove half alive. A field holding
// something that is NOT A VERSION NUMBER — "2", 1.0, 2.5, null, an integer
// past int32: this server cannot read it as a version at all, so it cannot be
// reading a later one. Every plausible spelling of a later format is a plain
// JSON integer, which is what makes it safe to leave both of those with the
// broken files. And a number BELOW this server's own — 0, -1: a version no
// server has ever written, which is a typo wearing a version's clothes.
//
// THE SENTINEL IS THE INTERFACE. mapdef.Resolve branches on errors.Is, never
// on message text, so this var is what carries the ruling across the package
// boundary. See pieceFromSidecar for why the version is read in a pass of its
// own before the strict decode.
var ErrFormatVersion = errors.New("artlib: unsupported sidecar format")

// Piece is one art entry.
//
// HasSidecar records whether an <id>.json was read, and it exists because Kind
// cannot carry that fact: a sidecar is OPTIONAL for object art (spec §3.4), so
// a sidecar declaring no kind and no sidecar at all both leave Kind empty.
// Exit criterion 6 — tile art without a sidecar — is decided on this bit by the
// caller that knows a square from an object. The criterion read "is refused"
// until 2026-09-03, when Patrik ruled that square degrades with its own
// warning; this bit is what the caller needs either way.
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

// declaredFormat is the version pre-pass's probe: one field, read on its own,
// before the strict decode below ever runs. See pieceFromSidecar for why that
// ordering is load-bearing.
//
// IT IS A NAMED, PACKAGE-LEVEL TYPE BECAUSE ITS NAME REACHES A DM. When this
// probe was an anonymous struct, encoding/json spelled the WHOLE STRUCT
// LITERAL — field name, Go type and json tag — into its own error for a
// sidecar holding a JSON array, and that error travels verbatim to whoever
// issued load_map. It told them this server's sidecar type has exactly one
// field, which is false of the sidecar type and is nobody's business either
// way. Named, the same error says artlib.declaredFormat: true, and no more
// than the spec already says. (Review finding F3, 2026-09-05.)
//
// json.RawMessage RATHER THAN int32, for the reason mapJSON.Pack is one
// (internal/mapdef/load.go, art-is-a-flat-library Task 5): a Go zero value
// cannot carry PRESENCE. A plain int32 here cannot tell an ABSENT field from
// an explicit {"format_version": 0}, so the rule this package documents —
// undeclared degrades, declared-and-unknown refuses — was implemented as
// zero-versus-non-zero, and a file that DID declare a version was told "an
// undeclared format is not assumed to be any of them" (review finding F4,
// 2026-09-05). A *int32 fixes that one case and leaves the next: JSON null
// unmarshals into a pointer as nil, so {"format_version": null} would read as
// absent. Raw bytes decide nothing until pieceFromSidecar decides.
type declaredFormat struct {
	FormatVersion json.RawMessage `json:"format_version"`
}

// sidecar is the on-disk shape of <id>.json.
//
// FormatVersion IS DECODED AND NEVER READ, and it must stay: since
// pieceFromSidecar reads the version through declaredFormat in a pass of its
// own, this field's only remaining job is to keep DisallowUnknownFields from
// refusing "format_version" as an unknown field. Delete it as dead and every
// sidecar in every campaign stops parsing —
// TestTileArtDeclaresItsNature fails with `json: unknown field
// "format_version"`, measured 2026-09-04.
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

// unsupportedFormat is the ONLY place ErrFormatVersion is wrapped, for the
// reason notFound is the only place ErrNotFound is: a caller decides what to
// do by errors.Is, so a second construction site is a second thing that can
// forget the sentinel and silently turn a refusal into a degrade.
//
// It names BOTH versions because that is the entire content of the message.
// The remedy is a newer server, and "this art is broken" does not say which
// one to go and get.
//
// declared IS ALWAYS GREATER THAN FormatVersion at the one call site, which is
// what makes that remedy true of every message this builds. The MESSAGE stays
// neutral about the direction anyway — campaigncfg.ErrFormatVersion's doc
// records the same choice for the same file one directory over — because the
// two versions are the fact, and a sentence about who is behind is an
// inference the reader can draw from them.
func unsupportedFormat(id string, declared int32) error {
	return fmt.Errorf(
		"artlib: art/%s%s: field \"format_version\": declares %d; this server understands %d: %w",
		id, sidecarExt, declared, FormatVersion, ErrFormatVersion)
}

// clip bounds a fragment of author-controlled sidecar text before it is
// interpolated into a message, and makes it safe to put on the wire.
//
// BOUNDED, because those messages ride back to whoever issued load_map on a
// CommandResult and a sidecar may hold a value of any length — spec §4's own
// measurement is a load whose warnings did not arrive at all because they
// exceeded the client read limit.
//
// AND VALID UTF-8, which a byte count alone does not give:
// CommandResult.warnings is a proto3 repeated string, proto3 strings must be
// valid UTF-8, and there are two ways raw campaign bytes would not be. A
// json.RawMessage keeps the file's bytes verbatim, so a sidecar written in some
// other encoding carries whatever it carries; and cutting at a fixed byte
// offset splits a multi-byte rune in half — {"format_version":
// "üüüüüüüüüüüüüüüüüüüüü"} is enough, measured. Either one makes protojson
// refuse to marshal the frame, which is a campaign file deciding that a
// load_map answer never arrives at all. Both are handled below by the same
// call, run twice.
//
// IT IS NOT THE ONLY SITE THAT INTERPOLATES RAW CAMPAIGN BYTES, and an earlier
// draft of this comment said it was. mapdef's pack refusal (load.go, the
// `raw.Pack != nil` arm) puts a map file's own bytes into an error that reaches
// CommandResult.error, which is the same proto3 string rule on the other
// channel, and it is neither bounded nor validated. That is older than this
// function and belongs to whoever owns that refusal; it is recorded here rather
// than fixed, because a comment claiming uniqueness is how the second site
// stops being looked for.
// TWO ToValidUTF8 PASSES AND NO HAND-ROLLED SCAN, which is the shape the
// mutation gate argued this into. The first draft backed up over continuation
// bytes with `for cut > 0 && !utf8.RuneStart(s[cut]) { cut-- }`, and the gate
// answered with three survivors: `cut > 0` is unreachable-different, because
// valid UTF-8 backs up at most three bytes and s[0] is always a rune start, so
// the guard was dead code that only a panic could have distinguished. The
// second pass says the same thing with no boundary to get wrong — a cut that
// splits a rune leaves bytes that are not valid UTF-8, and an EMPTY
// replacement drops exactly those.
func clip(raw []byte) string {
	const limit = 40
	s := strings.ToValidUTF8(string(raw), "\uFFFD")
	if len(s) <= limit {
		return s
	}
	return strings.ToValidUTF8(s[:limit], "") + "…"
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
	return isArtNameWithExt(name, pictureExt)
}

// isArtNameWithExt is the shape both name rules are made of: strip exactly one
// known extension and hold what is left to isArtID. One helper rather than two
// copies of the same two lines, because the two rules must not be able to
// disagree about what a stem is — the day isArtID changes, both move.
func isArtNameWithExt(name, ext string) bool {
	stem, ok := strings.CutSuffix(name, ext)
	return ok && isArtID(stem)
}

// IsArtFileName reports whether name is ONE FLAT ART FILENAME: an art id
// carrying the picture or the sidecar extension, and nothing else.
//
// EXPORTED FOR THE ROUTE, and that is its whole reason to exist.
// GET /api/art/{file} (internal/gateway/metadata.go) hands a browser raw bytes
// out of a directory an operator installed, and design spec §3.1/§3.3 say art/
// is FLAT. os.OpenRoot CONFINES WITHOUT FLATTENING — "art/pack-ish/x.png" is
// legitimately inside the root, and fs.ValidPath rejects only ".." — so nothing
// underneath the route stops a subdirectory being served. The pack route it
// replaces was saved from that only by net/http's single-segment {file}
// wildcard not matching across "/", which nobody had to think about, because a
// pack WAS a directory.
//
// So the route checks the NAME, here, against the same rule Lookup resolves by,
// rather than writing a second one in the gateway that could drift from this
// one. Everything a subdirectory needs — a "/" — fails isArtID, and so does
// "..", an absolute path, a dotfile and every uppercase spelling.
//
// TWO EXTENSIONS AND NO MORE. A picture is what a browser draws; the sidecar is
// what a client reads to learn which of a door's two pictures to ask for, the
// same order lookupIn resolves in. Anything else in art/ — a .DS_Store, a
// stray .svg, a README — is not art and is not this route's to hand out; an SVG
// in particular can embed <script>, and a same-origin script reads this
// client's Bearer token out of localStorage.
func IsArtFileName(name string) bool {
	return isArtNameWithExt(name, pictureExt) || isArtNameWithExt(name, sidecarExt)
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
//
// IT READS THE VERSION IN A PASS OF ITS OWN, before the strict decode, and
// that ordering is load-bearing rather than tidy. Since Patrik's ruling of
// 2026-09-04 the two failures have opposite verdicts at the caller — a sidecar
// this server cannot read degrades one square, a DECLARED version it does not
// understand refuses the map — and a single strict decode reports those two in
// the wrong order for the case the ruling exists for. A real v2 sidecar carries
// FIELDS, and DisallowUnknownFields fires on the first of those before any
// version check runs: measured on {"format_version":2,"kind":"wall",
// "variants":["mossy"]}, one strict decode answers `json: unknown field
// "variants"` and never mentions the version at all. The whole v2 art set
// would then degrade as a wall of parse warnings while a bare
// {"format_version":2} refused — which is the outcome the refusal was kept to
// prevent, arrived at through the decoder instead of the rule.
//
// The first pass is LENIENT ABOUT EVERY OTHER FIELD and about nothing else: it
// decodes into declaredFormat, which has only the one field, so an unknown or
// wrongly typed sibling is skipped rather than reported. Nothing is accepted on
// the strength of it — every field a Piece is built from comes from the strict
// decode below.
//
// IT IS A json.Decoder AND NOT json.Unmarshal, which looks like a stylistic
// choice and is not. Unmarshal REFUSES TRAILING BYTES and Decode ignores them,
// so the obvious spelling of this pre-pass silently narrowed what a sidecar may
// be: measured, `{"format_version":1,"kind":"wall"} SURPRISE` and two
// concatenated objects both resolved before this pass existed and began
// degrading with "invalid character after top-level value" after it. This pass
// exists to REORDER two reports, not to change which files are accepted, and a
// tightening nobody decided is the kind that ships unnoticed — the more so
// here, because it would have made artlib stricter than mapdef.decodeStrict,
// which still ignores trailing data in a map file. Whether trailing bytes
// should be refused is a real question and a separate one; it belongs to the
// map format and the sidecar format together, not to a version pre-pass.
// TestTrailingBytesAfterASidecarAreIgnoredAsTheyAlwaysWere is the guard.
// (Review finding F2, 2026-09-05.)
func pieceFromSidecar(root *os.Root, id string, raw []byte) (Piece, error) {
	var declared declaredFormat
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&declared); err != nil {
		return Piece{}, fmt.Errorf("artlib: art/%s%s: %w", id, sidecarExt, err)
	}
	if len(declared.FormatVersion) == 0 {
		// NO SENTINEL, on purpose: an absent field does not assert that the
		// content is newer than this server, so this degrades with the missing
		// braces rather than refusing with the later formats. See
		// ErrFormatVersion's own doc comment.
		return Piece{}, fmt.Errorf(
			"artlib: art/%s%s: field \"format_version\": required: this server understands "+
				"%d, and an undeclared format is not assumed to be any of them",
			id, sidecarExt, FormatVersion)
	}
	// A POINTER, so an explicit null is told apart from a number: JSON null
	// unmarshals into any pointer as nil without erroring, so a plain int32
	// here would read {"format_version": null} as 0 and refuse it as "declares
	// 0" — a version the author never wrote.
	var version *int32
	if err := json.Unmarshal(declared.FormatVersion, &version); err != nil || version == nil {
		// DEGRADES, and it is one of three answers this field can give that
		// are not the refusal. That refusal is for a LATER version; a value
		// that is not a version AT ALL — "2", 1.0, 2.5, null, an integer past
		// int32 — is a broken file, and a broken file draws its square plain
		// like every other broken file (Patrik, 2026-09-04). Every plausible
		// spelling of a LATER format is a plain JSON integer, so nothing this
		// arm catches is the case the refusal exists for.
		return Piece{}, fmt.Errorf(
			"artlib: art/%s%s: field \"format_version\": %s is not a version number; this "+
				"server understands %d", id, sidecarExt, clip(declared.FormatVersion), FormatVersion)
	}
	// TWO ARMS RATHER THAN ONE `!= FormatVersion`, and here the DIRECTION
	// decides the verdict rather than only the sentence — which is what makes
	// this split sharper than campaigncfg.Load's, where both arms refuse.
	//
	// A single arm shipped until art-is-a-flat-library Task 8 and gave a
	// typo'd `"format_version": 0` the refusal reserved for content from the
	// future: mapdef.Resolve returned the error, and composeServer turned it
	// into a refusal to BOOT when a committed map named that piece — the whole
	// campaign down, for everyone, over a mistyped digit whose remedy is to fix
	// the file and not to fetch a newer server. It is the same absent-versus-
	// zero shape mapJSON.Pack and campaigncfg's two fields were fixed for on
	// this same branch, and it was harmless only while no real sidecar existed.
	//
	// The boundary is killable in both directions: at *version == FormatVersion
	// a `>=` here refuses a sidecar this server understands, and a `<=` below
	// degrades one, and every test that resolves a v1 piece says so.
	if *version > FormatVersion {
		return Piece{}, unsupportedFormat(id, *version)
	}
	if *version < FormatVersion {
		// DEGRADES. A version BELOW the one this server understands is not
		// content this server is too old for, so no sentinel: 0 and -1 are what
		// a hand-written file says when somebody typed the field and not the
		// number, and no server has ever written either.
		//
		// WHEN A SECOND VERSION EXISTS this arm becomes a real compatibility
		// question rather than a typo report — a v1 sidecar read by a v2 server
		// is a file this server could very likely still read. Deciding that is
		// the job of whoever bumps FormatVersion; degrading is the answer that
		// keeps the campaign booting until they do.
		return Piece{}, fmt.Errorf(
			"artlib: art/%s%s: field \"format_version\": declares %d; this server understands "+
				"%d, and %d is not a format anything ever wrote — check the file",
			id, sidecarExt, *version, FormatVersion, *version)
	}

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
	// A DOOR MAY NAME THE SAME PICTURE TWICE, and that is a ruling rather than
	// a gap (art-is-a-flat-library Task 8 review, finding F4). Nothing below
	// compares Open against Closed, so a door that looks shut when opened
	// installs, validates and resolves without a word.
	//
	// THE CHECK WOULD BE SYNTAX WEARING JUDGEMENT'S CLOTHES. What an author gets
	// wrong is that the two pictures LOOK the same; what == sees is that they
	// are SPELLED the same. `cp cellar-door-closed.png cellar-door-open.png` is
	// the likelier mistake — it is one command — and it leaves two different
	// names over identical bytes, which such a check passes. So it would refuse
	// one spelling of the error, say nothing about the other, and read as
	// coverage of both. Comparing the BYTES is the check that would work, and it
	// is not this package's business: artlib decides what a piece IS, never what
	// a picture shows (CLAUDE.md rule 5's line, applied to pixels).
	//
	// RULE 9, MapTool: it cannot even ask the question. A token's several
	// pictures live in Token.imageAssetMap keyed by state name and valued by
	// MD5Key, and net.rptools.lib.MD5Key is a digest OF THE BYTES — so two
	// states showing one picture hold the same key by construction, and
	// Token.getAllImageAssets collapses them into a HashSet without comment.
	// Fifteen years of tables produced no such refusal because content-addressing
	// makes the condition invisible.
	//
	// AND IT IS SOMETIMES MEANT: an archway or a threshold that blocks movement
	// and sight while looking the same either way, or a placeholder pointing
	// both states at one picture until the second is drawn. Refusing costs those
	// authors their door; permitting costs the mistaken author a warning.
	//
	// A WARNING WAS THE THIRD OPTION and was not taken. mapdef.Resolve has the
	// channel for it, but it would carry the same defect one register quieter —
	// silent on the copied file, loud on the spelled-alike one — and a warning
	// that fires on the rarer half of a mistake teaches the wrong lesson about
	// what is checked.
	//
	// WHAT DOES GUARD IT is narrower and says so:
	// TestTheShippedArtResolvesThroughThisPackage requires door.Open !=
	// door.Closed for campaigns/example alone, because that campaign is a
	// fixture this repo owns and its door demonstrably opens. The permission
	// itself is pinned by TestADoorMayNameTheSamePictureForBothStates, so
	// whoever decides to refuse it later has to move a test and read this.
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
//
// IT REPORTS EVERY PROBLEM IT FINDS, NOT THE FIRST, and that is a deliberate
// change made at Task 4 of art-is-a-flat-library — Task 1 wrote this to return
// at its first finding. Patrik's severity ruling of 2026-09-03 is what forced
// it: cmd/vtt's composeServer runs this walk at boot, reports what it says, and
// starts the server ANYWAY, because a campaign with two hundred good pieces and
// one Masonry-1.png copied off a Windows box must not fail to boot over a file
// no map has ever named. Under an early return, an operator with three mistakes
// pays three boots to hear about them. The answer is one error per problem,
// joined with errors.Join, so the message carries them all and errors.Is still
// reaches each one. `vtt art install` reads it as the single refusal it always
// was: non-nil is non-nil.
//
// A FINDING HERE IS NOT A PROMISE ABOUT WHAT RENDERS, and the first version of
// Task 4's prose said it was: "each broken piece is refused when a map names
// it" was written in four places and is false in three of the five arms
// (measured 2026-09-03, review finding F1). What a map load actually does with
// each finding:
//
//   - A SIDECAR THAT DECLARES A LATER format_version THAN THIS SERVER
//     UNDERSTANDS: REFUSES the map. Since Patrik's ruling of 2026-09-04 this
//     arm alone is what the retired claim was true of; it named "a sidecar that
//     cannot be parsed" alongside, and that half moved to the line below. It
//     read "a format_version this server does not understand" until
//     art-is-a-flat-library Task 8, which was true of a typo'd 0 as well and so
//     described a refusal 0 no longer gets.
//   - A SIDECAR THAT CANNOT BE PARSED, ONE DECLARING A VERSION BELOW THIS
//     SERVER'S, AN ORPHAN SIDECAR, A SUBDIRECTORY, and a
//     wrong-cased name on a CASE-SENSITIVE filesystem: the square DRAWS PLAIN
//     and the DM gets a warning (spec §4) — from ErrNotFound for the last
//     three, and from the ordinary-error arm for the unparseable one, which
//     gets a different sentence because the file is sitting right there. A
//     subdirectory takes its own stem down with it and its contents are
//     unreachable, which is flatness holding — but it is a degrade, not a
//     refusal.
//   - A RELATIVE SYMLINK, and a wrong-cased name on a CASE-INSENSITIVE
//     filesystem: RENDERS, with nothing anywhere objecting. `ln -s aaa-good.png
//     linky.png` resolves through Root.Stat because its target is inside the
//     root — an ABSOLUTE link is refused as an escape, which is exactly why the
//     relative one is the shape that matters. Masonry-1.png resolves for
//     "masonry-1" on macOS and hands back a File that is not on disk.
//
// FOR THAT LAST PAIR THIS REPORT IS THE ONLY NOTICE ANYONE EVER GETS. Whether a
// wrong-cased filename should be a narrow boot refusal on those grounds — it is
// the one finding that changes what RENDERS, and changes it differently per
// platform — is Patrik's call and was open on 2026-09-03.
//
// THE ReadDir FAILURE IS STILL ALONE, because there is no walk after it: the
// directory itself could not be listed, so there are no entries to have
// problems. composeServer catches that condition before this runs anyway
// (cmd/vtt's artRootIsOpenable, which REFUSES the boot on it — an unopenable
// root is every piece failing, not one).
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
	var problems []error
	for _, e := range entries {
		if problem := entryProblem(dir, e); problem != nil {
			problems = append(problems, problem)
		}
	}
	// errors.Join returns nil for an empty slice, so a clean directory still
	// answers nil rather than a non-nil error wrapping nothing.
	return errors.Join(problems...)
}

// entryProblem is what is wrong with one entry of art/, or nil.
//
// ONE ENTRY CONTRIBUTES AT MOST ONE PROBLEM, AND THAT IS STRUCTURAL HERE — the
// return type says it, so an arm appended below cannot break it. The first
// collecting version of Validate held these arms inline in its loop, where the
// same invariant was POSITIONAL: every arm but the last needed a `continue` and
// the last one needed none, so appending an arm would have made the previous
// last arm fall through into it, silently, with no gate able to see it (review
// finding F4, 2026-09-03). The count an operator reads is the number of files
// to go and fix, and it stays that way by construction.
func entryProblem(dir string, e fs.DirEntry) error {
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
		return nil
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
	return nil
}
