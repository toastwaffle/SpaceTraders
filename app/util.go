package app

import (
	"bytes"
	"fmt"
	"text/template"
)

func printTemplate(tpl *template.Template, data any) error {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	fmt.Println(buf.String())
	return nil
}

func stringTemplate(tpl *template.Template, data any) (string, error) {
	var buf bytes.Buffer
	err := tpl.Execute(&buf, data)
	return buf.String(), err
}
