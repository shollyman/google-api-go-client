// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package disco

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

const (
	ChangeTypeAdd    = "ADDED"
	ChangeTypeModify = "CHANGED"
	ChangeTypeDelete = "DELETED"
)

// DiffEntry describes a change in a discovery document.
type DiffEntry struct {
	ChangeType  string
	ElementKind string
	ElementID   string
	OldValue    string
	NewValue    string
	Children    []*DiffEntry
}

func diffDocs(old, new *Document) []*DiffEntry {
	diffs := compareTopLevelStrings(old, new)
	schemaDiffs := compareSchemaMap(old, new)
	if schemaDiffs != nil {
		diffs = append(diffs, schemaDiffs...)
	}
	return diffs
}

func printDiff(entries []*DiffEntry) string {
	return printDiffInternal(entries, 0)
}

func printDiffInternal(entries []*DiffEntry, level int) string {
	if entries == nil {
		return ""
	}
	var buf bytes.Buffer
	for _, e := range entries {
		if e != nil {
			buf.WriteString(strings.Repeat(" ", level*2))
			buf.WriteString(fmt.Sprintf("%s %s %s", e.ChangeType, e.ElementKind, e.ElementID))
			if e.ChangeType == ChangeTypeModify {
				buf.WriteString(fmt.Sprintf(" [ %s ==> %s ]", e.OldValue, e.NewValue))
			}
			if e.ChangeType == ChangeTypeAdd && e.NewValue != "" {
				buf.WriteString(fmt.Sprintf("[ %s ]", e.NewValue))
			}
			buf.WriteString("\n")
			if e.Children != nil {
				buf.WriteString(printDiffInternal(e.Children, level+1))
			}
		}
	}
	return buf.String()
}

func compareTopLevelStrings(old, new *Document) []*DiffEntry {
	var diffs []*DiffEntry
	// we don't diff ID because we expect it to be opaque
	// and frequently changing.

	maybeDiffs := []*DiffEntry{
		diffString("Name", old.Name, new.Name),
		diffString("Version", old.Version, new.Version),
		diffString("Revision", old.Revision, new.Revision),
		diffString("Title", old.Title, new.Title),
		diffString("RootURL", old.RootURL, new.RootURL),
		diffString("ServicePath", old.ServicePath, new.ServicePath),
		diffString("BasePath", old.BasePath, new.BasePath),
		diffString("DocumentationLink", old.DocumentationLink, new.DocumentationLink),
	}

	for _, d := range maybeDiffs {
		if d != nil {
			diffs = append(diffs, d)
		}
	}
	if len(diffs) > 0 {
		return diffs
	}
	return nil
}

func compareSchemaMap(old, new *Document) []*DiffEntry {
	var diffs []*DiffEntry

	// combine keys from old and new
	keys := make(map[string]bool)

	for k := range old.Schemas {
		keys[k] = true
	}
	for k := range new.Schemas {
		keys[k] = true
	}
	var sortedKeys []string
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	for _, k := range sortedKeys {
		oldSchema := old.Schemas[k]
		newSchema := new.Schemas[k]
		if oldSchema == nil {
			if newSchema != nil {
				d := compareSchema(k, &Schema{}, newSchema)
				d.ChangeType = ChangeTypeAdd
				for _, v := range d.Children {
					v.ChangeType = ChangeTypeAdd
				}
				diffs = append(diffs, d)
			}
		}
		if newSchema != nil {
			d := compareSchema(k, oldSchema, newSchema)
			if d != nil {
				diffs = append(diffs, d)
			}
		} else {
			diffs = append(diffs, &DiffEntry{
				ChangeType:  ChangeTypeDelete,
				ElementKind: "schema",
				ElementID:   k,
			})
		}
	}
	if len(diffs) > 0 {
		return diffs
	}
	return nil
}

func compareSchema(name string, old, new *Schema) *DiffEntry {
	if old == nil || new == nil {
		return nil
	}
	baseDiff := &DiffEntry{
		ChangeType:  ChangeTypeModify,
		ElementKind: "schema",
		ElementID:   name,
	}

	var childDiffs []*DiffEntry
	maybeDiffs := []*DiffEntry{
		diffString("ID", old.ID, new.ID),
		diffString("Type", old.Type, new.Type),
		diffString("Format", old.Format, new.Format),
		diffString("Description", old.Description, new.Description),
		diffString("Ref", old.Ref, new.Ref),
		diffString("Default", old.Default, new.Default),
		diffString("Pattern", old.Pattern, new.Pattern),
	}

	for _, d := range maybeDiffs {
		if d != nil {
			childDiffs = append(childDiffs, d)
		}
	}

	if len(childDiffs) > 0 {
		baseDiff.Children = childDiffs
		return baseDiff
	}
	return nil
}

func diffString(kind, old, new string) *DiffEntry {
	if old != new {
		return &DiffEntry{
			ChangeType:  ChangeTypeModify,
			ElementKind: kind,
			OldValue:    old,
			NewValue:    new,
		}
	}
	return nil
}
