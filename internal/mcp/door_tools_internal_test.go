package mcp

import "testing"

// httpOriginFrom is the one piece of #45 that is pure arithmetic on a string,
// and the one most likely to be subtly wrong: scheme, port, path and query all
// have to be handled, and getting any of them wrong produces a tool that
// silently talks to the wrong place — or to nothing, which an agent reports to
// its user as "the table has no join link".
//
// INTERNAL, because the function is unexported and this is arithmetic rather
// than behaviour. The tools that use it are tested through the real server.
func TestHTTPOriginFromAWebSocketURL(t *testing.T) {
	for _, c := range []struct {
		name, in, want string
		wantErr        bool
	}{
		{name: "ws becomes http", in: "ws://localhost:8080/ws", want: "http://localhost:8080"},
		{name: "wss becomes https", in: "wss://table.example/ws", want: "https://table.example"},
		{
			// The URL the CLI actually passes carries a token in the query,
			// and carrying it forward would put a credential in every
			// subsequent request line — where it reaches logs and proxies,
			// which is the whole reason the Authorization header exists.
			name: "the token query is dropped",
			in:   "ws://localhost:8080/ws?token=secret-token&after=0",
			want: "http://localhost:8080",
		},
		{
			// A non-default port must survive: a table served on 9000 and
			// asked on 80 fails in a way that reads as "the server is down".
			name: "a non-default port survives",
			in:   "ws://127.0.0.1:9999/ws",
			want: "http://127.0.0.1:9999",
		},
		{
			// No port at all is legitimate (a table behind a reverse proxy on
			// 443) and must not gain one.
			name: "no port stays portless",
			in:   "wss://table.example/ws",
			want: "https://table.example",
		},
		{
			// A scheme this cannot translate must ERROR rather than be passed
			// through. Returning "http://..." for an ftp:// input would send
			// the token somewhere nobody intended.
			name: "an untranslatable scheme is refused", in: "ftp://host/ws", wantErr: true,
		},
		{name: "an unparseable url is refused", in: "://nonsense", wantErr: true},
		{name: "empty is refused", in: "", wantErr: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := httpOriginFrom(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("httpOriginFrom(%q) = %q, want an error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("httpOriginFrom(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("httpOriginFrom(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
