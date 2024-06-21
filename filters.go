package main

import (
	"math/rand"
	"reflect"
	"time"
)

// firstN returns the first n items of a slice.
func firstN(n int, items interface{}) interface{} {
	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		panic("firstN: not a slice")
	}
	if v.Len() < n {
		return items
	}
	return v.Slice(0, n).Interface()
}

// lastN returns the last n items of any slice.
func lastN(n int, items interface{}) interface{} {
	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		panic("lastN: not a slice")
	}
	l := v.Len()
	if l < n {
		return items
	}
	return v.Slice(l-n, l).Interface()
}

// randomN returns n random items of any slice.
func randomN(n int, items interface{}) interface{} {
	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		panic("randomN: not a slice")
	}
	l := v.Len()
	if l < n {
		return items
	}
	rand.Seed(time.Now().UnixNano())
	indices := rand.Perm(l)[:n]
	result := reflect.MakeSlice(v.Type(), n, n)
	for i, idx := range indices {
		result.Index(i).Set(v.Index(idx))
	}
	return result.Interface()
}

// filterByType filters pages by their type.
func filterByType(pageType string, pages interface{}) interface{} {
	v := reflect.ValueOf(pages)
	if v.Kind() != reflect.Slice {
		panic("filterByType: not a slice")
	}

	var filtered []interface{}
	for i := 0; i < v.Len(); i++ {
		page := v.Index(i).Interface().(Page)
		if page.Type == pageType {
			filtered = append(filtered, page)
		}
	}
	return filtered
}

