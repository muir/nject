package nject

import (
	"fmt"
)

// ReplaceNamed will edit the set of injectors, replacing target injector,
// identified by the name it was given with Provide(), with the
// injector provided here. If the replacement injector is nil, the
// target injector will simply be removed.
// This replacement happens very early in the
// injection chain processing, before Reorder or injector selection.
// If target does not exist, the injection chain is deemed invalid.
func ReplaceNamed(target string, fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.replaceByName = target
	})
}

// InsertAfterNamed will edit the set of injectors, inserting the
// provided injector after the target injector, which is identified
// by the name it was given with Provide().  That injector can be a
// Collection.  This re-arrangement happens very early in the injection
// chain processing, before Reorder or injector selection.
// If target does not exist, the injection chain is deemed invalid.
func InsertAfterNamed(target string, fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.insertAfterName = target
	})
}

// InsertBeforeNamed will edit the set of injectors, inserting the
// provided injector before the target injector, which is identified
// by the name it was given with Provide().  That injector can be a
// Collection.  This re-arrangement happens very early in the injection
// chain processing, before Reorder or injector selection.
// If target does not exist, the injection chain is deemed invalid.
func InsertBeforeNamed(target string, fn interface{}) Provider {
	return newThing(fn).modify(func(fm *provider) {
		fm.insertBeforeName = target
	})
}

// What makes handleReplaceByName complicated is that names can be duplicated
// and the replace directives can be duplicated.
//
// When you add a name, with Provide(), you can add it to a collection
// thus naming multiple providers with the same name.
//
// Likewise, when you tag a provider with InsertAfterName, you can
// be tagging a colleciton, not an individual.
func (c *Collection) handleReplaceByName() (err error) {
	defer func() {
		// if err != nil {
		debugln("replacment directives --------------------------------------")
		for _, fm := range c.contents {
			var tag string
			if fm.replaceByName != "" {
				tag = "replace:" + fm.replaceByName
			}
			if fm.insertBeforeName != "" {
				tag = "insertBefore:" + fm.insertBeforeName
			}
			if fm.insertAfterName != "" {
				tag = "insertAfter:" + fm.insertAfterName
			}
			if tag != "" {
				debugln("\t", tag, fm)
			}
		}
		// }
	}()

	var hasReplacements bool
	for _, fm := range c.contents {
		if fm.replaceByName != "" || fm.insertAfterName != "" || fm.insertBeforeName != "" {
			hasReplacements = true
			break
		}
	}
	if !hasReplacements {
		return nil
	}

	type node struct {
		i         int // for debugging
		fm        *provider
		prev      *node
		next      *node
		processed bool
	}

	// step 1, convert to a linked list with a fake head & tail
	head := &node{}
	prior := head
	for i, fm := range c.contents {
		var replacers int
		if fm.replaceByName != "" {
			replacers++
		}
		if fm.insertBeforeName != "" {
			replacers++
		}
		if fm.insertAfterName != "" {
			replacers++
		}
		if replacers > 1 {
			return fmt.Errorf("a provider, %s, can have only one of the ReplaceName, InsertAfterName, InsertBeforeName annotations", fm)
		}
		n := &node{
			i:    i,
			fm:   fm,
			prev: prior,
		}
		prior.next = n
		prior = n
	}
	tail := &node{}
	prior.next = tail

	// step 2, build the name index
	type firstLast struct {
		first      *node
		last       *node
		duplicated bool
	}
	names := make(map[string]*firstLast)
	var lastName string
	var lastFirstLast *firstLast
	for n := head.next; n != tail; n = n.next {
		switch {
		case n.fm.origin == "":
			// nothing to do
			lastName = ""
		case n.fm.origin == lastName:
			lastFirstLast.last = n
		default:
			lastFirstLast = &firstLast{
				first: n,
				last:  n,
			}
			lastName = n.fm.origin
			if current, ok := names[lastName]; ok {
				current.duplicated = true
			} else {
				names[lastName] = lastFirstLast
			}
		}
	}

	getTarget := func(name string, op string) (*firstLast, error) {
		target, ok := names[name]
		if !ok {
			return nil, fmt.Errorf("cannot %s '%s', not in chain", op, name)
		}
		if target.duplicated {
			return nil, fmt.Errorf("cannot %s '%s', duplicated in chain", op, name)
		}
		return target, nil
	}

	// step 3, do replacements
	var infiniteLoopCounter int
	for n := head.next; n != nil && n != tail; n = n.next {
		if infiniteLoopCounter > 10000 {
			return fmt.Errorf("internal error #92, infinite loop doing replacements")
		}
		// predicate must be true for target
		snip := func(target *node, predicate func(*node) bool) (start *node, end *node) {
			start = target
			for end = target; end.next != tail && predicate(end.next); end = end.next {
				end.processed = true
			}
			end.processed = true
			start.prev.next = end.next
			end.next.prev = start.prev
			return
		}
		insertBefore := func(target *node, start *node, end *node) {
			prev := target.prev
			start.prev = prev
			prev.next = start
			target.prev = end
			end.next = target
		}
		if n.processed {
			// processed could already be true if a node moved
			// forward in the list
			continue
		}
		switch {
		case n.fm.replaceByName != "":
			name := n.fm.replaceByName
			firstLast, err := getTarget(name, "replace")
			if err != nil {
				return err
			}
			delete(names, name)
			firstSnip, lastSnip := snip(firstLast.first, func(n *node) bool { return n.fm.origin == name })
			firstMove, lastMove := snip(n, func(n *node) bool { return n.fm.replaceByName == name })
			if lastSnip.next == firstMove {
				// adjacent blocks, snip before move, hack a reconnect
				lastSnip.next = lastMove.next
			}

			if firstSnip == lastSnip {
				if firstMove == lastMove {
					debugln("ReplaceNamed replacing", firstSnip.i, firstSnip.fm, "with", firstMove.i, firstMove.fm)
				} else {
					debugln("ReplaceNamed replacing", firstSnip.i, firstSnip.fm, "with sequence from", firstMove.i, firstMove.fm, "to", lastMove.i, lastMove.fm)
				}
			} else {
				if firstMove == lastMove {
					debugln("ReplaceNamed replacing sequence from", firstSnip.i, firstSnip.fm, "through", lastSnip.i, lastSnip.fm, "with", firstMove.i, firstMove.fm)
				} else {
					debugln("ReplaceNamed replacing sequence from", firstSnip.i, firstSnip.fm, "through", lastSnip.i, lastSnip.fm, "with sequence from", firstMove.i, firstMove.fm, "to", lastMove.i, lastMove.fm)
				}
			}
			afterLastMove := lastMove.next
			insertBefore(lastSnip.next, firstMove, lastMove)
			// Where to continue iteration is ticky. Generally, we want to continue
			// at lastMove.next but not in some special cases
			switch {
			case afterLastMove == firstSnip:
				// Adjacent forward move, continue with the end of what's moved
				n = lastMove
			case lastSnip.next == firstMove:
				// Adjacent backwards move, continue with the end of what's moved
				n = lastMove
			default:
				n = afterLastMove.prev
			}
		case n.fm.insertBeforeName != "":
			name := n.fm.insertBeforeName
			firstLast, err := getTarget(name, "insert before")
			if err != nil {
				return err
			}
			firstMove, lastMove := snip(n, func(n *node) bool { return n.fm.insertBeforeName == name })
			afterLastMove := lastMove.next
			insertBefore(firstLast.first, firstMove, lastMove)
			n = afterLastMove.prev
		case n.fm.insertAfterName != "":
			name := n.fm.insertAfterName
			firstLast, err := getTarget(name, "insert after")
			if err != nil {
				return err
			}
			firstMove, lastMove := snip(n, func(n *node) bool { return n.fm.insertBeforeName == name })
			afterLastMove := lastMove.next
			insertBefore(firstLast.last.next, firstMove, lastMove)
			n = afterLastMove.prev
		default:
			// nothing
		}
	}

	// step 4, convert back to list
	contents := make([]*provider, 0, len(c.contents))
	for n := head.next; n != tail; n = n.next {
		if n.fm != nil && n.fm.fn != nil {
			contents = append(contents, n.fm)
		}
	}
	c.contents = contents
	return nil
}
