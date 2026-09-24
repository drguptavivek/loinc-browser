package loinc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMapperGuide(t *testing.T) {
	defer mapperGuide.Store(nil)
	if count, err := LoadMapperGuide(filepath.Join(t.TempDir(), "missing.csv")); count != 0 || err != nil {
		t.Fatalf("a missing guide is optional, got %d %v", count, err)
	}
	path := filepath.Join(t.TempDir(), "guide.csv")
	csv := "loinc_num,long_common_name,class,rank,example_ucum,example_display,comment,system_adjusted\n" +
		"2345-7,Glucose,Chem,4,mg/dL,mg/dL,,Bld*/Ser/Plas\n" +
		"4548-4,HbA1c,Chem,81,%,%,\"Use for NGSP, not IFCC\",Bld\n" +
		"15530-9,RAST,Allergy,1289,,,,Ser\n"
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := LoadMapperGuide(path)
	if err != nil || count != 2 {
		t.Fatalf("want 2 entries (the row with no unit or comment is skipped), got %d %v", count, err)
	}
	if got := mapperGuideEntry("4548-4"); got.ExampleUCUM != "%" || got.Comment != "Use for NGSP, not IFCC" {
		t.Fatalf("4548-4: got %+v", got)
	}
	if got, _ := withMapperGuide(Term{LOINCNum: "2345-7"}, nil); got.ExampleUCUM != "mg/dL" {
		t.Fatalf("term not annotated: %+v", got)
	}
}

func TestLoadCLCI(t *testing.T) {
	defer clci.Store(nil)
	if count, err := LoadCLCI(filepath.Join(t.TempDir(), "missing.csv")); count != 0 || err != nil {
		t.Fatalf("CLCI is optional, got %d %v", count, err)
	}
	path := filepath.Join(t.TempDir(), "clci.csv")
	csv := "\xef\xbb\xbf\"General Name\",\"LOINC Code\",\"Fully-Specified Name (FSN)\",\"Long Common Name\"\n" +
		"\"Amylase, Blood\",\"1798-8\",\"Amylase:CCnc:Pt:Ser/Plas:Qn\",\"Amylase in Serum or Plasma\"\n" +
		"\"Junk\",\"1798-8'; drop table x;--\",\"\",\"\"\n"
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	if count, err := LoadCLCI(path); err != nil || count != 1 {
		t.Fatalf("want 1 entry (BOM stripped, malformed code skipped), got %d %v", count, err)
	}
	if clciName("1798-8") != "Amylase, Blood" || clciInList() != "('1798-8')" {
		t.Fatalf("got name %q list %q", clciName("1798-8"), clciInList())
	}
	for query, want := range map[string]int{"amylase blood": 1, "Blood, AMYLASE test": 1, "amylase": 0, "blood": 0} {
		if got := len(clciNameMatches(query)); got != want {
			t.Errorf("clciNameMatches(%q): %d matches, want %d", query, got, want)
		}
	}
}

func TestLoadCommonCodes(t *testing.T) {
	defer clci.Store(nil)

	// A headerless 2-column list, name before code: any deployment's own common-codes CSV.
	headerless := filepath.Join(t.TempDir(), "common.csv")
	if err := os.WriteFile(headerless, []byte("S. Sodium,2951-2\nS. Potassium,2823-3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := LoadCommonCodes(headerless, "US Common Codes")
	if err != nil || count != 2 {
		t.Fatalf("want 2 entries, got %d %v", count, err)
	}
	if label, n := CommonCodesInfo(); label != "US Common Codes" || n != 2 {
		t.Fatalf("got label %q count %d", label, n)
	}
	if clciName("2951-2") != "" {
		t.Fatalf("clciName must stay empty for a non-CLCI list, got %q", clciName("2951-2"))
	}
	if localName("2951-2") != "S. Sodium" {
		t.Fatalf("localName: got %q", localName("2951-2"))
	}

	// A header row plus a code column that isn't first, other columns blank.
	withHeader := filepath.Join(t.TempDir(), "common2.csv")
	csv := "site,region,code,note\n" +
		",,1751-7,\n" +
		"Lab A,,4548-4,\n"
	if err := os.WriteFile(withHeader, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err = LoadCommonCodes(withHeader, "Hospital List")
	if err != nil || count != 2 {
		t.Fatalf("want 2 entries (header skipped, code detected in column 3), got %d %v", count, err)
	}
	if localName("4548-4") != "Lab A" {
		t.Fatalf("localName from first other non-empty column: got %q", localName("4548-4"))
	}
	if localName("1751-7") != "" {
		t.Fatalf("row with no other non-empty column: got %q", localName("1751-7"))
	}
}
