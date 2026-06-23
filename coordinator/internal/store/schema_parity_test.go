package store

import (
	"context"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Eshpelin/calltoverify/coordinator/migrations"
)

var (
	checkRe = regexp.MustCompile(`CHECK \((\w+) IN \(([^)]*)\)\)`)
	valRe   = regexp.MustCompile(`'([^']*)'`)
)

// enumCheckSet returns the set of enum CHECK constraints in a schema, each
// normalized to "column:sorted,values". Keying by the full signature (not just
// the column) keeps the three distinct `status` enums separate.
func enumCheckSet(schema string) map[string]bool {
	set := map[string]bool{}
	for _, m := range checkRe.FindAllStringSubmatch(schema, -1) {
		var vals []string
		for _, v := range valRe.FindAllStringSubmatch(m[2], -1) {
			vals = append(vals, v[1])
		}
		sort.Strings(vals)
		set[m[1]+":"+strings.Join(vals, ",")] = true
	}
	return set
}

// TestSchemaEnumParity fails if the enum CHECK constraints drift between the
// Postgres migration and the SQLite schema, so the two backends keep rejecting
// the same invalid values.
func TestSchemaEnumParity(t *testing.T) {
	pgBytes, err := migrations.FS.ReadFile("0001_init.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	pg := enumCheckSet(string(pgBytes))
	lite := enumCheckSet(sqliteSchema)
	if len(pg) < 8 {
		t.Fatalf("parsed too few Postgres CHECK enums (%d) — regex drift?", len(pg))
	}
	if !reflect.DeepEqual(pg, lite) {
		t.Fatalf("enum CHECK drift between Postgres and SQLite:\n only in postgres: %v\n only in sqlite:   %v",
			diffKeys(pg, lite), diffKeys(lite, pg))
	}
}

func diffKeys(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// TestSQLiteRejectsInvalidEnum proves the CHECK constraints are live on the
// embedded backend (not just present in the DDL string).
func TestSQLiteRejectsInvalidEnum(t *testing.T) {
	st, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	ctx := context.Background()
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApp(ctx, App{Name: "a", APIKeyHash: "h", APIKeyPrefix: "p", WebhookSecret: "w"})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if _, err := st.CreateDevice(ctx, Device{AppID: app.ID, Name: "d", DeviceSecret: "s", Type: "bogus", Capabilities: []string{"sms"}}); err == nil {
		t.Fatal("CreateDevice with an invalid type should be rejected by the CHECK constraint")
	}
}
