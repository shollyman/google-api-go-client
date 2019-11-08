// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package disco

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"testing"
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

	got := diffDocs(old, new)

	want := []*DiffEntry{
		{
			ChangeType:  ChangeTypeModify,
			ElementKind: "Version",
			OldValue:    "v1",
			NewValue:    "v99",
		},
		{
			ChangeType:  ChangeTypeModify,
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
