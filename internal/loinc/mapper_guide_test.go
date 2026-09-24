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
