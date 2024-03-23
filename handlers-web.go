package main

import (
	"fmt"
	"html/template"
	"net/http"
)

func ssrHome(w http.ResponseWriter, req *http.Request) {
	// Subtree paths (ending with trailing slash) are essentially treated as
	// catch-all routes by ServeMux. Return 404 response if not exact match.
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	// Keep "base" template as first file in the slice.
	files := []string{
		"./ui/base.tmpl.html",
		"./ui/partial-nav.tmpl.html",
		"./ui/home.tmpl.html",
	}

	// Read the template file into a template set.
	ts, err := template.ParseFiles(files...)
	if err != nil {
		fmt.Printf("[err][api] parsing template file: %v [%s]\n", err, cts())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Use the ExecuteTemplate() method to write the content of the "base"
	// template as the response body.
	err = ts.ExecuteTemplate(w, "base", nil)
	if err != nil {
		fmt.Printf("[err][api] executing template: %v [%s]\n", err, cts())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	// w.Write([]byte("Hello from Cooperative Party"))
}

func ssrEximDetails(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("Display a specific exim..."))
}

func ssrCreateExim(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("Create a new exim..."))
}
