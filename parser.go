package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"unicode"
)

type Parser struct {
	stack   []*Component
	readbuf []string
	scanner *bufio.Scanner
}

func ParseFile(path string) (*Calendar, error) {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("failed to open file: %s", path)
	}
	defer f.Close()

	return Parse(f)
}

func Parse(r io.Reader) (*Calendar, error) {
	root := NewComponent("ROOT")

	p := &Parser{scanner: bufio.NewScanner(r)}
	p.stack = append(p.stack, root)

	for {
		line, err := p.nextLine()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch next line: %s", err)
		}

		keyval := strings.SplitN(line, ":", 2)
		params := strings.Split(keyval[0], ";")

		key := params[0]
		val := keyval[1]

		prop := newProperty(key, val)
		for _, param := range params[1:] {
			kv := strings.SplitN(param, "=", 2)
			prop.Params[kv[0]] = kv[1]
		}

		head := p.stack[len(p.stack)-1]
		switch key {
		case "BEGIN":
			e := NewComponent(prop.Value)
			p.stack = append(p.stack, e)
			head.Components = append(head.Components, e)
		case "END":
			e := p.stack[len(p.stack)-1]
			p.stack = p.stack[:len(p.stack)-1]

			if e.Name != prop.Value {
				return nil, fmt.Errorf("Unmatch component found: %s", prop.Value)
			}
		default:
			head.Properties[key] = prop
		}
	}

	for _, v := range root.Components {
		if v.Name == "VCALENDAR" {
			return &Calendar{*v}, nil
		}
	}
	return nil, fmt.Errorf("Could not found \"VCALENDAR\" component")
}

func (p *Parser) peek() (string, error) {
	line, err := p.next()
	if err != nil {
		return "", err
	}

	p.readbuf = append(p.readbuf, line)
	return line, nil
}

func (p *Parser) next() (string, error) {
	if len(p.readbuf) > 0 {
		line := p.readbuf[len(p.readbuf)-1]
		p.readbuf = p.readbuf[:len(p.readbuf)-1]
		return line, nil
	}

	if !p.scanner.Scan() {
		return "", io.EOF
	}
	return p.scanner.Text(), nil
}

func (p *Parser) nextLine() (string, error) {
	line, err := p.next()
	if err != nil {
		return "", err
	}

	for {
		peek, err := p.peek()
		if err == io.EOF || !unicode.IsSpace(rune(peek[0])) {
			break
		}

		p.next()
		line += strings.TrimLeftFunc(peek, unicode.IsSpace)
	}

	return line, nil
}
