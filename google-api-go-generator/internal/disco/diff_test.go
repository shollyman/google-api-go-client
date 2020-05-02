// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package disco

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDiff(t *testing.T) {
	old, err := loadDoc("testdata/test-api.json")
	if err != nil {
		t.Fatal(err)
	}
	new, err := loadDoc("testdata/modified-api.json")
	if err != nil {
		t.Fatal(err)
	}

	got := DiffDocs(old, new, AllOptions)

	want := []*DiffEntry{
		{
			ChangeType:  ModifyChange,
			ElementKind: "Revision",
			OldValue:    "20161109",
			NewValue:    "20191101",
		},
		{
			ChangeType:  ModifyChange,
			ElementKind: "Title",
			OldValue:    "Cloud Storage JSON API",
			NewValue:    "Cloud StOrAGe JSON API",
		},
	}

	for _, w := range want {
		found := false
		for _, g := range got {
			if cmp.Equal(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Didn't find diff entry: %#v", w)
		}
	}

	fmt.Printf(printDiff(got))
}

// quick helper for loading doc
func loadDoc(path string) (*Document, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := NewDocument(bytes)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
