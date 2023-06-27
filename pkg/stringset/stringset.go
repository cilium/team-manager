// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package stringset

import "sort"

// A StringSet is a set of strings.
type StringSet map[string]struct{}

// New returns a new StringSet containing elements.
func New(elements ...string) StringSet {
	s := make(StringSet)
	s.Add(elements...)
	return s
}

// Add adds elements to s.
func (s StringSet) Add(elements ...string) {
	for _, element := range elements {
		s[element] = struct{}{}
	}
}

// Elements returns all the elements of s.
func (s StringSet) Elements() []string {
	elements := make([]string, 0, len(s))
	for element := range s {
		elements = append(elements, element)
	}
	sort.Strings(elements)
	return elements
}

// Remove removes elements from s.
func (s StringSet) Remove(elements ...string) {
	for _, element := range elements {
		delete(s, element)
	}
}
