package topk

type item struct {
	value     int32
	frequency int
	index     int
}

// itemList is just a sorted item list by frequency.
// after changing the frequency, you must call fix with the item in order to fix the ordering.
type itemList []*item

func (l *itemList) put(i *item) {
	orig := *l
	i.index = len(orig)
	*l = append(orig, i)
	l.fix(i)
}

func (il *itemList) remove(i *item) {
	idx := i.index
	l := *il

	if i.frequency == l[len(l)-1].frequency {
		il.swap(idx, len(l)-1)
		idx = i.index
	}

	if idx < len(l)-1 {
		copy(l[idx:], l[idx+1:])
	}
	*il = l[0 : len(l)-1]
}

func (l itemList) fix(i *item) {
	if len(l) < 2 {
		return
	}

	idx := i.index
	for idx < len(l)-1 && l[idx+1].frequency > i.frequency {
		l.swap(idx, idx+1)
		idx = i.index
	}

	for idx > 0 && l[idx-1].frequency < i.frequency {
		l.swap(idx, idx-1)
		idx = i.index
	}
}

func (l itemList) swap(i, j int) {
	l[i], l[j] = l[j], l[i]
	l[i].index = i
	l[j].index = j
}
