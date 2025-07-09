<<<<<<< HEAD
﻿package gee
=======
package gee
>>>>>>> 84ce0810c2b4c7e4e4e951d5a8a523e755a0357a

import (
	"log"
	"net/http"
<<<<<<< HEAD
	"strings"
)

type router struct {
	handlers map[string]HandlerFunc //key:GET-/p/:lang/doc,POST-/p/book
	roots    map[string]*node       //key:GET,POST
=======
)

type router struct {
	handlers map[string]HandlerFunc
>>>>>>> 84ce0810c2b4c7e4e4e951d5a8a523e755a0357a
}

func newRouter() *router {
	return &router{
		handlers: make(map[string]HandlerFunc),
<<<<<<< HEAD
		roots:    make(map[string]*node),
=======
>>>>>>> 84ce0810c2b4c7e4e4e951d5a8a523e755a0357a
	}
}

func (r *router) addRoute(method, pattern string, handler HandlerFunc) {
	log.Printf("Router %4s - %s", method, pattern)
<<<<<<< HEAD
	parts := parsePattern(pattern)
	key := method + "-" + pattern
	_, ok := r.roots[method]
	if !ok {
		r.roots[method] = &node{}
	}
	r.roots[method].insert(pattern, parts, 0)
=======
	key := method + "-" + pattern
>>>>>>> 84ce0810c2b4c7e4e4e951d5a8a523e755a0357a
	r.handlers[key] = handler
}

func (r *router) handle(c *Context) {
<<<<<<< HEAD
	n, params := r.getRoute(c.Method, c.Path)
	if n != nil {
		c.Params = params
		key := c.Method + "-" + n.pattern
		r.handlers[key](c)
	} else {
		c.String(http.StatusNotFound, "404 page not found: %s\n", c.Path)
	}
}

// 只允许一个*
func parsePattern(pattern string) []string {
	vs := strings.Split(pattern, "/")
	parts := make([]string, 0)
	for _, item := range vs {
		if item != "" {
			parts = append(parts, item)
			if item[0] == '*' {
				break
			}
		}
	}
	return parts
}

func (r *router) getRoute(method, path string) (*node, map[string]string) {
	searchParts := parsePattern(path)
	params := make(map[string]string)
	root, ok := r.roots[method]
	if !ok {
		return nil, nil
	}
	node := root.search(searchParts, 0)
	if node != nil {
		parts := parsePattern(node.pattern)
		for index, part := range parts {
			if part[0] == ':' {
				params[part[1:]] = searchParts[index]
			}
			if part[0] == '*' && len(part) > 1 {
				params[part[1:]] = strings.Join(searchParts[index:], "/")
				break
			}
		}
		return node, params
	}
	return nil, nil
}
=======
	key := c.Method + "-" + c.Path
	if handler, ok := r.handlers[key]; ok {
		handler(c)
	} else {
		c.String(http.StatusNotFound, "404 page not found: %s\n", c.Path)
	}
}
>>>>>>> 84ce0810c2b4c7e4e4e951d5a8a523e755a0357a
