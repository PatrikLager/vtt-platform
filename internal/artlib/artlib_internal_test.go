package artlib

// artlib_internal_test.go pins one property no black-box test can reach:
// bareCause's behaviour when the error it is handed is NOT an *fs.PathError.
//
// Every os.Root method this package calls returns one, so that arm is
// unreachable through Lookup and Validate — which is exactly why it needs a
// test rather than a deletion. It is the arm that decides what happens when a
// future os.Root method, or a stdlib change, hands back an error shaped
// differently: the answer must be "pass it through", not "return nil" and not
// "panic". A silent nil here would turn an error into a success at the call
// sites in lookupIn and statPicture.

import (
	"errors"
	"io/fs"
	"testing"
)

func TestBareCausePassesThroughAnErrorThatCarriesNoPath(t *testing.T) {
	plain := errors.New("something the syscall layer did not shape as a PathError")
	if got := bareCause(plain); !errors.Is(got, plain) {
		t.Fatalf("bareCause(%v) = %v, want the error itself — an unrecognised shape must "+
			"travel on intact, never become nil", plain, got)
	}
}

func TestBareCauseKeepsTheReasonAndDropsThePath(t *testing.T) {
	wrapped := &fs.PathError{Op: "read", Path: "/var/campaigns/secret/art/masonry-1.json",
		Err: fs.ErrPermission}
	got := bareCause(wrapped)
	if !errors.Is(got, fs.ErrPermission) {
		t.Fatalf("bareCause = %v, want it to still answer errors.Is(fs.ErrPermission): "+
			"dropping the path must not drop the reason", got)
	}
	if got.Error() != fs.ErrPermission.Error() {
		t.Fatalf("bareCause = %q, want exactly the inner error's text and nothing of the path",
			got.Error())
	}
}
