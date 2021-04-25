package main

import "strings"

const (
	None = '\x00'
	Fore = '+'
	Back = '-'
)

const (
	Char = iota
	Line
)

func isaddrc(r rune) bool {
	if r != 0 && strings.ContainsRune("0123456789+-/$.#,;?", r) {
		return true
	}
	return false
}

/*
 * quite hard: could be almost anything but white space, but we are a little conservative,
 * aiming for regular expressions of alphanumerics and no white space
 */
func isregexc(r rune) bool {
	if r == 0 {
		return false
	}
	if isalnum(r) {
		return true
	}
	if strings.ContainsRune("^+-.*?#,;[]()$", r) {
		return true
	}
	return false
}

// nlcounttopos starts at q0 and advances nl lines,
// being careful not to walk past the end of the text,
// and then nr chars, being careful not to walk past
// the end of the current line.
// It returns the final position.
func nlcounttopos(t *Text, q0 int, nl int, nr int) int {
	for nl > 0 && q0 < t.file.b.nc {
		tmp1 := q0
		q0++
		if textreadc(t, tmp1) == '\n' {
			nl--
		}
	}
	if nl > 0 {
		return q0
	}
	for nr > 0 && q0 < t.file.b.nc && textreadc(t, q0) != '\n' {
		q0++
		nr--
	}
	return q0
}

func number(showerr bool, t *Text, r Range, line int, dir rune, size int, evalp *bool) Range {
	q0 := r.q0
	q1 := r.q1
	if size == Char {
		if dir == Fore {
			line = r.q1 + line
		} else if dir == Back {
			if r.q0 == 0 && line > 0 {
				r.q0 = t.file.b.nc
			}
			line = r.q0 - line
		}
		if line < 0 || line > t.file.b.nc {
			goto Rescue
		}
		*evalp = true
		return range_(line, line)
	}
	switch dir {
	case None:
		q0 = 0
		q1 = 0
		goto Forward
	case Fore:
		if q1 > 0 {
			for q1 < t.file.b.nc && textreadc(t, q1-1) != '\n' {
				q1++
			}
		}
		q0 = q1
		goto Forward
	case Back:
		if q0 < t.file.b.nc {
			for q0 > 0 && textreadc(t, q0-1) != '\n' {
				q0--
			}
		}
		q1 = q0
		for line > 0 && q0 > 0 {
			if textreadc(t, q0-1) == '\n' {
				line--
				if line >= 0 {
					q1 = q0
				}
			}
			q0--
		}
		/* :1-1 is :0 = #0, but :1-2 is an error */
		if line > 1 {
			goto Rescue
		}
		for q0 > 0 && textreadc(t, q0-1) != '\n' {
			q0--
		}
	}
Return:
	*evalp = true
	return range_(q0, q1)

Forward:
	for line > 0 && q1 < t.file.b.nc {
		tmp2 := q1
		q1++
		if textreadc(t, tmp2) == '\n' || q1 == t.file.b.nc {
			line--
			if line > 0 {
				q0 = q1
			}
		}
	}
	if line == 1 && q1 == t.file.b.nc { // 6 goes to end of 5-line file
		goto Return
	}
	if line > 0 {
		goto Rescue
	}
	goto Return

Rescue:
	if showerr {
		warning(nil, "address out of range\n")
	}
	*evalp = false
	return r
}

func regexp(showerr bool, t *Text, lim Range, r Range, pat []rune, dir rune, foundp *bool) Range {
	if pat[0] == '\x00' && rxnull() {
		if showerr {
			warning(nil, "no previous regular expression\n")
		}
		*foundp = false
		return r
	}
	if pat[0] != 0 && !rxcompile(pat) {
		*foundp = false
		return r
	}
	var found bool
	var sel Rangeset
	if dir == Back {
		found = rxbexecute(t, r.q0, &sel)
	} else {
		var q int
		if lim.q0 < 0 {
			q = Infinity
		} else {
			q = lim.q1
		}
		found = rxexecute(t, nil, r.q1, q, &sel)
	}
	if !found && showerr {
		warning(nil, "no match for regexp\n")
	}
	*foundp = found
	return sel.r[0]
}

func address(showerr bool, t *Text, lim Range, ar Range, a interface{}, q0 int, q1 int, getc func(interface{}, int) rune, evalp *bool, qp *int) Range {
	r := ar
	q := q0
	dir := None
	size := Line
	var c rune
	for q < q1 {
		prevc := c
		c = getc(a, q)
		q++
		var nr Range
		var pat []rune
		var n int
		var nc rune
		switch c {
		default:
			*qp = q - 1
			return r
		case ';':
			ar = r
			fallthrough
		/* fall through */
		case ',':
			if prevc == 0 { /* lhs defaults to 0 */
				r.q0 = 0
			}
			if q >= q1 && t != nil && t.file != nil { /* rhs defaults to $ */
				r.q1 = t.file.b.nc
			} else {
				nr = address(showerr, t, lim, ar, a, q, q1, getc, evalp, &q)
				r.q1 = nr.q1
			}
			*qp = q
			return r
		case '+', '-':
			if *evalp && (prevc == '+' || prevc == '-') {
				nc = getc(a, q)
				if nc != '#' && nc != '/' && nc != '?' {
					r = number(showerr, t, r, 1, prevc, Line, evalp) /* do previous one */
				}
			}
			dir = c
		case '.',
			'$':
			if q != q0+1 {
				*qp = q - 1
				return r
			}
			if *evalp {
				if c == '.' {
					r = ar
				} else {
					r = range_(t.file.b.nc, t.file.b.nc)
				}
			}
			if q < q1 {
				dir = Fore
			} else {
				dir = None
			}
		case '#':
			if q == q1 || func() bool { c = getc(a, q); _r := c < '0'; q++; return _r }() || '9' < c {
				*qp = q - 1
				return r
			}
			size = Char
			fallthrough
		/* fall through */
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			n = int(c - '0')
			for q < q1 {
				nc = getc(a, q)
				q++
				if nc < '0' || '9' < nc {
					q--
					break
				}
				n = n*10 + int(nc-'0')
			}
			if *evalp {
				r = number(showerr, t, r, n, dir, size, evalp)
			}
			dir = None
			size = Line
		case '?':
			dir = Back
			fallthrough
		/* fall through */
		case '/':
			pat = nil
			for q < q1 {
				c = getc(a, q)
				q++
				switch c {
				case '\n':
					q--
					goto out
				case '\\':
					pat = append(pat, c)
					if q == q1 {
						goto out
					}
					c = getc(a, q)
					q++
				case '/':
					goto out
				}
				pat = append(pat, c)
			}
		out:
			if *evalp {
				r = regexp(showerr, t, lim, r, pat, dir, evalp)
			}
			dir = None
			size = Line
		}
	}
	if *evalp && dir != None {
		r = number(showerr, t, r, 1, dir, Line, evalp) /* do previous one */
	}
	*qp = q
	return r
}