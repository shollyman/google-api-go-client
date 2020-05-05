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
	// got := DiffDocs(old, new, ResourceOption)

	want := []*DiffEntry{
		{
			ChangeType:  ModifyChange,
			ElementKind: StringFieldKind,
			ElementID:   "Revision",
			OldValue:    "20161109",
			NewValue:    "20191101",
		},
		{
			ChangeType:  ModifyChange,
			ElementKind: StringFieldKind,
			ElementID:   "Title",
			OldValue:    "Cloud Storage JSON API",
			NewValue:    "Cloud StOrAGe JSON API",
		},
		{
			ChangeType:  DeleteChange,
			ElementKind: SchemaKind,
			ElementID:   "Schemas.VariantExample",
		},
		{
			ChangeType:  AddChange,
			ElementKind: SchemaKind,
			ElementID:   "Schemas.Shovel",
			Children: []*DiffEntry{
				{
					ChangeType:  AddChange,
					ElementKind: StringFieldKind,
					ElementID:   "ID",
					NewValue:    "Shovel",
				},
				{
					ChangeType:  AddChange,
					ElementKind: StringFieldKind,
					ElementID:   "Type",
					NewValue:    "object",
				},
				{
					ChangeType:  AddChange,
					ElementKind: StringFieldKind,
					ElementID:   "Name",
					NewValue:    "Shovel",
				},
			},
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

	fmt.Printf(renderDiff(got))

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
