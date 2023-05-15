// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package main

import "sort"

// A stringSet is a set of strings.
type stringSet map[string]struct{}

// newStringSet returns a new StringSet containing elements.
func newStringSet(elements ...string) stringSet {
	s := make(stringSet)
	s.add(elements...)
	return s
}

// add adds elements to s.
func (s stringSet) add(elements ...string) {
	for _, element := range elements {
		s[element] = struct{}{}
	}
}

// elements returns all the elements of s.
func (s stringSet) elements() []string {
	elements := make([]string, 0, len(s))
	for element := range s {
		elements = append(elements, element)
	}
	sort.Strings(elements)
	return elements
}

// remove removes elements from s.
func (s stringSet) remove(elements ...string) {
	for _, element := range elements {
		delete(s, element)
	}
}
