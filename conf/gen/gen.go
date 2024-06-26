package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

type TmplVar struct {
	BuildMicroSeconds int64
}

func generate(tmpl_file, out_file string) error {
	tmpl, err := template.ParseFiles(tmpl_file)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(out_file, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	tmpl_base := filepath.Base(tmpl_file)

	param := TmplVar{
		BuildMicroSeconds: time.Now().UnixMicro(),
	}

	if _, err := fmt.Fprintf(out, "// Code generated by %s; DO NOT EDIT.\n",
		tmpl_base); err != nil {
		return err
	}

	return tmpl.Execute(out, param)
}

func main() {
	log.Println("Start: generation")
	defer log.Println("Done: generation")

	tmpl_files, err := filepath.Glob("./*.gen.go.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	for _, tmpl := range tmpl_files {
		out := tmpl[:len(tmpl)-5]
		log.Println("generate:", tmpl, "->", out)
		if err := generate(tmpl, out); err != nil {
			log.Fatal(err)
		}
	}
}
