package main

// Small readers the dispatch, the reuse route and `hw revive` share. They lived beside gentle's home checks until
// gentle-ai left hw; nothing about them was gentle's.

// rtJqWalk walks .a.b[0].c: a string is a key, an int an index. null indexes to null; a value that cannot be
// indexed that way is an error, and the text is jq's.
func rtJqWalk(v *jval, steps []any) (*jval, string) {
	cur := v
	for _, s := range steps {
		switch x := s.(type) {
		case string:
			n, ok := cur.get(x)
			if !ok {
				return nil, "Cannot index " + kindName(cur) + " with string " + jstr(x).toJSON()
			}
			cur = n
		case int:
			switch cur.kind {
			case vNull:
				cur = jnull
			case vArr:
				if x < len(cur.arr) {
					cur = cur.arr[x]
				} else {
					cur = jnull
				}
			default:
				return nil, "Cannot index " + kindName(cur) + " with number"
			}
		}
	}
	return cur, ""
}

// rtJqSteps is rtJqWalk without jq's error text: whether the walk held.
func rtJqSteps(v *jval, steps []any) (*jval, bool) {
	cur, msg := rtJqWalk(v, steps)
	return cur, msg == ""
}

