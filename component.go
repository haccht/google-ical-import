package main

import (
	"bytes"
	"fmt"
	"strings"
)

type Event struct {
	Component
}

func NewEvent() *Event {
	return &Event{*NewComponent("VEVENT")}
}

type Calendar struct {
	Component
}

func NewCalendar() *Calendar {
	c := &Calendar{*NewComponent("VCALENDAR")}
	c.AddProperty("PRODID", "iCal Encoder/Decoder by haccht", nil)
	c.AddProperty("VERSION", "2.0", nil)

	return c
}

func (c *Calendar) Events() []*Event {
	events := []*Event{}
	for _, v := range c.Components {
		if v.Name == "VEVENT" {
			events = append(events, &Event{*v})
		}
	}
	return events
}

type Component struct {
	Name       string
	Properties map[string]*Property
	Components []*Component
}

func NewComponent(name string) *Component {
	return &Component{
		Name:       name,
		Properties: make(map[string]*Property),
		Components: make([]*Component, 0),
	}
}

func (c *Component) AddProperty(name, value string, params map[string]string) {
	c.Properties[name] = newProperty(name, value, params)
}

func (c *Component) AddComponent(comp *Component) {
	c.Components = append(c.Components, comp)
}

func (c *Component) String() string {
	buf := bytes.NewBufferString("")
	fmt.Fprintf(buf, "BEGIN:%s\n", c.Name)

	for _, prop := range c.Properties {
		fmt.Fprintf(buf, "%s\n", prop.String())
	}

	for _, comp := range c.Components {
		fmt.Fprintf(buf, "%s", comp.String())
	}

	fmt.Fprintf(buf, "END:%s\n", c.Name)
	return buf.String()
}

type Property struct {
	Name   string
	Value  string
	Params map[string]string
}

func (p *Property) String() string {
	params := []string{p.Name}
	for k, v := range p.Params {
		param := strings.Join([]string{k, v}, "=")
		params = append(params, param)
	}

	str := strings.Join(params, ";")
	if p.Value != "" {
		str = fmt.Sprintf("%s:%s", str, p.Value)
	}

	// TODO: fold long property text
	return str
}

func newProperty(name, value string, params map[string]string) *Property {
	if params == nil {
		params = make(map[string]string)
	}

	return &Property{
		Name:   name,
		Value:  value,
		Params: params,
	}
}
